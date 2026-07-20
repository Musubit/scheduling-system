package services

import (
	"scheduling-system/backend/database"
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

// GetEnrichedScheduleEntries returns flattened schedule entries for the given semester.
// TODO(v0.6.1): Adapt for TimeAssignment+ScheduleEntry split model.
// The old implementation reads legacy ScheduleEntry with Course/Teacher/Classroom preloads
// that no longer exist after the TA+SE split. This read path will be properly implemented
// in Task 8 (frontend + bindings) when the new data model is fully populated.
func (s *ScheduleQueryService) GetEnrichedScheduleEntries(semesterID uint, scheduleVersionID uint) ([]EnrichedScheduleEntry, error) {
	_ = semesterID
	_ = scheduleVersionID
	return nil, nil
}
