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
		log.Printf("Warning: Could not open database: %v", err)
		// Don't fail — allow the app to start without a database for development
		return nil
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
		log.Printf("Warning: Auto-migration failed: %v", err)
		return nil
	}

	// Seed default data if empty
	seedDefaults()

	log.Println("Database initialized successfully")
	return nil
}

func seedDefaults() {
	if DB == nil {
		return
	}

	// Seed default semester if none exist
	var count int64
	DB.Model(&models.Semester{}).Count(&count)
	if count == 0 {
		DB.Create(&models.Semester{
			Name:     "2025-2026 第二学期",
			IsActive: true,
		})
	}

	// Seed default settings
	DB.Model(&models.Setting{}).Count(&count)
	if count == 0 {
		DB.Create(&models.Setting{
			Key:   "iterations",
			Value: "5000",
		})
		DB.Create(&models.Setting{
			Key:   "max_daily_hours",
			Value: "8",
		})
		DB.Create(&models.Setting{
			Key:   "buffer_minutes",
			Value: "10",
		})
	}
}
