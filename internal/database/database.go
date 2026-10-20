// Package database owns connection settings and versioned PostgreSQL migrations.
package database

import (
	"context"
	"embed"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

//go:embed migrations/*.sql
var migrations embed.FS

const LatestVersion = 1

func Open(ctx context.Context, url string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(url), &gorm.Config{
		TranslateError: true,
		// SQL parameters can contain credentials or personal data. Application
		// logging reports operation failures without logging queries or values.
		Logger:               logger.Default.LogMode(logger.Silent),
		DisableAutomaticPing: true,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	pool, err := db.DB()
	if err != nil {
		return nil, err
	}
	pool.SetMaxOpenConns(20)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(30 * time.Minute)
	pool.SetConnMaxIdleTime(5 * time.Minute)
	if err := pool.PingContext(ctx); err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("connect database: %w", err)
	}
	return db, nil
}

func Migrate(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize migrations across replicas. The transaction releases this
		// PostgreSQL advisory lock even if a migration fails or the process exits.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(741923601)").Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now()
		)`).Error; err != nil {
			return err
		}
		for version := 1; version <= LatestVersion; version++ {
			var count int64
			if err := tx.Table("schema_migrations").Where("version = ?", version).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			sql, err := migrations.ReadFile(fmt.Sprintf("migrations/%03d.sql", version))
			if err != nil {
				return err
			}
			if err := tx.Exec(string(sql)).Error; err != nil {
				return fmt.Errorf("migration %d: %w", version, err)
			}
			if err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func CheckVersion(ctx context.Context, db *gorm.DB) error {
	var version int
	if err := db.WithContext(ctx).Raw("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version).Error; err != nil {
		return fmt.Errorf("check migrations: %w", err)
	}
	if version != LatestVersion {
		return fmt.Errorf("database schema version is %d; expected %d; run the migrate command", version, LatestVersion)
	}
	return nil
}
