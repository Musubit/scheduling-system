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

func TestScorer_BucketValuesWithinMax(t *testing.T) {
	// 对抗性审查修复验证：每个 bucket.Value 必须 ≤ bucket.Max
	s := score.NewScorer()
	assignments := []schedtypes.TimeAssignmentDraft{
		{TeachingTaskID: 1, DayOfWeek: 0, StartPeriod: 0, Span: 2},
		{TeachingTaskID: 2, DayOfWeek: 2, StartPeriod: 2, Span: 2},
	}
	tasks := []schedtypes.TeachingTaskView{
		{ID: 1, CourseID: 10, TeacherID: 100, CourseHours: 32, StartWeek: 1, EndWeek: 16, ClassGroupIDs: []uint{1}},
		{ID: 2, CourseID: 20, TeacherID: 200, CourseHours: 32, StartWeek: 1, EndWeek: 16, ClassGroupIDs: []uint{1}},
	}
	teachers := []schedtypes.TeacherView{
		{ID: 100, MaxDaysPerWeek: 5},
		{ID: 200, MaxDaysPerWeek: 5},
	}

	result := s.Score(assignments, nil, teachers, nil, tasks, []string{"time", "teacher", "student"})

	buckets := []struct {
		name  string
		b     *schedtypes.ScoreBucket
	}{
		{"time", result.Time},
		{"teacher", result.Teacher},
		{"student", result.Student},
	}
	for _, b := range buckets {
		if b.b == nil {
			t.Errorf("%s bucket is nil", b.name)
			continue
		}
		if b.b.Value > b.b.Max+0.01 {
			t.Errorf("%s bucket Value=%.2f > Max=%.2f — BUG: values exceed bucket max",
				b.name, b.b.Value, b.b.Max)
		}
		if b.b.Value < 0 {
			t.Errorf("%s bucket Value=%.2f < 0", b.name, b.b.Value)
		}
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
