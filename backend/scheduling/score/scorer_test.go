package score

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"scheduling-system/backend/models"
	"scheduling-system/backend/services"
	"scheduling-system/backend/scheduling/types"
)

type ScoreFixture struct {
	Description    string                 `json:"description"`
	Mode           string                 `json:"mode"`
	Dimensions     []string               `json:"dimensions"`
	Assignments    []ScoreFixtureAssignment `json:"assignments"`
	Allocations    []ScoreFixtureAllocation `json:"allocations"`
	Teachers       []models.Teacher       `json:"teachers"`
	Classrooms     []models.Classroom     `json:"classrooms"`
	Tasks          []models.TeachingTask  `json:"tasks"`
	SportsCourseIDs []uint                `json:"sportsCourseIDs"`
	Expected       *services.ScoreBreakdown `json:"expected"`
}

type ScoreFixtureAssignment struct {
	TeachingTaskID uint   `json:"TeachingTaskID"`
	DayOfWeek      int    `json:"DayOfWeek"`
	StartPeriod    int    `json:"StartPeriod"`
	Span           int    `json:"Span"`
	CourseID       uint   `json:"CourseID,omitempty"`
	TeacherID      uint   `json:"TeacherID,omitempty"`
}

type ScoreFixtureAllocation struct {
	LocalRef     int `json:"LocalRef"`
	ClassroomID  uint `json:"ClassroomID"`
}

func TestScorerGolden(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil {
		t.Fatalf("glob testdata: %v", err)
	}

	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			var fixture ScoreFixture
			if err := json.Unmarshal(data, &fixture); err != nil {
				t.Fatalf("unmarshal fixture: %v", err)
			}

			entries := buildEntriesFromFixture(fixture)

			mode := types.SchedulingModeFromString(fixture.Mode)
			sportsIDs := make(map[uint]bool)
			for _, id := range fixture.SportsCourseIDs {
				sportsIDs[id] = true
			}
			ctx := services.ScoringContext{
				Version:               3,
				EnabledConstraints:    services.FullDefaultConstraints(),
				Mode:                  mode,
				SportsCourseIDs:       sportsIDs,
				TeachingTasks:         fixture.Tasks,
				ExpectedTotalSessions: len(fixture.Assignments),
			}

			scorer := NewScorer()
			result := scorer.Score(entries, fixture.Teachers, fixture.Classrooms, ctx)

			if os.Getenv("REGEN_GOLDEN") == "1" {
				fixture.Expected = &result
				updated, err := json.MarshalIndent(fixture, "", "  ")
				if err != nil {
					t.Fatalf("marshal updated fixture: %v", err)
				}
				if err := os.WriteFile(f, updated, 0644); err != nil {
					t.Fatalf("write fixture: %v", err)
				}
				return
			}

			if fixture.Expected == nil {
				t.Fatal("expected field is nil; run with REGEN_GOLDEN=1 to generate")
			}

			if result.Total != fixture.Expected.Total {
				t.Errorf("Total: got %v, want %v", result.Total, fixture.Expected.Total)
			}
			if result.FinalTotal != fixture.Expected.FinalTotal {
				t.Errorf("FinalTotal: got %v, want %v", result.FinalTotal, fixture.Expected.FinalTotal)
			}
			if result.PlacedSessions != fixture.Expected.PlacedSessions {
				t.Errorf("PlacedSessions: got %v, want %v", result.PlacedSessions, fixture.Expected.PlacedSessions)
			}
			if result.ExpectedSessions != fixture.Expected.ExpectedSessions {
				t.Errorf("ExpectedSessions: got %v, want %v", result.ExpectedSessions, fixture.Expected.ExpectedSessions)
			}
			if result.Completeness != fixture.Expected.Completeness {
				t.Errorf("Completeness: got %v, want %v", result.Completeness, fixture.Expected.Completeness)
			}
			if result.PerCategoryMax != fixture.Expected.PerCategoryMax {
				t.Errorf("PerCategoryMax: got %v, want %v", result.PerCategoryMax, fixture.Expected.PerCategoryMax)
			}
			if result.EnabledCategoryCount != fixture.Expected.EnabledCategoryCount {
				t.Errorf("EnabledCategoryCount: got %v, want %v", result.EnabledCategoryCount, fixture.Expected.EnabledCategoryCount)
			}

			if fixture.Expected.Buckets != nil && result.Buckets != nil {
				if (fixture.Expected.Buckets.Time == nil) != (result.Buckets.Time == nil) {
					t.Error("Time bucket nil mismatch")
				}
				if (fixture.Expected.Buckets.Teacher == nil) != (result.Buckets.Teacher == nil) {
					t.Error("Teacher bucket nil mismatch")
				}
				if (fixture.Expected.Buckets.Student == nil) != (result.Buckets.Student == nil) {
					t.Error("Student bucket nil mismatch")
				}
				if (fixture.Expected.Buckets.Resource == nil) != (result.Buckets.Resource == nil) {
					t.Error("Resource bucket nil mismatch")
				}
			}
		})
	}
}

func buildEntriesFromFixture(fixture ScoreFixture) []models.ScheduleEntry {
	entries := make([]models.ScheduleEntry, 0, len(fixture.Assignments))

	for i, assign := range fixture.Assignments {
		var classroomID uint
		if i < len(fixture.Allocations) {
			classroomID = fixture.Allocations[i].ClassroomID
		}

		var task *models.TeachingTask
		for _, t := range fixture.Tasks {
			if t.ID == assign.TeachingTaskID {
				task = &t
				break
			}
		}

		courseID := uint(100)
		if task != nil {
			courseID = task.CourseID
		}

		entries = append(entries, models.ScheduleEntry{
			CourseID:        courseID,
			TeacherID:       assign.TeacherID,
			ClassroomID:     classroomID,
			TeachingTaskID:  &assign.TeachingTaskID,
			DayOfWeek:       models.DayOfWeek(assign.DayOfWeek),
			StartPeriod:     models.Period(assign.StartPeriod),
			Span:            assign.Span,
		})
	}

	return entries
}

