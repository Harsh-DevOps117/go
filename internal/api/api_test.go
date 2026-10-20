package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"harshDevops117/internal/core"
	"harshDevops117/internal/ratelimit"
	"harshDevops117/internal/testutil"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func fixture(t *testing.T, db *gorm.DB, mutate func(*Options)) *gin.Engine {
	t.Helper()
	auth, err := core.NewAuth(db, "test-only-secret-containing-more-than-32-bytes", time.Minute, time.Hour, bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{Auth: auth, Resources: core.NewResources(db), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Ready: func(context.Context) error { return nil }}
	if mutate != nil {
		mutate(&opts)
	}
	r, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func request(t *testing.T, h http.Handler, method, path, body, token string, want int) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != want {
		t.Fatalf("%s %s: expected %d, got %d: %s", method, path, want, w.Code, w.Body.String())
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing request ID")
	}
	return w
}

func data[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var body struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Data
}

func signup(t *testing.T, r http.Handler, email string) core.Tokens {
	t.Helper()
	body := fmt.Sprintf(`{"username":"Example","email":%q,"password":"a long test password"}`, email)
	w := request(t, r, "POST", "/api/v1/auth/register", body, "", 201)
	if strings.Contains(w.Body.String(), "password") {
		t.Fatal("password hash leaked")
	}
	w = request(t, r, "POST", "/api/v1/auth/login", fmt.Sprintf(`{"email":%q,"password":"a long test password"}`, email), "", 200)
	if strings.Contains(w.Body.String(), "password") {
		t.Fatal("password hash leaked in login")
	}
	return data[core.Tokens](t, w)
}

func TestHTTPValidationAndHealth(t *testing.T) {
	r := fixture(t, nil, nil)
	request(t, r, "GET", "/health/live", "", "", 200)
	request(t, r, "GET", "/health/ready", "", "", 200)
	request(t, r, "GET", "/unknown", "", "", 404)
	request(t, r, "PUT", "/api/v1/auth/register", "{}", "", 405)
	request(t, r, "GET", "/api/v1/notes", "", "", 401)
	request(t, r, "GET", "/api/v1/notes", "", "invalid-token", 401)
	for _, body := range []string{`{`, `[]`, `null`, `{}`, `{"extra":true}`, `{} {}`, `{"email":12}`} {
		request(t, r, "POST", "/api/v1/auth/register", body, "", 400)
	}
	request(t, r, "POST", "/api/v1/auth/register", `{"username":"`+strings.Repeat("a", 1<<20)+`"}`, "", 413)
	raw := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, raw)
	if w.Code != 400 {
		t.Fatal("missing content type accepted")
	}
	bad := fixture(t, nil, func(o *Options) { o.Ready = func(context.Context) error { return fmt.Errorf("database-secret") } })
	w = request(t, bad, "GET", "/health/ready", "", "", 503)
	if strings.Contains(w.Body.String(), "database-secret") {
		t.Fatal("readiness leaked error")
	}
}

func TestAuthRateLimitAndProxySpoofing(t *testing.T) {
	r := fixture(t, nil, func(o *Options) { o.AuthLimit = ratelimit.New(1, time.Minute, "test", nil, nil) })
	for i, want := range []int{400, 429} {
		raw := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader("{}"))
		raw.Header.Set("Content-Type", "application/json")
		raw.Header.Set("X-Forwarded-For", fmt.Sprintf("10.0.0.%d", i+1))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, raw)
		if w.Code != want {
			t.Fatal("spoofed forwarding header bypassed limit", w.Code)
		}
		if want == 429 && w.Header().Get("Retry-After") == "" {
			t.Fatal("missing retry header")
		}
	}
}

func TestHTTPAuthenticationIntegration(t *testing.T) {
	db := testutil.Database(t)
	r := fixture(t, db, nil)
	tokens := signup(t, r, "http@example.com")
	request(t, r, "POST", "/api/v1/auth/register", `{"username":"Example","email":"HTTP@example.com","password":"a long test password"}`, "", 409)
	request(t, r, "POST", "/api/v1/auth/login", `{"email":"http@example.com","password":"incorrect"}`, "", 401)
	request(t, r, "GET", "/api/v1/me", "", tokens.AccessToken, 200)
	w := request(t, r, "POST", "/api/v1/auth/refresh", fmt.Sprintf(`{"refresh_token":%q}`, tokens.RefreshToken), "", 200)
	rotated := data[core.Tokens](t, w)
	request(t, r, "POST", "/api/v1/auth/refresh", fmt.Sprintf(`{"refresh_token":%q}`, tokens.RefreshToken), "", 401)
	request(t, r, "POST", "/api/v1/auth/logout", "", "", 401)
	w = request(t, r, "POST", "/api/v1/auth/logout", "", rotated.AccessToken, 204)
	if w.Body.Len() != 0 {
		t.Fatal("204 response contained a body")
	}
	request(t, r, "GET", "/api/v1/me", "", tokens.AccessToken, 401)
	request(t, r, "GET", "/api/v1/me", "", rotated.AccessToken, 401)
	request(t, r, "POST", "/api/v1/auth/refresh", fmt.Sprintf(`{"refresh_token":%q}`, rotated.RefreshToken), "", 401)
}

