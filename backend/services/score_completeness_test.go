package services

import (
	"math"
	"scheduling-system/backend/models"
	"testing"

	"gorm.io/gorm"
)

// v0.5.2 Goal 2 Case B: 50% completeness must yield ≤60.
// The β curve produces factor = 0.5 × (0.5 + 0.5×0.5) = 0.375, so a full-marks
// Total of 100 becomes FinalTotal = 37.5 — well under 60.
func TestFinalTotalCompleteness50PctBelow60(t *testing.T) {
	teachers := []models.Teacher{{Model: gorm.Model{ID: 1}, Name: "T1"}}
	classrooms := []models.Classroom{{Model: gorm.Model{ID: 1}, Name: "R1", Capacity: 100, Floor: 1}}
	// 4 entries, expected 8 → 50% placement
	entries := []models.ScheduleEntry{
		{Model: gorm.Model{ID: 1}, CourseID: 1, TeacherID: 1, ClassroomID: 1, DayOfWeek: 0, StartPeriod: 0, Span: 2},
		{Model: gorm.Model{ID: 2}, CourseID: 2, TeacherID: 1, ClassroomID: 1, DayOfWeek: 1, StartPeriod: 2, Span: 2},
		{Model: gorm.Model{ID: 3}, CourseID: 3, TeacherID: 1, ClassroomID: 1, DayOfWeek: 2, StartPeriod: 4, Span: 2},
		{Model: gorm.Model{ID: 4}, CourseID: 4, TeacherID: 1, ClassroomID: 1, DayOfWeek: 3, StartPeriod: 6, Span: 2},
	}
	ctx := NewScoringContextWithExpected(FullDefaultConstraints(), nil, nil, 8)
	bd := NewScoringService().ScoreSchedule(entries, teachers, classrooms, ctx)

	if bd.PlacedSessions != 4 {
		t.Fatalf("PlacedSessions = %d, want 4", bd.PlacedSessions)
	}
	if bd.ExpectedSessions != 8 {
		t.Fatalf("ExpectedSessions = %d, want 8", bd.ExpectedSessions)
	}
	if bd.Completeness < 0.499 || bd.Completeness > 0.501 {
		t.Fatalf("Completeness = %.4f, want ≈0.5", bd.Completeness)
	}
	if bd.FinalTotal > 60.0 {
		t.Errorf("Case B ACCEPTANCE FAIL: 50%% completeness FinalTotal = %.2f, must be ≤60", bd.FinalTotal)
	}
	// Total (v0.4 semantics) is not scaled — it should be exactly as it was in v0.5.1.
	if bd.Total < 90 || bd.Total > 100 {
		t.Errorf("Total (v0.4 semantics) = %.2f, expected close to 100 (soft categories all met)", bd.Total)
	}
	t.Logf("Case B OK: Total=%.2f FinalTotal=%.2f (completeness=%.3f)", bd.Total, bd.FinalTotal, bd.Completeness)
}

// Case B': ExpectedTotalSessions=0 (legacy path) → FinalTotal == Total (v0.5.1 backward compat).
func TestFinalTotalLegacyPathEqualsTotal(t *testing.T) {
	teachers := []models.Teacher{{Model: gorm.Model{ID: 1}, Name: "T1"}}
	classrooms := []models.Classroom{{Model: gorm.Model{ID: 1}, Capacity: 100, Floor: 1}}
	entries := []models.ScheduleEntry{
		{Model: gorm.Model{ID: 1}, CourseID: 1, TeacherID: 1, ClassroomID: 1, DayOfWeek: 0, StartPeriod: 0, Span: 2},
	}
	ctx := NewScoringContext(FullDefaultConstraints(), nil, nil) // ExpectedTotalSessions defaults to 0
	bd := NewScoringService().ScoreSchedule(entries, teachers, classrooms, ctx)
	if bd.FinalTotal != bd.Total {
		t.Errorf("Legacy path: FinalTotal=%.2f Total=%.2f — must be equal for backward compat", bd.FinalTotal, bd.Total)
	}
}

// Case B'': 100% completeness → FinalTotal == Total (β(1.0) = 1.0).
func TestFinalTotal100PctEqualsTotal(t *testing.T) {
	teachers := []models.Teacher{{Model: gorm.Model{ID: 1}, Name: "T1"}}
	classrooms := []models.Classroom{{Model: gorm.Model{ID: 1}, Capacity: 100, Floor: 1}}
	entries := []models.ScheduleEntry{
		{Model: gorm.Model{ID: 1}, CourseID: 1, TeacherID: 1, ClassroomID: 1, DayOfWeek: 0, StartPeriod: 0, Span: 2},
	}
	ctx := NewScoringContextWithExpected(FullDefaultConstraints(), nil, nil, 1)
	bd := NewScoringService().ScoreSchedule(entries, teachers, classrooms, ctx)
	if bd.FinalTotal < bd.Total-0.01 || bd.FinalTotal > bd.Total+0.01 {
		t.Errorf("100%% completeness: FinalTotal=%.2f Total=%.2f — should be equal", bd.FinalTotal, bd.Total)
	}
	if bd.Completeness != 1.0 {
		t.Errorf("Completeness = %.4f, want 1.0", bd.Completeness)
	}
}

