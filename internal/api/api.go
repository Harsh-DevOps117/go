// Package api translates HTTP requests into core operations and stable JSON responses.
package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"harshDevops117/internal/core"
	"harshDevops117/internal/ratelimit"

	"github.com/gin-gonic/gin"
)

type Options struct {
	Auth           *core.Auth
	Resources      *core.Resources
	Logger         *slog.Logger
	Ready          func(context.Context) error
	GeneralLimit   *ratelimit.Limiter
	AuthLimit      *ratelimit.Limiter
	TrustedProxies []string
}

type server struct{ Options }

func New(opts Options) (*gin.Engine, error) {
	if opts.Auth == nil || opts.Resources == nil || opts.Ready == nil {
		return nil, errors.New("API dependencies are required")
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	s := &server{opts}
	r := gin.New()
	if err := r.SetTrustedProxies(opts.TrustedProxies); err != nil {
		return nil, err
	}
	r.HandleMethodNotAllowed = true
	r.RedirectTrailingSlash = false
	r.Use(s.requestContext())
	r.NoRoute(func(c *gin.Context) { writeError(c, 404, "not_found", "Route not found") })
	r.NoMethod(func(c *gin.Context) { writeError(c, 405, "method_not_allowed", "Method not allowed") })
	r.GET("/health/live", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/health", s.readiness)
	r.GET("/health/ready", s.readiness)

	v1 := r.Group("/api/v1", s.limit(opts.GeneralLimit))
	auth := v1.Group("/auth", s.limit(opts.AuthLimit))
	auth.POST("/register", s.handle(201, s.register))
	auth.POST("/login", s.handle(200, s.login))
	auth.POST("/refresh", s.handle(200, s.refresh))

	private := v1.Group("", s.authenticate())
	private.POST("/auth/logout", s.handle(204, s.logout))
	private.GET("/me", s.handle(200, s.me))
	private.POST("/notes", s.handle(201, s.createNote))
	private.GET("/notes", s.handle(200, s.listNotes))
	private.GET("/notes/:id", s.handle(200, s.getNote))
	private.PATCH("/notes/:id", s.handle(200, s.updateNote))
	private.DELETE("/notes/:id", s.handle(204, s.deleteNote))
	private.POST("/alarms", s.handle(201, s.createAlarm))
	private.GET("/alarms", s.handle(200, s.listAlarms))
	private.GET("/alarms/:id", s.handle(200, s.getAlarm))
	private.PATCH("/alarms/:id", s.handle(200, s.updateAlarm))
	private.DELETE("/alarms/:id", s.handle(204, s.deleteAlarm))
	return r, nil
}

func (s *server) readiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
	defer cancel()
	if err := s.Ready(ctx); err != nil {
		writeError(c, 503, "unavailable", "Database is unavailable")
		return
	}
	c.JSON(200, gin.H{"status": "ready"})
}

type operation func(*gin.Context) (any, error)

func (s *server) handle(status int, fn operation) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := fn(c)
		if err != nil {
			s.fail(c, err)
			return
		}
		if status == http.StatusNoContent {
			c.Status(status)
			return
		}
		c.JSON(status, gin.H{"data": result})
	}
}

func (s *server) fail(c *gin.Context, err error) {
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		writeError(c, 413, "body_too_large", "Request body must not exceed 1 MiB")
	case errors.Is(err, core.ErrValidation):
		writeError(c, 400, "validation_error", err.Error())
	case errors.Is(err, core.ErrUnauthorized):
		c.Header("WWW-Authenticate", "Bearer")
		writeError(c, 401, "unauthorized", "Invalid or expired credentials")
	case errors.Is(err, core.ErrNotFound):
		writeError(c, 404, "not_found", "Resource not found")
	case errors.Is(err, core.ErrConflict):
		writeError(c, 409, "conflict", "Email is already registered")
	case errors.Is(err, context.DeadlineExceeded):
		writeError(c, 504, "timeout", "Request timed out")
	case errors.Is(err, context.Canceled):
		writeError(c, 408, "request_canceled", "Request canceled")
	default:
		s.Logger.Error("request failed", "request_id", c.GetString("request_id"), "error", err)
		writeError(c, 500, "internal_error", "An internal error occurred")
	}
}

func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code, "message": message, "request_id": c.GetString("request_id")}})
}

func (s *server) requestContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		idBytes := make([]byte, 16)
		if _, err := rand.Read(idBytes); err != nil {
			writeError(c, 500, "internal_error", "An internal error occurred")
			return
		}
		id := hex.EncodeToString(idBytes)
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Cache-Control", "no-store")
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		defer func() {
			if recover() != nil {
				// Do not dump headers, bodies, or panic values: these can contain
				// bearer tokens and passwords. The request ID correlates logs.
				s.Logger.Error("request panic", "request_id", id)
				if !c.Writer.Written() {
					writeError(c, 500, "internal_error", "An internal error occurred")
				}
				c.Abort()
			}
			s.Logger.Info("http request", "request_id", id, "method", c.Request.Method,
				"route", c.FullPath(), "status", c.Writer.Status(), "duration_ms", time.Since(started).Milliseconds())
		}()
		c.Next()
	}
}

func (s *server) limit(limiter *ratelimit.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter != nil {
			if allowed, retry := limiter.Allow(c.Request.Context(), c.ClientIP()); !allowed {
				c.Header("Retry-After", strconv.Itoa(max(1, int(math.Ceil(retry.Seconds())))))
				writeError(c, 429, "rate_limited", "Too many requests; try again later")
				return
			}
		}
		c.Next()
	}
}

func (s *server) authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			s.fail(c, core.ErrUnauthorized)
			return
		}
		identity, err := s.Auth.Authenticate(c.Request.Context(), parts[1])
		if err != nil {
			s.fail(c, err)
			return
		}
		c.Set("identity", identity)
		c.Next()
	}
}

func identity(c *gin.Context) core.Identity { return c.MustGet("identity").(core.Identity) }

func decode(c *gin.Context, target any) error {
	if c.ContentType() != "application/json" {
		return fmt.Errorf("%w: Content-Type must be application/json", core.ErrValidation)
	}
	decoder := json.NewDecoder(c.Request.Body)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return err
		}
		return fmt.Errorf("%w: body must be a JSON object with valid fields", core.ErrValidation)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return err
		}
		return fmt.Errorf("%w: body must contain exactly one JSON object", core.ErrValidation)
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return fmt.Errorf("%w: body must be a JSON object", core.ErrValidation)
	}
	// Explicit null must not silently turn a string into its zero value or a
	// PATCH field into an omitted field. Clients can omit optional properties.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("%w: body must be a JSON object", core.ErrValidation)
	}
	for _, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("%w: fields cannot be null; omit optional fields instead", core.ErrValidation)
		}
	}
	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: body must be a JSON object with valid fields", core.ErrValidation)
	}
	return nil
}

func resourceID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%w: id must be a positive integer", core.ErrValidation)
	}
	return id, nil
}

func pagination(c *gin.Context) (core.Page, error) {
	page := core.Page{Limit: 20}
	if raw, exists := c.GetQuery("limit"); exists {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			return page, fmt.Errorf("%w: limit must be between 1 and 100", core.ErrValidation)
		}
		page.Limit = limit
	}
	if raw, exists := c.GetQuery("before"); exists {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return page, fmt.Errorf("%w: before must be a positive integer", core.ErrValidation)
		}
		page.Before = id
	}
	return page, nil
}
