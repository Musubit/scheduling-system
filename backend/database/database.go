package database

import (
	"log"
	"os"
	"path/filepath"

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
// The database is stored at {baseDir}/resources/schedule.db.
// baseDir: path to the executable directory (or project root in dev mode).
// If baseDir is empty, it falls back to the working directory.
//
// It also records the resolved database path in DBPath for use by backup/restore.
func InitDB(baseDir string) (*GormAdapter, error) {
	if baseDir == "" {
		baseDir = resolveBaseDir()
	}

	dbDir := filepath.Join(baseDir, "resources")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dbDir, "schedule.db")

	// Migration: if old scheduling.db exists at baseDir, rename it
	oldPath := filepath.Join(baseDir, "scheduling.db")
	if _, err := os.Stat(oldPath); err == nil {
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			if renameErr := os.Rename(oldPath, dbPath); renameErr == nil {
				log.Printf("Database: migrated %s → %s", oldPath, dbPath)
			}
		}
	}

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
	)
	if err != nil {
		return nil, err
	}

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

// resolveBaseDir returns the executable directory, falling back to working directory.
func resolveBaseDir() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if dir != "" {
			return dir
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}
