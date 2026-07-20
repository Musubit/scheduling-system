package services

import (
	"scheduling-system/backend/database"
	"scheduling-system/backend/models"
)

// EnrichedScheduleEntry is a flattened read-model for frontend consumption.
// It denormalises schedule_entries with their related entities (course, teacher,
// classroom, building, teaching task, class groups) into a single struct.
//
// Phase 0 (v0.6.0-pre): populated from the legacy schedule_entries table.
// ScheduleVersionID is always 0 — the legacy table has no version linkage.
// A future phase will wire this to schedule_version_entries instead.
type EnrichedScheduleEntry struct {
	ID                uint     `json:"id"`
	DayOfWeek         int      `json:"dayOfWeek"`
	StartPeriod       int      `json:"startPeriod"`
	Span              int      `json:"span"`
	Weeks             string   `json:"weeks"`
	TeacherID         uint     `json:"teacherId"`
	TeacherName       string   `json:"teacherName"`
	CourseID          uint     `json:"courseId"`
	CourseName        string   `json:"courseName"`
	CourseCode        string   `json:"courseCode,omitempty"`
	CourseCredit      float64  `json:"courseCredit,omitempty"`
	ClassGroupIDs     []uint   `json:"classGroupIds"`
	ClassGroupNames   []string `json:"classGroupNames"`
	ClassroomID       *uint    `json:"classroomId,omitempty"`
	ClassroomName     *string  `json:"classroomName,omitempty"`
	ClassroomFloor    *int     `json:"classroomFloor,omitempty"`
	ClassroomType     *string  `json:"classroomType,omitempty"`
	ClassroomCode     *string  `json:"classroomCode,omitempty"`
	BuildingName      *string  `json:"buildingName,omitempty"`
	TeachingTaskID    *uint    `json:"teachingTaskId,omitempty"`
	SemesterID        uint     `json:"semesterId"`
	ScheduleVersionID uint     `json:"scheduleVersionId"`
}

// ScheduleQueryService provides read-path queries for schedule data.
// It is a purely additive service for Phase 0 — no existing production
// code paths are modified.
type ScheduleQueryService struct {
	db database.DB
}

// NewScheduleQueryService creates a new ScheduleQueryService.
func NewScheduleQueryService(db database.DB) *ScheduleQueryService {
	return &ScheduleQueryService{db: db}
}

// GetEnrichedScheduleEntries returns flattened schedule entries for the given
// semester. For Phase 0, this reads from the legacy schedule_entries table with
// GORM preloads.
//
// Parameters:
//   - semesterID: filter entries by semester (required).
//   - scheduleVersionID: optional filter; pass 0 to ignore.
//     Phase 0 ignores this — the legacy table has no schedule_version_id column.
//
// Classroom fields are nil when IsVirtual is true (TIME_ONLY mode).
func (s *ScheduleQueryService) GetEnrichedScheduleEntries(semesterID uint, scheduleVersionID uint) ([]EnrichedScheduleEntry, error) {
	var entries []models.ScheduleEntry

	q := s.db.
		Preload("Course").
		Preload("Teacher").
		Preload("Classroom.Building").
		Preload("TeachingTask.Classes.ClassGroup").
		Where("semester_id = ?", semesterID)

	// Phase 0: schedule_version_id does not exist on the legacy schedule_entries
	// table. The parameter is accepted for forward compatibility.
	_ = scheduleVersionID

	if err := q.Find(&entries).Error(); err != nil {
		return nil, err
	}

	result := make([]EnrichedScheduleEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, s.mapToEnriched(e))
	}

	return result, nil
}

// mapToEnriched converts a model.ScheduleEntry (with preloaded associations)
// into an EnrichedScheduleEntry.
func (s *ScheduleQueryService) mapToEnriched(e models.ScheduleEntry) EnrichedScheduleEntry {
	entry := EnrichedScheduleEntry{
		ID:                e.ID,
		DayOfWeek:         int(e.DayOfWeek),
		StartPeriod:       int(e.StartPeriod),
		Span:              e.Span,
		Weeks:             e.Weeks,
		TeacherID:         e.TeacherID,
		TeacherName:       e.Teacher.Name,
		CourseID:          e.CourseID,
		CourseName:        e.Course.Name,
		CourseCode:        e.Course.Code,
		CourseCredit:      e.Course.Credit,
		ClassGroupIDs:     []uint{},
		ClassGroupNames:   []string{},
		TeachingTaskID:    e.TeachingTaskID,
		SemesterID:        e.SemesterID,
		ScheduleVersionID: 0, // Phase 0: legacy table has no version linkage
	}

	// Resolve class groups from TeachingTask.Classes.ClassGroup.
	if e.TeachingTask != nil && len(e.TeachingTask.Classes) > 0 {
		entry.ClassGroupIDs = make([]uint, 0, len(e.TeachingTask.Classes))
		entry.ClassGroupNames = make([]string, 0, len(e.TeachingTask.Classes))
		for _, tc := range e.TeachingTask.Classes {
			entry.ClassGroupIDs = append(entry.ClassGroupIDs, tc.ClassGroupID)
			entry.ClassGroupNames = append(entry.ClassGroupNames, tc.ClassGroup.Name)
		}
	}

	// Classroom fields are nil when IsVirtual (TIME_ONLY mode).
	if !e.IsVirtual {
		cid := e.ClassroomID
		cname := e.Classroom.Name
		cfloor := e.Classroom.Floor
		ctype := e.Classroom.RoomType
		ccode := e.Classroom.Code
		bname := e.Classroom.Building.Name

		entry.ClassroomID = &cid
		entry.ClassroomName = &cname
		entry.ClassroomFloor = &cfloor
		entry.ClassroomType = &ctype
		entry.ClassroomCode = &ccode
		entry.BuildingName = &bname
	}

	return entry
}
