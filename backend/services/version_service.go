package services

import (
	"fmt"

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
// semester — semester name string, used to look up Semester.ID.
// name     — user-visible version label (e.g. "自动方案 #3").
// source   — one of the VersionSource* constants.
// score    — final ScoreSchedule total at the time of capture.
// solver   — solver identifier ("simulated_annealing", "ortools", etc.).
// entries  — current schedule entries to copy into the version.
func (s *VersionService) CreateVersion(semester, name, source string, score float64, solver string, entries []models.ScheduleEntry) (*models.ScheduleVersion, error) {
	// Resolve semester ID from name
	semesterID, err := s.resolveSemesterID(semester)
	if err != nil {
		return nil, fmt.Errorf("version: 未找到学期 '%s': %w", semester, err)
	}

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
	s.enforceRetention(semesterID)

	return version, nil
}

// ListVersions returns all versions for the given semester, ordered by
// created_at descending (newest first).
func (s *VersionService) ListVersions(semester string) ([]models.ScheduleVersion, error) {
	semesterID, err := s.resolveSemesterID(semester)
	if err != nil {
		return nil, fmt.Errorf("version: 找不到学期 '%s'", semester)
	}

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

// enforceRetention ensures the per-semester version count does not exceed
// DefaultMaxVersions by deleting the oldest version(s) if necessary.
func (s *VersionService) enforceRetention(semesterID uint) {
	var count int64
	if err := s.db.Model(&models.ScheduleVersion{}).
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
	if err := s.db.Where("semester_id = ?", semesterID).
		Order("created_at ASC").Find(&allVersions).Error(); err != nil {
		return
	}

	// Delete the N oldest
	for i := 0; i < excess && i < len(allVersions); i++ {
		v := allVersions[i]
		s.db.Where("version_id = ?", v.ID).Delete(&models.ScheduleVersionEntry{})
		s.db.Delete(&v)
	}
}

// resolveSemesterID returns the Semester.ID for a given semester name.
func (s *VersionService) resolveSemesterID(name string) (uint, error) {
	var sem models.Semester
	if err := s.db.Where("name = ?", name).First(&sem).Error(); err != nil {
		return 0, err
	}
	return sem.ID, nil
}
