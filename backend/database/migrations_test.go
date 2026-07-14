package database

import (
	"testing"
	"time"

	"scheduling-system/backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newMigrationTestDB(t *testing.T) DB {
	t.Helper()
	raw, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	adapter := &GormAdapter{db: raw}
	if err := adapter.AutoMigrate(&models.SchemaMigration{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return adapter
}

func TestEnsureMigrationApplied_InsertsOnce(t *testing.T) {
	db := newMigrationTestDB(t)
	if err := EnsureMigrationApplied(db, "v0.5.5-prep"); err != nil {
		t.Fatalf("first call: %v", err)
	}
	var rows []models.SchemaMigration
	if err := db.Find(&rows).Error(); err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].Version != "v0.5.5-prep" {
		t.Fatalf("want version v0.5.5-prep, got %s", rows[0].Version)
	}
	if time.Since(rows[0].AppliedAt) > time.Minute {
		t.Fatalf("AppliedAt too old: %v", rows[0].AppliedAt)
	}
}

func TestEnsureMigrationApplied_Idempotent(t *testing.T) {
	db := newMigrationTestDB(t)
	for i := 0; i < 5; i++ {
		if err := EnsureMigrationApplied(db, "v0.5.5-prep"); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	var count int64
	if err := db.Model(&models.SchemaMigration{}).Count(&count).Error(); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 row after 5 calls, got %d", count)
	}
}

func TestEnsureMigrationApplied_MultipleVersions(t *testing.T) {
	db := newMigrationTestDB(t)
	for _, v := range []string{"v0.5.5-prep", "v0.5.5-dbgate", "v0.5.5-release"} {
		if err := EnsureMigrationApplied(db, v); err != nil {
			t.Fatalf("apply %s: %v", v, err)
		}
	}
	var count int64
	if err := db.Model(&models.SchemaMigration{}).Count(&count).Error(); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Fatalf("want 3 rows, got %d", count)
	}
}