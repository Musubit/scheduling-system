package database

import (
	"fmt"
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

// MigrateV060 执行 v0.6.0 核心数据模型迁移：
// 将旧的 ScheduleEntry（时间+教室耦合）替换为  TimeAssignment + ScheduleEntry 拆分模型。
//
// 迁移流程（backup → drop old → create new → mark）：
//  1. 备份旧 schedule_entries → _bak_schedule_entries_v057
//  2. DROP 旧 schedule_entries 表
//  3. AutoMigrate 创建新的 TimeAssignment + ScheduleEntry 表
//  4. 标记迁移完成（schema_migrations 表）
//
// 幂等：标记检查确保重复调用无副作用。
func MigrateV060(db DB) error {
	const markerKey = "v0.6.0_schedule_entry_refactor"
	if isMigrationMarked(db, markerKey) {
		return nil
	}

	return db.Transaction(func(tx DB) error {
		adapter, ok := tx.(*GormAdapter)
		if !ok {
			return fmt.Errorf("MigrateV060 requires *GormAdapter (raw DDL)")
		}

		// 1. Backup old schedule_entries
		if err := adapter.db.Exec(`
			CREATE TABLE IF NOT EXISTS _bak_schedule_entries_v057 AS
			SELECT * FROM schedule_entries
		`).Error; err != nil {
			return fmt.Errorf("backup schedule_entries failed: %w", err)
		}

		// 2. Drop old schedule_entries
		if err := adapter.db.Exec(`DROP TABLE IF EXISTS schedule_entries`).Error; err != nil {
			return fmt.Errorf("drop old schedule_entries failed: %w", err)
		}

		// 3. AutoMigrate creates new TimeAssignment + ScheduleEntry tables
		migrateErr := adapter.db.AutoMigrate(
			&models.TimeAssignment{},
			&models.ScheduleEntry{},
		)
		if migrateErr != nil {
			return fmt.Errorf("v0.6.0 automigrate failed: %w", migrateErr)
		}

		// 4. Mark migration complete
		return EnsureMigrationApplied(adapter, "v0.6.0")
	})
}
