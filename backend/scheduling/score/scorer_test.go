package score_test

import (
	"testing"

	"scheduling-system/backend/scheduling/score"
	schedtypes "scheduling-system/backend/scheduling/types"
)

func TestScorer_TIME_ONLY_ThreeBuckets(t *testing.T) {
	s := score.NewScorer()
	assignments := []schedtypes.TimeAssignmentDraft{
		{TeachingTaskID: 1, DayOfWeek: 0, StartPeriod: 0, Span: 2},
		{TeachingTaskID: 2, DayOfWeek: 2, StartPeriod: 2, Span: 2},
	}
	tasks := []schedtypes.TeachingTaskView{
		{ID: 1, CourseID: 10, TeacherID: 100, CourseHours: 32, StartWeek: 1, EndWeek: 16, ClassGroupIDs: []uint{1}},
		{ID: 2, CourseID: 20, TeacherID: 100, CourseHours: 32, StartWeek: 1, EndWeek: 16, ClassGroupIDs: []uint{1}},
	}
	teachers := []schedtypes.TeacherView{
		{ID: 100, MaxDaysPerWeek: 5},
	}
	result := s.Score(assignments, nil, teachers, nil, tasks, []string{"time", "teacher", "student"})

	if result.Time == nil {
		t.Error("time bucket should not be nil")
	}
	if result.Teacher == nil {
		t.Error("teacher bucket should not be nil")
	}
	if result.Student == nil {
		t.Error("student bucket should not be nil")
	}
	if result.Resource != nil {
		t.Error("resource bucket should be nil in TIME_ONLY mode")
	}
	if result.PlacedSessions != 2 {
		t.Errorf("PlacedSessions = %d, want 2", result.PlacedSessions)
	}
}

func TestScorer_FULL_FourBuckets(t *testing.T) {
	s := score.NewScorer()
	assignments := []schedtypes.TimeAssignmentDraft{
		{TeachingTaskID: 1, DayOfWeek: 0, StartPeriod: 0, Span: 2},
	}
	allocations := []schedtypes.RoomAllocationDraft{
		{LocalRef: 0, ClassroomID: 10},
	}
	tasks := []schedtypes.TeachingTaskView{
		{ID: 1, CourseID: 10, TeacherID: 100, TotalStudents: 30, CourseHours: 32, StartWeek: 1, EndWeek: 16, ClassGroupIDs: []uint{1}},
	}
	teachers := []schedtypes.TeacherView{
		{ID: 100, MaxDaysPerWeek: 5, PreferLowFloor: true},
	}
	classrooms := []schedtypes.ClassroomView{
		{ID: 10, Capacity: 60, Floor: 2, Type: "STANDARD"},
	}
	result := s.Score(assignments, allocations, teachers, classrooms, tasks,
		[]string{"time", "teacher", "student", "resource"})

	if result.Resource == nil {
		t.Error("resource bucket should not be nil in FULL mode")
	}
	if result.ExpectedSessions < 1 {
		t.Error("ExpectedSessions should be >= 1")
	}
}

func TestScorer_EmptyAssignments(t *testing.T) {
	s := score.NewScorer()
	result := s.Score(nil, nil, nil, nil, nil, []string{"time", "teacher", "student"})

	if result.PlacedSessions != 0 {
		t.Errorf("PlacedSessions = %d, want 0", result.PlacedSessions)
	}
	if result.Time == nil {
		t.Error("time bucket should be populated even for empty")
	}
}