func TestHTTPResourcesIntegration(t *testing.T) {
	db := testutil.Database(t)
	r := fixture(t, db, nil)
	a, b := signup(t, r, "owner@example.com"), signup(t, r, "other@example.com")
	w := request(t, r, "POST", "/api/v1/notes", `{"title":"Note","content":"Private"}`, a.AccessToken, 201)
	note := data[core.Note](t, w)
	notePath := fmt.Sprintf("/api/v1/notes/%d", note.ID)
	w = request(t, r, "POST", "/api/v1/alarms", fmt.Sprintf(`{"note_id":%d,"alarm_time":%q}`, note.ID, time.Now().Add(time.Hour).UTC().Format(time.RFC3339)), a.AccessToken, 201)
	alarm := data[core.Alarm](t, w)
	alarmPath := fmt.Sprintf("/api/v1/alarms/%d", alarm.ID)
	for _, path := range []string{notePath, alarmPath} {
		request(t, r, "GET", path, "", a.AccessToken, 200)
		// Repeat reads to catch any reintroduction of an unsafe cache-hit path.
		request(t, r, "GET", path, "", b.AccessToken, 404)
		request(t, r, "GET", path, "", b.AccessToken, 404)
		request(t, r, "DELETE", path, "", b.AccessToken, 404)
	}
	request(t, r, "PATCH", notePath, `{"title":"stolen"}`, b.AccessToken, 404)
	request(t, r, "PATCH", alarmPath, fmt.Sprintf(`{"alarm_time":%q}`, time.Now().Add(time.Hour).UTC().Format(time.RFC3339)), b.AccessToken, 404)
	request(t, r, "POST", "/api/v1/alarms", fmt.Sprintf(`{"note_id":%d,"alarm_time":%q}`, note.ID, time.Now().Add(time.Hour).UTC().Format(time.RFC3339)), b.AccessToken, 404)
	w = request(t, r, "PATCH", notePath, `{"content":""}`, a.AccessToken, 200)
	if data[core.Note](t, w).Content != "" {
		t.Fatal("cannot clear content")
	}
	request(t, r, "PATCH", notePath, `{}`, a.AccessToken, 400)
	request(t, r, "PATCH", notePath, `{"title":"Valid","content":null}`, a.AccessToken, 400)
	request(t, r, "PATCH", alarmPath, `{}`, a.AccessToken, 400)
	for _, path := range []string{"/api/v1/notes/abc", "/api/v1/notes/-1", "/api/v1/notes?limit=101", "/api/v1/notes?before=-1", "/api/v1/alarms?status=invalid"} {
		request(t, r, "GET", path, "", a.AccessToken, 400)
	}
	w = request(t, r, "GET", "/api/v1/notes?limit=1", "", a.AccessToken, 200)
	if len(data[core.List[core.Note]](t, w).Items) != 1 {
		t.Fatal("missing note")
	}
	w = request(t, r, "GET", "/api/v1/alarms", "", b.AccessToken, 200)
	if len(data[core.List[core.Alarm]](t, w).Items) != 0 {
		t.Fatal("foreign alarms leaked")
	}
	request(t, r, "DELETE", notePath, "", a.AccessToken, 204)
	request(t, r, "GET", alarmPath, "", a.AccessToken, 404)
	request(t, r, "GET", notePath, "", a.AccessToken, 404)
}

func TestDatabaseFailuresAreNotSuccessfulIntegration(t *testing.T) {
	db := testutil.Database(t)
	r := fixture(t, db, nil)
	tokens := signup(t, r, "outage@example.com")
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	w := request(t, r, "GET", "/api/v1/notes", "", tokens.AccessToken, 500)
	if strings.Contains(w.Body.String(), "closed") {
		t.Fatal("database error leaked")
	}
}
