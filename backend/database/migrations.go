package database

import (
	"time"

	"scheduling-system/backend/models"
)

func EnsureMigrationApplied(db DB, name string) error {
	rec := models.SchemaMigration{
		Version:   name,
		AppliedAt: time.Now(),
	}
	return db.Where("version = ?", name).FirstOrCreate(&rec).Error()
}