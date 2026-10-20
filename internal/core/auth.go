package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Auth struct {
	db         *gorm.DB
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	cost       int
	dummyHash  []byte
}

type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Registration struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	User         User   `json:"user"`
}

type Identity struct {
	UserID    int64
	SessionID string
}

func NewAuth(db *gorm.DB, secret string, accessTTL, refreshTTL time.Duration, cost int) (*Auth, error) {
	if len(secret) < 32 || accessTTL <= 0 || refreshTTL <= accessTTL || cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return nil, fmt.Errorf("invalid authentication configuration")
	}
	// Missing users still perform a bcrypt comparison, reducing the useful
	// timing difference between an unknown email and a wrong password.
	dummy, err := bcrypt.GenerateFromPassword([]byte("not-a-real-user-password"), cost)
	if err != nil {
		return nil, err
	}
	return &Auth{db: db, secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL, cost: cost, dummyHash: dummy}, nil
}

func normalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > 254 || !strings.Contains(email, ".") {
		return "", fmt.Errorf("%w: email must be a valid address", ErrValidation)
	}
	return email, nil
}

func (a *Auth) Register(ctx context.Context, input Registration) (User, error) {
	var user User
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return user, err
	}
	username := strings.TrimSpace(input.Username)
	if !utf8.ValidString(username) || utf8.RuneCountInString(username) < 3 || utf8.RuneCountInString(username) > 40 {
		return user, fmt.Errorf("%w: username must contain 3 to 40 characters", ErrValidation)
	}
	// bcrypt accepts at most 72 bytes, not 72 Unicode characters.
	if !utf8.ValidString(input.Password) || utf8.RuneCountInString(input.Password) < 12 || len(input.Password) > 72 {
		return user, fmt.Errorf("%w: password must contain at least 12 characters and at most 72 bytes", ErrValidation)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), a.cost)
	if err != nil {
		return user, fmt.Errorf("hash password: %w", err)
	}
	user = User{Username: username, Email: email, PasswordHash: string(hash)}
	if err := a.db.WithContext(ctx).Create(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return User{}, ErrConflict
		}
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (a *Auth) Login(ctx context.Context, input Credentials) (Tokens, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil || len(input.Password) == 0 || len(input.Password) > 72 {
		return Tokens{}, ErrUnauthorized
	}
	var user User
	err = a.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = bcrypt.CompareHashAndPassword(a.dummyHash, []byte(input.Password))
			return Tokens{}, ErrUnauthorized
		}
		return Tokens{}, fmt.Errorf("find user: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)) != nil {
		return Tokens{}, ErrUnauthorized
	}
	id, err := randomHex(16)
	if err != nil {
		return Tokens{}, err
	}
	refresh, err := newRefresh(id)
	if err != nil {
		return Tokens{}, err
	}
	session := Session{ID: id, UserID: user.ID, RefreshHash: hashRefresh(refresh), ExpiresAt: time.Now().UTC().Add(a.refreshTTL)}
	tokens, err := a.issue(user, session, refresh)
	if err != nil {
		return Tokens{}, err
	}
	if err := a.db.WithContext(ctx).Create(&session).Error; err != nil {
		return Tokens{}, fmt.Errorf("create session: %w", err)
	}
	return tokens, nil
}

func (a *Auth) issue(user User, session Session, refresh string) (Tokens, error) {
	now := time.Now().UTC()
	expires := now.Add(a.accessTTL)
	if session.ExpiresAt.Before(expires) {
		expires = session.ExpiresAt
	}
	claims := jwt.RegisteredClaims{
		Issuer: "noteflow", Audience: jwt.ClaimStrings{"noteflow-api"},
		Subject: strconv.FormatInt(user.ID, 10), ID: session.ID,
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expires),
	}
	access, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.secret)
	if err != nil {
		return Tokens{}, fmt.Errorf("sign access token: %w", err)
	}
	return Tokens{AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: int64(expires.Sub(now).Seconds()), User: user}, nil
}

func (a *Auth) Authenticate(ctx context.Context, raw string) (Identity, error) {
	claims := new(jwt.RegisteredClaims)
	token, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) { return a.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("noteflow"),
		jwt.WithAudience("noteflow-api"), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid || claims.IssuedAt == nil || len(claims.ID) != 32 {
		return Identity{}, ErrUnauthorized
	}
	uid, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || uid <= 0 {
		return Identity{}, ErrUnauthorized
	}
	var session Session
	err = a.db.WithContext(ctx).Where("id = ? AND user_id = ? AND expires_at > ?", claims.ID, uid, time.Now().UTC()).First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Identity{}, ErrUnauthorized
	}
	if err != nil {
		return Identity{}, fmt.Errorf("check session: %w", err)
	}
	return Identity{UserID: uid, SessionID: claims.ID}, nil
}

func (a *Auth) Refresh(ctx context.Context, raw string) (Tokens, error) {
	id, secret, ok := strings.Cut(raw, ".")
	if !ok || len(id) != 32 || len(secret) != 64 {
		return Tokens{}, ErrUnauthorized
	}
	if _, err := hex.DecodeString(id + secret); err != nil {
		return Tokens{}, ErrUnauthorized
	}
	refresh, err := newRefresh(id)
	if err != nil {
		return Tokens{}, err
	}
	var tokens Tokens
	err = a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session Session
		// Compare-and-swap makes each refresh token single-use, including when
		// two requests race. The session has a fixed lifetime, not an endless one.
		result := tx.Model(&session).Where("id = ? AND refresh_hash = ? AND expires_at > ?", id, hashRefresh(raw), time.Now().UTC()).
			Updates(map[string]any{"refresh_hash": hashRefresh(refresh), "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return fmt.Errorf("rotate session: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrUnauthorized
		}
		if err := tx.First(&session, "id = ?", id).Error; err != nil {
			return err
		}
		var user User
		if err := tx.First(&user, session.UserID).Error; err != nil {
			return err
		}
		var err error
		tokens, err = a.issue(user, session, refresh)
		return err
	})
	return tokens, err
}

func (a *Auth) Logout(ctx context.Context, identity Identity) error {
	// Access tokens are checked against this session on every request, so
	// deleting it revokes both access and refresh tokens immediately.
	return a.db.WithContext(ctx).Where("id = ? AND user_id = ?", identity.SessionID, identity.UserID).Delete(&Session{}).Error
}

func (a *Auth) Me(ctx context.Context, identity Identity) (User, error) {
	var user User
	err := a.db.WithContext(ctx).First(&user, identity.UserID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return user, ErrUnauthorized
	}
	return user, err
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func newRefresh(id string) (string, error) {
	secret, err := randomHex(32)
	if err != nil {
		return "", err
	}
	return id + "." + secret, nil
}

func hashRefresh(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
