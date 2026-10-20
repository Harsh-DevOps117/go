package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"harshDevops117/internal/testutil"
)

const testSecret = "test-only-signing-key-with-at-least-32-bytes"

func testAuth(t *testing.T, db *gorm.DB) *Auth {
	t.Helper()
	a, err := NewAuth(db, testSecret, 15*time.Minute, 24*time.Hour, bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func register(t *testing.T, a *Auth, email string) User {
	t.Helper()
	user, err := a.Register(context.Background(), Registration{Username: "Example User", Email: email, Password: "a long test password"})
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func login(t *testing.T, a *Auth, email string) Tokens {
	t.Helper()
	tokens, err := a.Login(context.Background(), Credentials{Email: email, Password: "a long test password"})
	if err != nil {
		t.Fatal(err)
	}
	return tokens
}

func TestInvalidAccessTokens(t *testing.T) {
	a := testAuth(t, nil)
	for _, name := range []string{"expired", "missing_expiry", "missing_issued_at", "bad_subject", "bad_id", "wrong_issuer", "wrong_audience", "wrong_algorithm", "wrong_signature", "future_issued_at", "wrong_claim_type"} {
		t.Run(name, func(t *testing.T) {
			claims := jwt.MapClaims{"iss": "noteflow", "aud": "noteflow-api", "sub": "1", "jti": strings.Repeat("a", 32), "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix()}
			method := jwt.SigningMethodHS256
			secret := testSecret
			switch name {
			case "expired":
				claims["exp"] = time.Now().Add(-time.Minute).Unix()
			case "missing_expiry":
				delete(claims, "exp")
			case "missing_issued_at":
				delete(claims, "iat")
			case "bad_subject":
				claims["sub"] = "-1"
			case "bad_id":
				claims["jti"] = ""
			case "wrong_issuer":
				claims["iss"] = "another-app"
			case "wrong_audience":
				claims["aud"] = "another-api"
			case "wrong_algorithm":
				method = jwt.SigningMethodHS384
			case "wrong_signature":
				secret = "wrong-secret"
			case "future_issued_at":
				claims["iat"] = time.Now().Add(time.Hour).Unix()
			case "wrong_claim_type":
				claims["sub"] = 12
			}
			raw, err := jwt.NewWithClaims(method, claims).SignedString([]byte(secret))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := a.Authenticate(context.Background(), raw); !errors.Is(err, ErrUnauthorized) {
				t.Fatalf("got %v", err)
			}
		})
	}
	if _, err := a.Authenticate(context.Background(), "not.a.token"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
}

func TestRegistrationValidation(t *testing.T) {
	a := testAuth(t, nil)
	for _, input := range []Registration{
		{Username: "ab", Email: "a@example.com", Password: "a long test password"},
		{Username: "Valid", Email: "invalid", Password: "a long test password"},
		{Username: "Valid", Email: "Name <a@example.com>", Password: "a long test password"},
		{Username: "Valid", Email: "a@example.com", Password: "short"},
		{Username: "Valid", Email: "a@example.com", Password: strings.Repeat("界", 25)},
	} {
		if _, err := a.Register(context.Background(), input); !errors.Is(err, ErrValidation) {
			t.Fatalf("got %v", err)
		}
	}
}

func TestAuthLifecycleIntegration(t *testing.T) {
	db := testutil.Database(t)
	a := testAuth(t, db)
	ctx := context.Background()
	user := register(t, a, " Alice@Example.com ")
	if user.Email != "alice@example.com" || user.PasswordHash == "a long test password" {
		t.Fatal("email or password was not normalized/hashed")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("a long test password")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Register(ctx, Registration{Username: "Other", Email: "ALICE@example.com", Password: "a long test password"}); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	for _, input := range []Credentials{{Email: user.Email, Password: "wrong"}, {Email: "missing@example.com", Password: "wrong"}} {
		if _, err := a.Login(ctx, input); !errors.Is(err, ErrUnauthorized) {
			t.Fatal(err)
		}
	}
	tokens := login(t, a, user.Email)
	id, err := a.Authenticate(ctx, tokens.AccessToken)
	if err != nil || id.UserID != user.ID {
		t.Fatal(id, err)
	}
	var session Session
	if err := db.First(&session, "id = ?", id.SessionID).Error; err != nil {
		t.Fatal(err)
	}
	if session.RefreshHash == tokens.RefreshToken || len(session.RefreshHash) != 64 {
		t.Fatal("refresh token stored in plaintext")
	}
	rotated, err := a.Refresh(ctx, tokens.RefreshToken)
	if err != nil || rotated.RefreshToken == tokens.RefreshToken {
		t.Fatal("rotation failed", err)
	}
	if _, err := a.Refresh(ctx, tokens.RefreshToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("replay accepted", err)
	}
	if err := a.Logout(ctx, id); err != nil {
		t.Fatal(err)
	}
	for _, access := range []string{tokens.AccessToken, rotated.AccessToken} {
		if _, err := a.Authenticate(ctx, access); !errors.Is(err, ErrUnauthorized) {
			t.Fatal("logged-out token accepted", err)
		}
	}
	if _, err := a.Refresh(ctx, rotated.RefreshToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("logged-out refresh accepted", err)
	}
}

func TestConcurrentRefreshIntegration(t *testing.T) {
	db := testutil.Database(t)
	a := testAuth(t, db)
	register(t, a, "race@example.com")
	tokens := login(t, a, "race@example.com")
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := a.Refresh(context.Background(), tokens.RefreshToken)
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, ErrUnauthorized) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("expected one refresh winner, got %d", successes.Load())
	}
}

func TestConcurrentRegistrationIntegration(t *testing.T) {
	db := testutil.Database(t)
	a := testAuth(t, db)
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := a.Register(context.Background(), Registration{Username: "Valid", Email: "unique@example.com", Password: "a long test password"})
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, ErrConflict) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("expected one registration, got %d", successes.Load())
	}
}

func TestExpiredSessionsIntegration(t *testing.T) {
	db := testutil.Database(t)
	a := testAuth(t, db)
	register(t, a, "expired@example.com")
	tokens := login(t, a, "expired@example.com")
	if err := db.Model(&Session{}).Where("refresh_hash = ?", hashRefresh(tokens.RefreshToken)).Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := a.Authenticate(context.Background(), tokens.AccessToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if _, err := a.Refresh(context.Background(), tokens.RefreshToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
}
