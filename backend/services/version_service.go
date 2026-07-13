package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"scheduling-system/backend/database"
	"scheduling-system/backend/models"
)

// DefaultMaxVersions is the maximum number of versions retained per semester.
// When a new version is created and the count exceeds this limit, the oldest
// version(s) are deleted (FIFO sliding window).
const DefaultMaxVersions = 50

// VersionService manages ScheduleVersion and ScheduleVersionEntry records.
type VersionService struct {
	db database.DB
}

func NewVersionService(db database.DB) *VersionService {
	return &VersionService{db: db}
}

// CreateVersion stores a new historical version from the given schedule entries.
// It automatically enforces the per-semester retention limit (DefaultMaxVersions).
//
// semesterID — Semester.ID (v0.5.5 起改为直接传 FK，取代旧的 name 查找).
// name       — user-visible version label (e.g. "自动方案 #3").
// source     — one of the VersionSource* constants.
// score      — final ScoreSchedule total at the time of capture.
// solver     — solver identifier ("simulated_annealing", "ortools", etc.).
// entries    — current schedule entries to copy into the version.
func (s *VersionService) CreateVersion(semesterID uint, name, source string, score float64, solver string, entries []models.ScheduleEntry) (*models.ScheduleVersion, error) {
	// Build version entry rows from current schedule entries
	versionEntries := make([]models.ScheduleVersionEntry, 0, len(entries))
	for _, e := range entries {
		originalID := e.ID
		ve := models.ScheduleVersionEntry{
			OriginalEntry:  &originalID,
			TeachingTaskID: e.TeachingTaskID,
			CourseID:       e.CourseID,
			TeacherID:      e.TeacherID,
			ClassroomID:    e.ClassroomID,
			DayOfWeek:      int(e.DayOfWeek),
			StartPeriod:    int(e.StartPeriod),
			Span:           e.Span,
			Weeks:          e.Weeks,
		}
		versionEntries = append(versionEntries, ve)
	}

	version := &models.ScheduleVersion{
		SemesterID: semesterID,
		Name:       name,
		Source:     source,
		Score:      score,
		EntryCount: len(versionEntries),
		Solver:     solver,
		Entries:    versionEntries,
	}

	if err := s.db.Create(version).Error(); err != nil {
		return nil, fmt.Errorf("version: 创建版本失败: %w", err)
	}

	// Enforce retention limit
	s.enforceRetention(s.db, semesterID)

	return version, nil
}

