package database

import (
	"scheduling-system/backend/models"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSoftDeleteUniqueIndexConflict(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Warn),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	gormDB.Exec("PRAGMA foreign_keys = OFF")

	if err := gormDB.AutoMigrate(
		&models.Classroom{},
		&models.ScheduleEntry{},
		&models.Course{},
		&models.Teacher{},
		&models.Semester{},
	); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	course := models.Course{Code: "TEST001", Name: "Test Course", Hours: 32}
	gormDB.Create(&course)
	teacher := models.Teacher{Code: "T001", Name: "Test Teacher"}
	gormDB.Create(&teacher)
	classroom := models.Classroom{Code: "R001", Name: "Room 1", Capacity: 50}
	gormDB.Create(&classroom)
	semester := models.Semester{AcademicYear: "2024-2025", Term: "FIRST", Status: "planned"}
	gormDB.Create(&semester)

	makeEntry := func() models.ScheduleEntry {
		return models.ScheduleEntry{
			CourseID:    course.ID,
			TeacherID:   teacher.ID,
			ClassroomID: classroom.ID,
			SemesterID:  semester.ID,
			DayOfWeek:   0,
			StartPeriod: 0,
			Span:        2,
			Weeks:       "1-16",
		}
	}

	t.Run("soft_delete_causes_unique_conflict", func(t *testing.T) {
		entry := makeEntry()
		if err := gormDB.Create(&entry).Error; err != nil {
			t.Fatalf("first insert failed: %v", err)
		}
		if err := gormDB.Where("id = ?", entry.ID).Delete(&models.ScheduleEntry{}).Error; err != nil {
			t.Fatalf("soft delete failed: %v", err)
		}
		entry2 := makeEntry()
		err := gormDB.Create(&entry2).Error
		if err == nil {
			t.Fatal("expected UNIQUE constraint error after soft delete, got nil")
		}
		t.Logf("Confirmed: soft delete + unique index conflict: %v", err)
	})

	t.Run("hard_delete_prevents_unique_conflict", func(t *testing.T) {
		// Clean up from previous sub-test
		gormDB.Unscoped().Where("1=1").Delete(&models.ScheduleEntry{})

		entry := makeEntry()
		if err := gormDB.Create(&entry).Error; err != nil {
			t.Fatalf("first insert failed: %v", err)
		}
		// Hard delete (Unscoped)
		if err := gormDB.Unscoped().Where("id = ?", entry.ID).Delete(&models.ScheduleEntry{}).Error; err != nil {
			t.Fatalf("hard delete failed: %v", err)
		}
		// Re-insert with same unique key
		entry2 := makeEntry()
		if err := gormDB.Create(&entry2).Error; err != nil {
			t.Fatalf("re-insert after hard delete should succeed, got: %v", err)
		}
		t.Log("Confirmed: hard delete + unique index = no conflict")
	})

	t.Run("transaction_rollback_restores_hard_deleted", func(t *testing.T) {
		gormDB.Unscoped().Where("1=1").Delete(&models.ScheduleEntry{})

		entry := makeEntry()
		if err := gormDB.Create(&entry).Error; err != nil {
			t.Fatalf("initial insert failed: %v", err)
		}

		err := gormDB.Transaction(func(tx *gorm.DB) error {
			// Hard delete inside transaction
			if err := tx.Unscoped().Where("id = ?", entry.ID).Delete(&models.ScheduleEntry{}).Error; err != nil {
				return err
			}
			// Verify deleted inside tx
			var count int64
			tx.Unscoped().Model(&models.ScheduleEntry{}).Where("id = ?", entry.ID).Count(&count)
			if count != 0 {
				t.Fatalf("expected 0 entries after hard delete in tx, got %d", count)
			}
			// Rollback by returning error
			return errSimulatedRollback
		})
		if err == nil {
			t.Fatal("expected transaction error, got nil")
		}

		// After rollback, entry should still exist
		var count int64
		gormDB.Model(&models.ScheduleEntry{}).Where("id = ?", entry.ID).Count(&count)
		if count != 1 {
			t.Fatalf("after rollback, expected 1 entry, got %d — hard delete was not rolled back", count)
		}
		t.Log("Confirmed: SQLite transaction rollback restores hard-deleted rows")
	})
}

var errSimulatedRollback = &simulatedRollbackError{}

type simulatedRollbackError struct{}

func (e *simulatedRollbackError) Error() string { return "simulated rollback" }
