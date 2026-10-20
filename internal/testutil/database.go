// Package testutil provisions isolated PostgreSQL schemas for integration tests.
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"
	"harshDevops117/internal/database"
)

func Database(t *testing.T) *gorm.DB {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("integration test: set TEST_DATABASE_URL or run make test-integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	admin, err := database.Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	adminPool, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = adminPool.Close() })
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	schema := "test_" + hex.EncodeToString(buf)
	// The identifier is generated entirely from hex, never from user input.
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := admin.WithContext(ctx).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Errorf("cleanup schema: %v", err)
		}
	})
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, err := database.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	return db
}