// v0.5.2 Goal 1 Case A: same entries scored via ScoringService should produce
// identical FinalTotal regardless of which caller (SA / OR-Tools bridge / snapshot)
// invokes it. This documents the "single source of truth" contract.
func TestScoreScheduleDeterministicAcrossCallers(t *testing.T) {
	teachers := []models.Teacher{{Model: gorm.Model{ID: 1}, Name: "T1"}}
	classrooms := []models.Classroom{{Model: gorm.Model{ID: 1}, Capacity: 100, Floor: 1}}
	entries := []models.ScheduleEntry{
		{Model: gorm.Model{ID: 1}, CourseID: 1, TeacherID: 1, ClassroomID: 1, DayOfWeek: 0, StartPeriod: 0, Span: 2},
		{Model: gorm.Model{ID: 2}, CourseID: 2, TeacherID: 1, ClassroomID: 1, DayOfWeek: 2, StartPeriod: 4, Span: 2},
	}
	ctx1 := NewScoringContextWithExpected(FullDefaultConstraints(), nil, nil, 2)
	ctx2 := NewScoringContextWithExpected(FullDefaultConstraints(), nil, nil, 2)

	a := NewScoringService().ScoreSchedule(entries, teachers, classrooms, ctx1).FinalTotal
	b := NewScoringService().ScoreSchedule(entries, teachers, classrooms, ctx2).FinalTotal
	if a != b {
		t.Errorf("Same entries scored via two contexts diverge: a=%.4f b=%.4f", a, b)
	}
	// Repeated scoring on the exact same context must be idempotent.
	c := NewScoringService().ScoreSchedule(entries, teachers, classrooms, ctx1).FinalTotal
	if a != c {
		t.Errorf("Score not idempotent: first=%.4f second=%.4f", a, c)
	}
}

// TestCategoryMaxesWithWeightedScoring verifies that CategoryMaxes are correctly
// computed with weighted scoring — each category's actual max = weight × perCategoryMax.
// This prevents the frontend from showing inflated percentages when weights > 1.
func TestCategoryMaxesWithWeightedScoring(t *testing.T) {
	teachers := []models.Teacher{{Model: gorm.Model{ID: 1}, Name: "T1"}}
	classrooms := []models.Classroom{{Model: gorm.Model{ID: 1}, Name: "R1", Capacity: 100, Floor: 1}}
	entries := []models.ScheduleEntry{
		{Model: gorm.Model{ID: 1}, CourseID: 1, TeacherID: 1, ClassroomID: 1, DayOfWeek: 0, StartPeriod: 0, Span: 2},
	}

	// Weighted: teacher_preference=50, others default=1
	weights := map[string]int{"teacher_preference": 50}
	ctx := NewScoringContextWithExpected(FullDefaultConstraints(), nil, nil, 1).
		WithConstraintWeights(weights)
	bd := NewScoringService().ScoreSchedule(entries, teachers, classrooms, ctx)

	if bd.CategoryMaxes == nil {
		t.Fatal("CategoryMaxes must not be nil")
	}

	// perCategoryMax = 100 / totalWeight. With teacher=50 and 7 other categories at weight=1,
	// totalWeight = 50 + 6 = 56 (low_floor disabled in FULL mode but still counted... actually
	// it depends on enabled constraints). Let's just verify the relationship:
	// teacherMax = 50 * perCategoryMax
	expectedTeacherMax := 50.0 * bd.PerCategoryMax
	actualTeacherMax := bd.CategoryMaxes["teacherPref"]
	if math.Abs(actualTeacherMax-expectedTeacherMax) > 0.1 {
		t.Errorf("teacherPref max: got %.2f, want %.2f (50 × %.2f)",
			actualTeacherMax, expectedTeacherMax, bd.PerCategoryMax)
	}

	// Verify no category score exceeds its max
	if bd.TeacherPref > actualTeacherMax+0.01 {
		t.Errorf("teacherPref score %.2f exceeds max %.2f", bd.TeacherPref, actualTeacherMax)
	}

	// Verify the total is still ≤ 100
	if bd.Total > 100.01 {
		t.Errorf("Total %.2f exceeds 100", bd.Total)
	}

	t.Logf("perCategoryMax=%.2f teacherMax=%.2f teacherPref=%.2f Total=%.2f",
		bd.PerCategoryMax, actualTeacherMax, bd.TeacherPref, bd.Total)
}
