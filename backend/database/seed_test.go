//go:build !production

package database

import (
	"path/filepath"
	"testing"

	"scheduling-system/backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newTestDB spins up an ephemeral on-disk SQLite instance (glebarez pure-Go
// driver, matching production) and runs the full InitDB-equivalent pipeline:
// settings → room-domain migration → AutoMigrate → seed. It returns an
// adapter ready for assertions.
//
// We use a temp file rather than :memory: because MigrateRoomDomainV055
// uses migrator.HasTable / RenameTable across separate GORM statements —
// pure in-memory mode can drop the connection between calls under some driver
// configurations. A file inside t.TempDir() gives us the same production path
// with automatic cleanup.
func newTestDB(t *testing.T) *GormAdapter {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "seed_test.db")
	gormDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	adapter := &GormAdapter{db: gormDB}
	// SQLite on Windows keeps the file locked until the underlying *sql.DB is
	// closed; without this Close, t.TempDir() cleanup logs a spurious
	// "process cannot access the file" during teardown and can even fail the
	// test. Registering it as t.Cleanup runs strictly before TempDir removal.
	t.Cleanup(func() {
		if sqlDB, err := gormDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	if err := adapter.AutoMigrate(&models.Setting{}); err != nil {
		t.Fatalf("automigrate settings: %v", err)
	}
	if err := MigrateRoomDomainV055(adapter); err != nil {
		t.Fatalf("MigrateRoomDomainV055: %v", err)
	}
	if err := adapter.AutoMigrate(
		&models.Course{},
		&models.Teacher{},
		&models.Building{},
		&models.Classroom{},
		&models.ClassGroup{},
		&models.ScheduleEntry{},
		&models.Semester{},
		&models.VersionDetail{},
		&models.TeachingTask{},
		&models.TeachingTaskClass{},
		&models.Department{},
		&models.ScheduleVersion{},
		&models.ScheduleVersionEntry{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if err := MigrateCourseCategoryV055(adapter); err != nil {
		t.Fatalf("MigrateCourseCategoryV055: %v", err)
	}
	return adapter
}

// TestB5_BuildingSeed_AllCategoriesPresent verifies the B5-Final seed writes
// all three Building.Category values (teaching / lab / sports) — the
// representative-diversity contract that downstream Room Domain UI relies on.
func TestB5_BuildingSeed_AllCategoriesPresent(t *testing.T) {
	adapter := newTestDB(t)
	SeedData(adapter)

	var buildings []models.Building
	adapter.db.Find(&buildings)
	if len(buildings) == 0 {
		t.Fatalf("no buildings seeded")
	}

	seenCategories := map[string]int{}
	for _, b := range buildings {
		seenCategories[b.Category]++
	}
	for _, want := range []string{
		models.BuildingCategoryTeaching,
		models.BuildingCategoryLab,
		models.BuildingCategorySports,
	} {
		if seenCategories[want] == 0 {
			t.Errorf("no Building with Category=%q seeded; got %+v", want, seenCategories)
		}
	}
}

// TestB5_ClassroomsAllHaveValidBuildingFK enforces the B5-Final invariant that
// every classroom row carries a non-zero BuildingID resolving to an actual
// Building. A regression here would mean a Classroom orphaned from its
// physical building — the exact defect B5-Final was meant to close.
func TestB5_ClassroomsAllHaveValidBuildingFK(t *testing.T) {
	adapter := newTestDB(t)
	SeedData(adapter)

	var rooms []models.Classroom
	adapter.db.Find(&rooms)
	if len(rooms) == 0 {
		t.Fatalf("no classrooms seeded")
	}

	buildingIDs := map[uint]bool{}
	var buildings []models.Building
	adapter.db.Find(&buildings)
	for _, b := range buildings {
		buildingIDs[b.ID] = true
	}

	for _, r := range rooms {
		if r.BuildingID == 0 {
			t.Errorf("classroom %s has BuildingID=0 (orphan)", r.Code)
		}
		if !buildingIDs[r.BuildingID] {
			t.Errorf("classroom %s has BuildingID=%d not resolving to any Building row", r.Code, r.BuildingID)
		}
	}
}

// TestB5_SportsResources_NoLegacyGYM verifies the migration off the legacy
// virtual "GYM" building + "GYM_MAIN" classroom. Presence of either after seed
// means the cleanup path in seedBuildings failed or a stale seed spec crept
// back in.
func TestB5_SportsResources_NoLegacyGYM(t *testing.T) {
	adapter := newTestDB(t)
	SeedData(adapter)

	var b models.Building
	if err := adapter.db.Where("code = ?", "GYM").First(&b).Error; err == nil {
		t.Errorf("legacy virtual building code=GYM still present after seed")
	}

	var c models.Classroom
	if err := adapter.db.Where("code = ?", "GYM_MAIN").First(&c).Error; err == nil {
		t.Errorf("legacy classroom code=GYM_MAIN still present after seed")
	}

	// Positive: the two real fields must exist and carry RoomType=GYM.
	for _, code := range []string{"MIDDLE_FIELD", "WEST_FIELD"} {
		var room models.Classroom
		if err := adapter.db.Where("code = ?", code).First(&room).Error; err != nil {
			t.Errorf("expected sports classroom %q missing: %v", code, err)
			continue
		}
		if room.RoomType != models.RoomTypeGym {
			t.Errorf("sports classroom %q has RoomType=%q, want %q", code, room.RoomType, models.RoomTypeGym)
		}
	}
}

// TestB5_LegacyGYMCleanup_IdempotentOnPrePopulated proves the cleanup branch
// inside seedBuildings actually runs, not just that a clean DB happens to lack
// GYM. We insert the legacy rows first, run seed, and expect them gone.
func TestB5_LegacyGYMCleanup_IdempotentOnPrePopulated(t *testing.T) {
	adapter := newTestDB(t)

	// Simulate a v0.5.4 dev DB where the legacy virtual GYM building + its
	// classroom were previously seeded. Insert a fake Building with id first
	// so the FK is resolvable — the classroom row keeps its shape.
	legacy := models.Building{Code: "GYM", Name: "体育馆", Category: models.BuildingCategorySports, Status: models.BuildingStatusActive}
	if err := adapter.db.Create(&legacy).Error; err != nil {
		t.Fatalf("insert legacy GYM building: %v", err)
	}
	legacyRoom := models.Classroom{
		Code:       "GYM_MAIN",
		Name:       "体育馆主馆",
		BuildingID: legacy.ID,
		Floor:      1,
		Number:     "MAIN",
		Capacity:   500,
		RoomType:   models.RoomTypeGym,
		Status:     "available",
	}
	if err := adapter.db.Create(&legacyRoom).Error; err != nil {
		t.Fatalf("insert legacy GYM_MAIN: %v", err)
	}

	SeedData(adapter)

	var count int64
	adapter.db.Model(&models.Building{}).Where("code = ?", "GYM").Count(&count)
	if count != 0 {
		t.Errorf("legacy GYM building not cleaned by seed (count=%d)", count)
	}
	// GORM soft-delete leaves a tombstone row; scoped queries — which is what
	// every production code path uses — must not see GYM_MAIN. Checking the
	// scoped count is the meaningful assertion. We intentionally do NOT check
	// Unscoped: any downstream ScheduleEntry.ClassroomID pointing to the old
	// row would remain resolvable via FK, and hard-deleting would break that.
	adapter.db.Model(&models.Classroom{}).Where("code = ?", "GYM_MAIN").Count(&count)
	if count != 0 {
		t.Errorf("legacy GYM_MAIN classroom still visible to scoped queries (count=%d)", count)
	}
}

// TestB5_SeedIdempotency runs SeedData twice and asserts that the second call
// does not double the row counts of any base table. This is the explicit
// idempotency clause of the B5-Final acceptance criteria.
func TestB5_SeedIdempotency(t *testing.T) {
	adapter := newTestDB(t)

	SeedData(adapter)
	firstCounts := snapshotCounts(t, adapter)

	SeedData(adapter)
	secondCounts := snapshotCounts(t, adapter)

	for table, want := range firstCounts {
		if got := secondCounts[table]; got != want {
			t.Errorf("%s: row count changed on second seed: %d → %d (seed not idempotent)",
				table, want, got)
		}
	}
}

// snapshotCounts returns row counts of the tables mutated by SeedData /
// seedBaseData / seedTeachingTasks / seedDemoEntries, so idempotency is
// checked across the entire seed surface.
func snapshotCounts(t *testing.T, adapter *GormAdapter) map[string]int64 {
	t.Helper()
	targets := map[string]interface{}{
		"buildings":              &models.Building{},
		"classrooms":             &models.Classroom{},
		"departments":            &models.Department{},
		"teachers":               &models.Teacher{},
		"courses":                &models.Course{},
		"class_groups":           &models.ClassGroup{},
		"semesters":              &models.Semester{},
		"teaching_tasks":         &models.TeachingTask{},
		"teaching_task_classes":  &models.TeachingTaskClass{},
		"schedule_entries":       &models.ScheduleEntry{},
	}
	out := make(map[string]int64, len(targets))
	for name, m := range targets {
		var c int64
		adapter.db.Model(m).Count(&c)
		out[name] = c
	}
	return out
}

// TestB5_VerifyRoomDomainMigration_NoWarnings is the direct B5-Final acceptance
// check: after seed, VerifyRoomDomainMigration must return an empty slice.
// Any warning is a fatal regression — either an orphan Classroom, an invalid
// RoomType, or leftover legacy columns.
func TestB5_VerifyRoomDomainMigration_NoWarnings(t *testing.T) {
	adapter := newTestDB(t)
	SeedData(adapter)

	warnings := VerifyRoomDomainMigration(adapter)
	if len(warnings) != 0 {
		t.Errorf("VerifyRoomDomainMigration returned %d warning(s), want 0:", len(warnings))
		for _, w := range warnings {
			t.Errorf("  - %s", w)
		}
	}
}
