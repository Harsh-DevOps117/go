package config

import (
	"strings"
	"testing"
)

func baseline(t *testing.T) {
	t.Helper()
	for _, key := range []string{"APP_ENV", "PORT", "AUTO_MIGRATE", "REDIS_URL", "TRUSTED_PROXIES"} {
		t.Setenv(key, "")
	}
	t.Setenv("JWT_SECRET", strings.Repeat("a", 64))
	t.Setenv("DATABASE_URL", "postgres://example/test")
}

func TestConfigurationDefaults(t *testing.T) {
	baseline(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Address != ":3000" || !c.AutoMigrate || len(c.TrustedProxies) != 0 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	t.Setenv("APP_ENV", "production")
	c, err = Load()
	if err != nil || c.AutoMigrate {
		t.Fatal("production must require explicit migrations", err)
	}
}

func TestRejectsInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"JWT_SECRET", "secret"}, {"JWT_SECRET", "replace-with-at-least-32-random-bytes"},
		{"DATABASE_URL", ""}, {"PORT", "0"}, {"PORT", "65536"}, {"PORT", "abc"},
		{"APP_ENV", "typo"}, {"AUTO_MIGRATE", "maybe"}, {"TRUSTED_PROXIES", "anything"},
	} {
		t.Run(tc.key+"/"+tc.value, func(t *testing.T) {
			baseline(t)
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}

func TestTrustedProxies(t *testing.T) {
	baseline(t)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1, 10.0.0.0/24")
	c, err := Load()
	if err != nil || len(c.TrustedProxies) != 2 {
		t.Fatal(c.TrustedProxies, err)
	}
}
