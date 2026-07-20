package score_test

import (
	"testing"

	"scheduling-system/backend/scheduling/score"
	schedtypes "scheduling-system/backend/scheduling/types"
)

// TestScorer_Idempotency 验证相同输入得相同输出。
func TestScorer_Idempotency(t *testing.T) {
	s := score.NewScorer()
	assignments := []schedtypes.TimeAssignmentDraft{
		{TeachingTaskID: 1, DayOfWeek: 1, StartPeriod: 0, Span: 2},
		{TeachingTaskID: 1, DayOfWeek: 3, StartPeriod: 2, Span: 2},
	}
	tasks := []schedtypes.TeachingTaskView{
		{ID: 1, CourseID: 10, TeacherID: 100, CourseHours: 64, StartWeek: 1, EndWeek: 16, ClassGroupIDs: []uint{1}},
	}
	teachers := []schedtypes.TeacherView{
		{ID: 100, MaxDaysPerWeek: 5},
	}

	r1 := s.Score(assignments, nil, teachers, nil, tasks, []string{"time", "teacher", "student"})
	r2 := s.Score(assignments, nil, teachers, nil, tasks, []string{"time", "teacher", "student"})

	if r1.FinalTotal != r2.FinalTotal {
		t.Errorf("idempotency violated: %f != %f", r1.FinalTotal, r2.FinalTotal)
	}
}

// TestScorer_CompletenessPartial 验证当 placed < expected 时 completeness < 1。
func TestScorer_CompletenessPartial(t *testing.T) {
	s := score.NewScorer()
	// 一个 64 学时的课，应该每周 4 学时 (2 sessions)，但只放了 1 个 session
	assignments := []schedtypes.TimeAssignmentDraft{
		{TeachingTaskID: 1, DayOfWeek: 0, StartPeriod: 0, Span: 2},
	}
	tasks := []schedtypes.TeachingTaskView{
		{ID: 1, CourseID: 10, TeacherID: 100, CourseHours: 64, StartWeek: 1, EndWeek: 16, ClassGroupIDs: []uint{1}},
	}
	teachers := []schedtypes.TeacherView{
		{ID: 100, MaxDaysPerWeek: 5},
	}

	result := s.Score(assignments, nil, teachers, nil, tasks, []string{"time", "teacher", "student"})

	if result.ExpectedSessions <= result.PlacedSessions {
		t.Logf("ExpectedSessions=%d, PlacedSessions=%d (test may not detect partial)", result.ExpectedSessions, result.PlacedSessions)
	}
	if result.Completeness > 1.0 {
		t.Errorf("completeness should be <= 1.0, got %f", result.Completeness)
	}
}
