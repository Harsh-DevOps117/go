// Package config reads and validates configuration before any listeners start.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment    string
	Address        string
	DatabaseURL    string
	RedisURL       string
	JWTSecret      string
	TrustedProxies []string
	AutoMigrate    bool
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
}

func Load() (Config, error) {
	c := Config{
		Environment: value("APP_ENV", "development"),
		Address:     ":" + value("PORT", "3000"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		AccessTTL:   15 * time.Minute,
		RefreshTTL:  7 * 24 * time.Hour,
	}
	if c.Environment != "development" && c.Environment != "production" && c.Environment != "test" {
		return c, fmt.Errorf("APP_ENV must be development, production, or test")
	}
	port, err := strconv.Atoi(strings.TrimPrefix(c.Address, ":"))
	if err != nil || port < 1 || port > 65535 {
		return c, fmt.Errorf("PORT must be between 1 and 65535")
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	if len(c.JWTSecret) < 32 || strings.Contains(strings.ToLower(c.JWTSecret), "replace") {
		return c, fmt.Errorf("JWT_SECRET must contain at least 32 bytes of random secret material")
	}
	c.AutoMigrate, err = strconv.ParseBool(value("AUTO_MIGRATE", strconv.FormatBool(c.Environment != "production")))
	if err != nil {
		return c, fmt.Errorf("AUTO_MIGRATE must be true or false")
	}
	if raw := os.Getenv("TRUSTED_PROXIES"); raw != "" {
		for _, proxy := range strings.Split(raw, ",") {
			proxy = strings.TrimSpace(proxy)
			if _, _, err := net.ParseCIDR(proxy); err != nil && net.ParseIP(proxy) == nil {
				return c, fmt.Errorf("TRUSTED_PROXIES must contain IP addresses or CIDR networks")
			}
			c.TrustedProxies = append(c.TrustedProxies, proxy)
		}
	}
	return c, nil
}

func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
