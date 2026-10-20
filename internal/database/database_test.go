package database_test

import (
	"context"
	"testing"

	"harshDevops117/internal/database"
	"harshDevops117/internal/testutil"
)

func TestMigrationsAreRepeatableIntegration(t *testing.T) {
	db := testutil.Database(t)
	ctx := context.Background()
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := database.CheckVersion(ctx, db); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Table("schema_migrations").Count(&count).Error; err != nil || count != int64(database.LatestVersion) {
		t.Fatal(count, err)
	}
	if err := db.Exec("INSERT INTO schema_migrations(version) VALUES (999)").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.CheckVersion(ctx, db); err == nil {
		t.Fatal("accepted a schema from a newer application")
	}
}
