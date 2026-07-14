package score

import (
	"testing"

	"gorm.io/gorm"
	"scheduling-system/backend/models"
	"scheduling-system/backend/services"
	"scheduling-system/backend/scheduling/types"
)

func TestScorerEquivalence(t *testing.T) {
	testCases := []struct {
		name         string
		mode         types.SchedulingMode
		entries      []models.ScheduleEntry
		teachers     []models.Teacher
		classrooms   []models.Classroom
		tasks        []models.TeachingTask
		sportsIDs    map[uint]bool
		expectedSess int
	}{
		{
			name: "empty schedule",
			mode: types.ModeFullScheduling,
			entries: []models.ScheduleEntry{},
			teachers: []models.Teacher{},
			classrooms: []models.Classroom{},
			tasks: []models.TeachingTask{},
			sportsIDs: map[uint]bool{},
			expectedSess: 0,
		},
		{
			name: "single entry",
			mode: types.ModeFullScheduling,
			entries: []models.ScheduleEntry{
				{CourseID: 100, TeacherID: 10, ClassroomID: 1000, DayOfWeek: 0, StartPeriod: 2, Span: 2},
			},
			teachers: []models.Teacher{
				{Model: gorm.Model{ID: 10}, Name: "A"},
			},
			classrooms: []models.Classroom{
				{Model: gorm.Model{ID: 1000}, Floor: 1, RoomType: "STANDARD", BuildingID: 1},
			},
			tasks: []models.TeachingTask{},
			sportsIDs: map[uint]bool{},
			expectedSess: 1,
		},
		{
			name: "time only mode",
			mode: types.ModeTimeOnlyScheduling,
			entries: []models.ScheduleEntry{
				{CourseID: 100, TeacherID: 10, ClassroomID: 0, DayOfWeek: 0, StartPeriod: 0, Span: 2},
				{CourseID: 100, TeacherID: 10, ClassroomID: 0, DayOfWeek: 1, StartPeriod: 2, Span: 2},
			},
			teachers: []models.Teacher{
				{Model: gorm.Model{ID: 10}, Name: "A"},
			},
			classrooms: []models.Classroom{},
			tasks: []models.TeachingTask{},
			sportsIDs: map[uint]bool{},
			expectedSess: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := services.ScoringContext{
				Version:               3,
				EnabledConstraints:    services.FullDefaultConstraints(),
				Mode:                  tc.mode,
				SportsCourseIDs:       tc.sportsIDs,
				TeachingTasks:         tc.tasks,
				ExpectedTotalSessions: tc.expectedSess,
			}

			newScorer := NewScorer()
			newResult := newScorer.Score(tc.entries, tc.teachers, tc.classrooms, ctx)

			oldScorer := services.NewScoringService()
			oldResult := oldScorer.ScoreSchedule(tc.entries, tc.teachers, tc.classrooms, ctx)

			if newResult.Total != oldResult.Total {
				t.Errorf("Total: new=%v, old=%v", newResult.Total, oldResult.Total)
			}
			if newResult.FinalTotal != oldResult.FinalTotal {
				t.Errorf("FinalTotal: new=%v, old=%v", newResult.FinalTotal, oldResult.FinalTotal)
			}
			if newResult.PlacedSessions != oldResult.PlacedSessions {
				t.Errorf("PlacedSessions: new=%v, old=%v", newResult.PlacedSessions, oldResult.PlacedSessions)
			}
			if newResult.ExpectedSessions != oldResult.ExpectedSessions {
				t.Errorf("ExpectedSessions: new=%v, old=%v", newResult.ExpectedSessions, oldResult.ExpectedSessions)
			}
			if newResult.Completeness != oldResult.Completeness {
				t.Errorf("Completeness: new=%v, old=%v", newResult.Completeness, oldResult.Completeness)
			}
			if newResult.PerCategoryMax != oldResult.PerCategoryMax {
				t.Errorf("PerCategoryMax: new=%v, old=%v", newResult.PerCategoryMax, oldResult.PerCategoryMax)
			}
			if newResult.EnabledCategoryCount != oldResult.EnabledCategoryCount {
				t.Errorf("EnabledCategoryCount: new=%v, old=%v", newResult.EnabledCategoryCount, oldResult.EnabledCategoryCount)
			}

			if (newResult.Buckets == nil) != (oldResult.Buckets == nil) {
				t.Error("Buckets nil mismatch")
			}
			if newResult.Buckets != nil && oldResult.Buckets != nil {
				if (newResult.Buckets.Time == nil) != (oldResult.Buckets.Time == nil) {
					t.Error("Time bucket nil mismatch")
				}
				if (newResult.Buckets.Teacher == nil) != (oldResult.Buckets.Teacher == nil) {
					t.Error("Teacher bucket nil mismatch")
				}
				if (newResult.Buckets.Student == nil) != (oldResult.Buckets.Student == nil) {
					t.Error("Student bucket nil mismatch")
				}
				if (newResult.Buckets.Resource == nil) != (oldResult.Buckets.Resource == nil) {
					t.Error("Resource bucket nil mismatch")
				}
			}

			if len(newResult.EnabledDimensions) != len(oldResult.EnabledDimensions) {
				t.Errorf("EnabledDimensions length: new=%v, old=%v", newResult.EnabledDimensions, oldResult.EnabledDimensions)
			}
		})
	}
}