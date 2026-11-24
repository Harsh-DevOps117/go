package core

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestJWTv5APIContract pins the golang-jwt/jwt v5 surface that Auth depends on.
// Auth is written against v5: parser state is private and set through options,
// registered claims are exposed through jwt.Claims getters, and signing
// methods exchange []byte signatures. Reverting the dependency to v4 changes
// each of these shapes, so this test fails when the migration is undone.
func TestJWTv5APIContract(t *testing.T) {
	parser := reflect.TypeOf(jwt.Parser{})
	for i := 0; i < parser.NumField(); i++ {
		if field := parser.Field(i); field.PkgPath == "" {
			t.Fatalf("jwt.Parser.%s is exported; jwt v5 keeps parser state private and configures it through options", field.Name)
		}
	}

	claims := reflect.TypeOf((*jwt.Claims)(nil)).Elem()
	for _, name := range []string{"GetExpirationTime", "GetIssuedAt", "GetNotBefore", "GetIssuer", "GetSubject", "GetAudience"} {
		if _, ok := claims.MethodByName(name); !ok {
			t.Fatalf("jwt.Claims.%s is missing; jwt v5 exposes claims through getters", name)
		}
	}

	audience, ok := reflect.TypeOf(jwt.RegisteredClaims{}).FieldByName("Audience")
	if !ok || audience.Type != reflect.TypeOf(jwt.ClaimStrings{}) {
		t.Fatalf("jwt.RegisteredClaims.Audience is %v; want jwt.ClaimStrings in v5", audience.Type)
	}

	sign, ok := reflect.TypeOf((*jwt.SigningMethod)(nil)).Elem().MethodByName("Sign")
	if !ok || sign.Type.NumOut() != 2 || sign.Type.Out(0) != reflect.TypeOf([]byte(nil)) {
		t.Fatalf("jwt.SigningMethod.Sign has signature %v; want ([]byte, error) in v5", sign.Type)
	}

	// A token minted by Auth must satisfy the same v5 parser options Auth uses
	// to verify it: HS256 only, matching issuer/audience, required exp, valid iat.
	a := testAuth(t, nil)
	now := time.Now().UTC()
	session := Session{ID: strings.Repeat("a", 32), ExpiresAt: now.Add(time.Hour)}
	tokens, err := a.issue(User{ID: 1, Email: "guard@example.com"}, session, "refresh-token")
	if err != nil {
		t.Fatal(err)
	}
	parsed := new(jwt.RegisteredClaims)
	if _, err := jwt.ParseWithClaims(tokens.AccessToken, parsed, func(*jwt.Token) (any, error) { return []byte(testSecret), nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("noteflow"),
		jwt.WithAudience("noteflow-api"), jwt.WithExpirationRequired(), jwt.WithIssuedAt()); err != nil {
		t.Fatalf("issued access token does not satisfy the v5 parser options: %v", err)
	}
	if parsed.Subject != "1" || parsed.ID != session.ID || parsed.IssuedAt == nil || parsed.ExpiresAt == nil {
		t.Fatalf("unexpected registered claims: %+v", parsed)
	}
	if len(parsed.Audience) != 1 || parsed.Audience[0] != "noteflow-api" {
		t.Fatalf("unexpected audience: %v", parsed.Audience)
	}

	// The v5 options are load-bearing: a correctly signed token carrying the
	// wrong audience or issuer is still rejected without touching the database.
	for name, claims := range map[string]jwt.RegisteredClaims{
		"wrong_audience": {
			Issuer: "noteflow", Audience: jwt.ClaimStrings{"another-api"},
			Subject: "1", ID: session.ID,
			IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
		"wrong_issuer": {
			Issuer: "another-app", Audience: jwt.ClaimStrings{"noteflow-api"},
			Subject: "1", ID: session.ID,
			IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
	} {
		raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.Authenticate(context.Background(), raw); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("%s: got %v, want ErrUnauthorized", name, err)
		}
	}
}