// CreateManualVersion saves the current schedule as a new version with
// source = ManualAdjust. It loads the live schedule entries from the
// database, computes the current ScoreSchedule score, and persists a
// new ScheduleVersion (with entries) in a single transaction.
func (s *VersionService) CreateManualVersion(semesterID uint, name string) (*models.ScheduleVersion, error) {
	// Guard against empty name — generate a default so callers that forget
	// to provide one (e.g. old RPC callers, test code) still produce a valid version.
	if strings.TrimSpace(name) == "" {
		name = fmt.Sprintf("手动方案 %s", time.Now().Format("2006-01-02 15:04"))
	}

	// Deduplicate: append counter if the same name already exists for this semester.
	// This prevents confusion when users save multiple versions with the default name.
	var dupCount int64
	s.db.Model(&models.ScheduleVersion{}).Where("semester_id = ? AND name = ?", semesterID, name).Count(&dupCount)
	if dupCount > 0 {
		name = fmt.Sprintf("%s (%d)", name, dupCount+1)
	}

	// Load current schedule entries
	var entries []models.ScheduleEntry
	if err := s.db.Where("semester_id = ?", semesterID).
		Preload("Course").Preload("Teacher").Preload("Classroom").
		Preload("TeachingTask.Classes.ClassGroup").
		Find(&entries).Error(); err != nil {
		return nil, fmt.Errorf("version: 加载当前课表失败: %w", err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("version: 当前学期无课表数据，请先运行自动排课")
	}

	// Build scoring context and reference data
	scoringCtx, teachers, classrooms, err := s.buildScoringContext(semesterID)
	if err != nil {
		return nil, err
	}

	// Compute score using the unified scoring pipeline
	breakdown := NewScoringService().ScoreSchedule(entries, teachers, classrooms, scoringCtx)

	// Build version entry rows
	versionEntries := make([]models.ScheduleVersionEntry, 0, len(entries))
	for _, e := range entries {
		originalID := e.ID
		ve := models.ScheduleVersionEntry{
			OriginalEntry:  &originalID,
			TeachingTaskID: e.TeachingTaskID,
			CourseID:       e.CourseID,
			TeacherID:      e.TeacherID,
			ClassroomID:    e.ClassroomID,
			DayOfWeek:      int(e.DayOfWeek),
			StartPeriod:    int(e.StartPeriod),
			Span:           e.Span,
			Weeks:          e.Weeks,
		}
		versionEntries = append(versionEntries, ve)
	}

	// Persist version + enforce retention in a single transaction
	var version *models.ScheduleVersion
	err = s.db.Transaction(func(tx database.DB) error {
		v := &models.ScheduleVersion{
			SemesterID: semesterID,
			Name:       name,
			Source:     models.VersionSourceManualAdjust,
			Score:      breakdown.Total,
			EntryCount: len(versionEntries),
			Entries:    versionEntries,
		}
		if err := tx.Create(v).Error(); err != nil {
			return fmt.Errorf("version: 创建版本失败: %w", err)
		}
		version = v

		// Enforce retention within the same transaction
		s.enforceRetention(tx, semesterID)
		return nil
	})

	return version, err
}

// ListVersions returns all versions for the given semester, ordered by
// created_at descending (newest first).
func (s *VersionService) ListVersions(semesterID uint) ([]models.ScheduleVersion, error) {
	var versions []models.ScheduleVersion
	if err := s.db.Where("semester_id = ?", semesterID).
		Order("created_at DESC").Find(&versions).Error(); err != nil {
		return nil, fmt.Errorf("version: 查询版本列表失败: %w", err)
	}
	return versions, nil
}

// GetVersion retrieves a single version with its entries preloaded.
func (s *VersionService) GetVersion(id uint) (*models.ScheduleVersion, error) {
	var version models.ScheduleVersion
	if err := s.db.Preload("Entries").First(&version, id).Error(); err != nil {
		return nil, fmt.Errorf("version: 未找到 ID=%d: %w", id, err)
	}
	return &version, nil
}

// DeleteVersion removes a version and its entries by ID.
func (s *VersionService) DeleteVersion(id uint) error {
	// Delete entries first (FK constraint)
	if err := s.db.Where("version_id = ?", id).Delete(&models.ScheduleVersionEntry{}).Error(); err != nil {
		return fmt.Errorf("version: 删除版本条目失败: %w", err)
	}
	if err := s.db.Delete(&models.ScheduleVersion{}, id).Error(); err != nil {
		return fmt.Errorf("version: 删除版本失败: %w", err)
	}
	return nil
}

// ClearSemesterVersions removes every ScheduleVersion (and its entries)
// belonging to the given semester in a single transaction. Only affects
// the specified semester; other semesters, courses, teachers, classrooms,
// snapshots, and live schedule entries are untouched.
//
// Returns the number of versions deleted. If the semester has no versions,
// returns 0 with a nil error.
func (s *VersionService) ClearSemesterVersions(semesterID uint) (int64, error) {
	var deleted int64
	err := s.db.Transaction(func(tx database.DB) error {
		// Collect version IDs for this semester so we can delete entries
		// via IN (...) without relying on Raw SQL (not on our DB interface).
		var versions []models.ScheduleVersion
		if err := tx.Where("semester_id = ?", semesterID).
			Find(&versions).Error(); err != nil {
			return fmt.Errorf("version: 查询版本失败: %w", err)
		}

		if len(versions) == 0 {
			return nil
		}

		ids := make([]uint, 0, len(versions))
		for _, v := range versions {
			ids = append(ids, v.ID)
		}

		// Delete entries first (FK safety, matches DeleteVersion order).
		if err := tx.Where("version_id IN ?", ids).
			Delete(&models.ScheduleVersionEntry{}).Error(); err != nil {
			return fmt.Errorf("version: 删除版本条目失败: %w", err)
		}
		if err := tx.Where("semester_id = ?", semesterID).
			Delete(&models.ScheduleVersion{}).Error(); err != nil {
			return fmt.Errorf("version: 删除版本失败: %w", err)
		}

		deleted = int64(len(versions))
		return nil
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

// buildScoringContext assembles all data needed to evaluate a full semester's
// schedule. It loads teachers, classrooms, and teaching tasks, identifies
// sports courses, reads the constraint list from the latest snapshot, and
// returns a ready-to-use ScoringContext together with the teacher and
// classroom slices required by ScoreSchedule.
func (s *VersionService) buildScoringContext(semesterID uint) (ScoringContext, []models.Teacher, []models.Classroom, error) {
	var teachers []models.Teacher
	if err := s.db.Find(&teachers).Error(); err != nil {
		return ScoringContext{}, nil, nil, fmt.Errorf("version: 加载教师数据失败: %w", err)
	}

	var classrooms []models.Classroom
	if err := s.db.Find(&classrooms).Error(); err != nil {
		return ScoringContext{}, nil, nil, fmt.Errorf("version: 加载教室数据失败: %w", err)
	}

	// Load teaching tasks for sports course detection and student_fatigue
	var teachingTasks []models.TeachingTask
	if semesterID > 0 {
		if err := s.db.Where("semester_id = ?", semesterID).
			Preload("Course").Preload("Teacher").
			Preload("Classes.ClassGroup").
			Find(&teachingTasks).Error(); err != nil {
			return ScoringContext{}, nil, nil, fmt.Errorf("version: 加载教学任务失败: %w", err)
		}
	}

	// Build sports course IDs
	sportsCourseIDs := make(map[uint]bool)
	for _, tt := range teachingTasks {
		if tt.CourseID > 0 && strings.Contains(tt.Course.Name, "体育") {
			sportsCourseIDs[tt.CourseID] = true
		}
	}

	// Read constraints from latest snapshot; fall back to all constraints
	constraints := FullDefaultConstraints()
	var snap models.ScheduleSnapshot
	if err := s.db.Where("semester_id = ?", semesterID).
		Order("created_at DESC").First(&snap).Error(); err == nil && snap.EnabledConstraints != "" {
		var parsed []string
		if json.Unmarshal([]byte(snap.EnabledConstraints), &parsed) == nil && len(parsed) > 0 {
			constraints = parsed
		}
	}

	ctx := NewScoringContext(constraints, sportsCourseIDs, teachingTasks)
	return ctx, teachers, classrooms, nil
}

// enforceRetention ensures the per-semester version count does not exceed
// DefaultMaxVersions by deleting the oldest version(s) if necessary.
// Accepts a DB argument so it can be called inside a transaction.
func (s *VersionService) enforceRetention(tx database.DB, semesterID uint) {
	var count int64
	if err := tx.Model(&models.ScheduleVersion{}).
		Where("semester_id = ?", semesterID).
		Count(&count).Error(); err != nil {
		return
	}

	if count <= DefaultMaxVersions {
		return
	}

	excess := int(count) - DefaultMaxVersions

	// Load all versions for this semester ordered oldest-first
	var allVersions []models.ScheduleVersion
	if err := tx.Where("semester_id = ?", semesterID).
		Order("created_at ASC").Find(&allVersions).Error(); err != nil {
		return
	}

	// Delete the N oldest
	for i := 0; i < excess && i < len(allVersions); i++ {
		v := allVersions[i]
		tx.Where("version_id = ?", v.ID).Delete(&models.ScheduleVersionEntry{})
		tx.Delete(&v)
	}
}
