package database

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"

	"scheduling-system/backend/appenv"
	"scheduling-system/backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the database interface injected into services.
// It exposes the subset of GORM methods actually used by the codebase,
// enabling testability through mock implementations.
type DB interface {
	Find(dest interface{}, conds ...interface{}) DB
	Create(value interface{}) DB
	Save(value interface{}) DB
	Delete(value interface{}, conds ...interface{}) DB
	Where(query interface{}, args ...interface{}) DB
	Preload(query string, args ...interface{}) DB
	First(dest interface{}, conds ...interface{}) DB
	Model(value interface{}) DB
	Order(value interface{}) DB
	Unscoped() DB
	Count(count *int64) DB
	Transaction(fc func(tx DB) error) error
	AutoMigrate(dst ...interface{}) error
	Error() error
}

// GormAdapter wraps *gorm.DB to implement the DB interface.
type GormAdapter struct {
	db *gorm.DB
}

// ensure GormAdapter implements DB at compile time.
var _ DB = (*GormAdapter)(nil)

func (g *GormAdapter) Find(dest interface{}, conds ...interface{}) DB {
	return &GormAdapter{db: g.db.Find(dest, conds...)}
}

func (g *GormAdapter) Create(value interface{}) DB {
	return &GormAdapter{db: g.db.Create(value)}
}

func (g *GormAdapter) Save(value interface{}) DB {
	return &GormAdapter{db: g.db.Save(value)}
}

func (g *GormAdapter) Delete(value interface{}, conds ...interface{}) DB {
	return &GormAdapter{db: g.db.Delete(value, conds...)}
}

func (g *GormAdapter) Where(query interface{}, args ...interface{}) DB {
	return &GormAdapter{db: g.db.Where(query, args...)}
}

func (g *GormAdapter) Preload(query string, args ...interface{}) DB {
	return &GormAdapter{db: g.db.Preload(query, args...)}
}

func (g *GormAdapter) First(dest interface{}, conds ...interface{}) DB {
	return &GormAdapter{db: g.db.First(dest, conds...)}
}

func (g *GormAdapter) Model(value interface{}) DB {
	return &GormAdapter{db: g.db.Model(value)}
}

func (g *GormAdapter) Order(value interface{}) DB {
	return &GormAdapter{db: g.db.Order(value)}
}

func (g *GormAdapter) Unscoped() DB {
	return &GormAdapter{db: g.db.Unscoped()}
}

func (g *GormAdapter) Count(count *int64) DB {
	return &GormAdapter{db: g.db.Count(count)}
}

func (g *GormAdapter) Transaction(fc func(tx DB) error) error {
	return g.db.Transaction(func(tx *gorm.DB) error {
		return fc(&GormAdapter{db: tx})
	})
}

func (g *GormAdapter) AutoMigrate(dst ...interface{}) error {
	return g.db.AutoMigrate(dst...)
}

func (g *GormAdapter) Error() error {
	return g.db.Error
}

// InitDB initializes the SQLite database and runs auto-migration.
// The database is stored at {resourcesDir}/schedule.db.
// resourcesDir is the writable directory (e.g., appenv.ResourcesDir()).
func InitDB(resourcesDir string) (*GormAdapter, error) {
	if resourcesDir == "" {
		resourcesDir = appenv.ResourcesDir()
	}

	if err := os.MkdirAll(resourcesDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(resourcesDir, "schedule.db")

	gormDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}

	adapter := &GormAdapter{db: gormDB}

	// Auto-migrate all models
	err = adapter.AutoMigrate(
		&models.Course{},
		&models.Teacher{},
		&models.Classroom{},
		&models.ClassGroup{},
		&models.ScheduleEntry{},
		&models.Semester{},
		&models.Setting{},
		&models.ScheduleSnapshot{},
		&models.SnapshotDetail{},
		&models.TeachingTask{},
		&models.TeachingTaskClass{},
		&models.Department{},
		&models.ScheduleVersion{},
		&models.ScheduleVersionEntry{},
	)
	if err != nil {
		return nil, err
	}

	// One-time migration: import departments from legacy settings JSON if table is empty
	MigrateDepartmentsFromSettings(adapter)

	// Seed default data if empty
	SeedData(adapter)

	SetDBPath(dbPath)
	log.Printf("Database initialized: %s", dbPath)
	return adapter, nil
}

var (
	activeDBPath string
)

// SetDBPath records the currently active database file path.
func SetDBPath(path string) {
	activeDBPath = path
}

// GetDBPath returns the path to the currently active database file.
func GetDBPath() string {
	return activeDBPath
}

// MigrateDepartmentsFromSettings imports departments from the legacy
// settings JSON (key="departments") into the new departments table.
// Safe to call on every startup: it only imports if the table is empty.
func MigrateDepartmentsFromSettings(db DB) {
	var count int64
	db.Model(&models.Department{}).Count(&count)
	if count > 0 {
		return // already migrated
	}

	var setting models.Setting
	if err := db.Where("key = ?", "departments").First(&setting).Error(); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return // no legacy data, nothing to do
		}
		log.Printf("[migrate] failed to read departments setting: %v", err)
		return
	}

	if setting.Value == "" {
		return
	}

	var raw []map[string]interface{}
	if err := json.Unmarshal([]byte(setting.Value), &raw); err != nil {
		log.Printf("[migrate] failed to parse departments JSON: %v", err)
		return
	}

	if len(raw) == 0 {
		return
	}

	depts := make([]models.Department, 0, len(raw))
	for _, item := range raw {
		code, _ := item["code"].(string)
		name, _ := item["name"].(string)
		if code == "" || name == "" {
			continue
		}
		depts = append(depts, models.Department{Code: code, Name: name})
	}

	if len(depts) == 0 {
		return
	}

	if err := db.Create(&depts).Error(); err != nil {
		log.Printf("[migrate] failed to import departments: %v", err)
		return
	}

	// Delete the legacy settings entry to avoid future confusion
	db.Where("key = ?", "departments").Delete(&models.Setting{})
	log.Printf("[migrate] imported %d departments from legacy settings", len(depts))
}

