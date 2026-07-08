package database

import (
	"log"

	"scheduling-system/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB initializes the SQLite database and runs auto-migration.
// Uses github.com/glebarez/sqlite which is a pure Go SQLite driver (no CGO required).
func InitDB() error {
	var err error
	DB, err = gorm.Open(sqlite.Open("scheduling.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return err
	}

	// Auto-migrate all models
	err = DB.AutoMigrate(
		&models.Course{},
		&models.Teacher{},
		&models.Classroom{},
		&models.ClassGroup{},
		&models.ScheduleEntry{},
		&models.Semester{},
		&models.Setting{},
	)
	if err != nil {
		return err
	}

	// Seed default data if empty
	SeedData()

	log.Println("Database initialized successfully")
	return nil
}
