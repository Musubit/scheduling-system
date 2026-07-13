package database

import (
	"encoding/json"
	"errors"
	"fmt"
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
	FirstOrCreate(dest interface{}, conds ...interface{}) DB
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

func (g *GormAdapter) FirstOrCreate(dest interface{}, conds ...interface{}) DB {
	if len(conds) > 0 {
		return &GormAdapter{db: g.db.Where(conds[0], conds[1:]...).FirstOrCreate(dest)}
	}
	return &GormAdapter{db: g.db.FirstOrCreate(dest)}
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

	// 先确保 settings 表存在 —— MigrateRoomDomainV055 依赖 settings 存储 flag。
	if err := adapter.AutoMigrate(&models.Setting{}); err != nil {
		return nil, err
	}

	// v0.5.5 Stage B — Room domain migration.
	// 在其余表 AutoMigrate 之前执行，确保后续 AutoMigrate 用新 schema 建 classrooms。
	// 幂等：通过 settings 表标记完成状态，重复调用无副作用。
	if err := MigrateRoomDomainV055(adapter); err != nil {
		log.Printf("[migrate] room domain migration failed: %v", err)
		return nil, err
	}

	// Auto-migrate all models
	err = adapter.AutoMigrate(
		&models.Course{},
		&models.Teacher{},
		&models.Building{},
		&models.Classroom{},
		&models.ClassGroup{},
		&models.ScheduleEntry{},
		&models.Semester{},
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

// MigrateRoomDomainV055 一次性迁移：清理旧 classrooms schema，为新的
// Building/Classroom 领域模型腾空间。
//
// 迁移流程（backup → drop old → create new → seed → verify）：
//  1. 若已标记完成 → 跳过（幂等）
//  2. 若旧 classrooms 表存在且含旧列（building 字符串 或 type 列） →
//     rename classrooms → classrooms_v054_backup（保留数据以便调试查证）
//  3. 若不含旧列 → 表不存在或已是新 schema，直接跳过
//  4. 迁移标记写入 settings 表（key=room_domain_v055_migrated）
//
// 表实际创建（buildings + 新 classrooms）由后续 AutoMigrate 完成。
// Seed 阶段将灌入 Building + Classroom 记录（含 BuildingID FK）。
// Verify 由 VerifyRoomDomainMigration 单独执行。
func MigrateRoomDomainV055(db DB) error {
	const markerKey = "room_domain_v055_migrated"

	// (1) 幂等：已迁移则跳过
	if isMigrationMarked(db, markerKey) {
		return nil
	}

	adapter, ok := db.(*GormAdapter)
	if !ok {
		return errors.New("MigrateRoomDomainV055 requires *GormAdapter (raw DDL)")
	}
	raw := adapter.db

	migrator := raw.Migrator()

	// (2) 表不存在 → 首次启动，无需 rename
	if !migrator.HasTable("classrooms") {
		return markMigration(db, markerKey)
	}

	// (3) 检测旧列：building (string) 或 type (string) 存在 → 旧 schema
	hasOldBuilding := migrator.HasColumn("classrooms", "building")
	hasOldType := migrator.HasColumn("classrooms", "type")

	if !hasOldBuilding && !hasOldType {
		// 已是新 schema（可能是首次 create 后重启）→ 直接标记
		return markMigration(db, markerKey)
	}

	// (4) 备份：rename classrooms → classrooms_v054_backup
	// 若 backup 表已存在（之前失败重试），先 drop
	if migrator.HasTable("classrooms_v054_backup") {
		if err := migrator.DropTable("classrooms_v054_backup"); err != nil {
			return fmt.Errorf("drop stale backup: %w", err)
		}
	}
	if err := migrator.RenameTable("classrooms", "classrooms_v054_backup"); err != nil {
		return fmt.Errorf("backup classrooms: %w", err)
	}
	log.Printf("[migrate] classrooms → classrooms_v054_backup (rows preserved)")

	// (5) 标记完成 —— AutoMigrate 将随后用新 schema 创建 classrooms + buildings
	return markMigration(db, markerKey)
}

// VerifyRoomDomainMigration 校验迁移后数据合法性：
//   - 所有 Classroom.BuildingID 非 0
//   - 所有 Classroom.RoomType ∈ 英文枚举
//   - classrooms 表不含旧列 (building, type)
//
// 用于测试与运维核对；生产启动流程不强依赖，允许 warn-only。
func VerifyRoomDomainMigration(db DB) []string {
	var warnings []string

	adapter, ok := db.(*GormAdapter)
	if !ok {
		return []string{"VerifyRoomDomainMigration: not *GormAdapter"}
	}
	raw := adapter.db

	// (a) 旧列残留检查
	migrator := raw.Migrator()
	if migrator.HasColumn(&models.Classroom{}, "building") {
		warnings = append(warnings, "classrooms.building column still present")
	}
	if migrator.HasColumn(&models.Classroom{}, "type") {
		warnings = append(warnings, "classrooms.type column still present")
	}

	// (b) 所有 classrooms 必须有合法 BuildingID
	var orphanCount int64
	raw.Model(&models.Classroom{}).Where("building_id IS NULL OR building_id = 0").Count(&orphanCount)
	if orphanCount > 0 {
		warnings = append(warnings, fmt.Sprintf("%d classrooms have BuildingID = 0 (no Building FK)", orphanCount))
	}

	// (c) RoomType 值域检查
	validTypes := map[string]bool{
		models.RoomTypeNormal:     true,
		models.RoomTypeMultimedia: true,
		models.RoomTypeLab:        true,
		models.RoomTypeComputer:   true,
		models.RoomTypeGym:        true,
		models.RoomTypeLecture:    true,
	}
	var rooms []models.Classroom
	raw.Find(&rooms)
	for _, r := range rooms {
		if !validTypes[r.RoomType] {
			warnings = append(warnings, fmt.Sprintf("classroom %s has invalid RoomType=%q", r.Code, r.RoomType))
		}
	}

	return warnings
}

// isMigrationMarked 检查 settings 表中的 migration flag。
func isMigrationMarked(db DB, key string) bool {
	var s models.Setting
	err := db.Where("key = ?", key).First(&s).Error()
	return err == nil && s.Value == "true"
}

// markMigration 将 migration flag 写入 settings 表（幂等 upsert）。
func markMigration(db DB, key string) error {
	var s models.Setting
	err := db.Where("key = ?", key).First(&s).Error()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&models.Setting{Key: key, Value: "true"}).Error()
	}
	if err != nil {
		return err
	}
	s.Value = "true"
	return db.Save(&s).Error()
}


