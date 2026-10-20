package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"harshDevops117/internal/api"
	"harshDevops117/internal/config"
	"harshDevops117/internal/core"
	"harshDevops117/internal/database"
	"harshDevops117/internal/ratelimit"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	if command != "serve" && command != "migrate" || len(os.Args) > 2 {
		return errors.New("usage: noteflow [serve|migrate]")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	startup, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelStartup()

	db, err := database.Open(startup, cfg.DatabaseURL)
	if err != nil {
		return err
	}

	pool, err := db.DB()

	if err != nil {
		return err
	}

	defer pool.Close()

	if command == "migrate" || cfg.AutoMigrate {
		if err := database.Migrate(startup, db); err != nil {
			return err
		}
	}

	if err := database.CheckVersion(startup, db); err != nil {
		return err
	}

	if command == "migrate" {
		logger.Info("database migrations applied")
		return nil
	}

	var redisClient *redis.Client
	if cfg.RedisURL != "" {
		opts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			return errors.New("REDIS_URL must be a valid redis:// or rediss:// URL")
		}

		opts.DialTimeout = time.Second
		opts.ReadTimeout = 200 * time.Millisecond
		opts.WriteTimeout = 200 * time.Millisecond
		opts.ContextTimeoutEnabled = true
		opts.MaxRetries = -1
		redisClient = redis.NewClient(opts)
		defer redisClient.Close()

	}

	auth, err := core.NewAuth(db, cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL, 12)
	if err != nil {
		return err
	}

	resources := core.NewResources(db)
	router, err := api.New(api.Options{
		Auth: auth, Resources: resources, Logger: logger, Ready: pool.PingContext,
		GeneralLimit:   ratelimit.New(120, time.Minute, "api", redisClient, logger),
		AuthLimit:      ratelimit.New(10, time.Minute, "auth", redisClient, logger),
		TrustedProxies: cfg.TrustedProxies,
	})
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr: cfg.Address, Handler: router,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10,
	}

	stop, cancelSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelSignals()
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	workers.Add(1)

	go func() {
		defer workers.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(workerCtx, 5*time.Second)
				_, err := resources.Tick(ctx)
				cancel()
				if err != nil && workerCtx.Err() == nil {
					logger.Error("reminder worker failed", "error", err)
				}
			}
		}
	}()

	errorsCh := make(chan error, 1)
	go func() { errorsCh <- httpServer.ListenAndServe() }()
	logger.Info("server starting", "address", cfg.Address, "environment", cfg.Environment)
	var serveErr error
	select {
	case <-stop.Done():
		logger.Info("shutdown requested")
	case serveErr = <-errorsCh:
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
	}
	cancelWorker()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdown); err != nil {
		_ = httpServer.Close()
		serveErr = errors.Join(serveErr, fmt.Errorf("graceful shutdown: %w", err))
	}
	workers.Wait()
	return serveErr
}
