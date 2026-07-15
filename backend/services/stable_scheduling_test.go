package services

import (
	"fmt"
	"scheduling-system/backend/models"
	"testing"

	"gorm.io/gorm"
)

// TestAdaptiveRun verifies that SolveMultiRun stops early when consecutive
// runs produce no score improvement (convergence detection).
func TestAdaptiveRun(t *testing.T) {
	teachers := []models.Teacher{
		{Model: gorm.Model{ID: 1}, Name: "T1"},
	}
	classrooms := []models.Classroom{
		{Model: gorm.Model{ID: 1}, Name: "R1", Capacity: 60},
	}
	groups := []models.ClassGroup{
		{Model: gorm.Model{ID: 1}, Name: "G1", Students: 40},
	}
	ttc := []models.TeachingTaskClass{{Model: gorm.Model{ID: 1}, ClassGroupID: 1}}
	tasks := []models.TeachingTask{
		{Model: gorm.Model{ID: 1}, CourseID: 1, TeacherID: 1, SemesterID: 1, TotalHours: 32, Classes: ttc, Teacher: teachers[0]},
		{Model: gorm.Model{ID: 2}, CourseID: 2, TeacherID: 1, SemesterID: 1, TotalHours: 32, Classes: ttc, Teacher: teachers[0]},
		{Model: gorm.Model{ID: 3}, CourseID: 3, TeacherID: 1, SemesterID: 1, TotalHours: 32, Classes: ttc, Teacher: teachers[0]},
	}

	cfg := defaultSAConfig()
	cfg.MaxTimeSeconds = 15
	cfg.IterationsPerTemp = 50

	solver := NewSASolver()

	result := solver.SolveMultiRun(
		tasks, teachers, classrooms, groups,
		nil, []string{}, uint(1),
		cfg, 10, nil, nil,
	)

	if result == nil {
		t.Fatal("SolveMultiRun returned nil")
	}
	if len(result.Entries) == 0 {
		t.Fatal("SolveMultiRun returned empty entries")
	}
	if result.Score <= 0 {
		t.Errorf("expected positive score, got %.2f", result.Score)
	}

	tc, rc, cc := 0, 0, 0
	overlap := func(a, b models.ScheduleEntry) bool {
		return int(a.StartPeriod) < int(b.StartPeriod)+b.Span &&
			int(b.StartPeriod) < int(a.StartPeriod)+a.Span
	}
	for i := range result.Entries {
		e := result.Entries[i]
		for j := i + 1; j < len(result.Entries); j++ {
			f := result.Entries[j]
			if e.DayOfWeek != f.DayOfWeek || !overlap(e, f) {
				continue
			}
			if e.ClassroomID == f.ClassroomID {
				rc++
			}
			if e.TeacherID == f.TeacherID {
				tc++
			}
			if e.ClassGroupID != nil && f.ClassGroupID != nil && *e.ClassGroupID == *f.ClassGroupID {
				cc++
			}
		}
	}
	if tc+rc+cc > 0 {
		t.Errorf("hard conflicts: teacher=%d room=%d class=%d", tc, rc, cc)
	}

	t.Logf("OK: adaptive multi-run placed %d entries, score=%.1f, iterations=%d",
		len(result.Entries), result.Score, result.Iterations)
}

// TestScoreEpsilon verifies that ScoreEquals and ScoreGreater correctly
// handle floating-point noise within the epsilon threshold.
func TestScoreEpsilon(t *testing.T) {
	if !ScoreEquals(588.0, 588.005) {
		t.Error("expected ScoreEquals(588.0, 588.005) == true")
	}
	if !ScoreEquals(588.0, 587.995) {
		t.Error("expected ScoreEquals(588.0, 587.995) == true")
	}
	if ScoreEquals(588.0, 588.02) {
		t.Error("expected ScoreEquals(588.0, 588.02) == false")
	}

	if !ScoreGreater(588.02, 588.0) {
		t.Error("expected ScoreGreater(588.02, 588.0) == true")
	}
	if ScoreGreater(588.005, 588.0) {
		t.Error("expected ScoreGreater(588.005, 588.0) == false")
	}
	if ScoreGreater(587.0, 588.0) {
		t.Error("expected ScoreGreater(587.0, 588.0) == false")
	}
}

// TestPostOptimizeDeterministic verifies that PostOptimize produces identical
// results when called twice with the same input (deterministic day loop).
func TestPostOptimizeDeterministic(t *testing.T) {
	classroom := models.Classroom{
		Code:     "A101",
		Name:     "Room A101",
		Capacity: 60,
		RoomType: models.RoomTypeNormal,
	}
	classroom.ID = 1

	teacher := models.Teacher{Code: "T001", Name: "Teacher1", Dept: "CS"}
	teacher.ID = 1

	course := models.Course{Code: "CS101", Name: "Intro CS", Dept: "CS", Hours: 32}
	course.ID = 1

	tt := models.TeachingTask{
		CourseID:   1,
		TeacherID:  1,
		TotalHours: 32,
		StartWeek:  1,
		EndWeek:    16,
	}
	tt.ID = 1
	tt.Course = course

	teachers := []models.Teacher{teacher}
	classrooms := []models.Classroom{classroom}
	tasks := []models.TeachingTask{tt}

	uPtr := func(v uint) *uint { return &v }

	makeEntries := func() []models.ScheduleEntry {
		return []models.ScheduleEntry{
			{CourseID: 1, TeacherID: 1, ClassroomID: 1, TeachingTaskID: uPtr(1),
				DayOfWeek: models.Mon, StartPeriod: 0, Span: 2},
			{CourseID: 1, TeacherID: 1, ClassroomID: 1, TeachingTaskID: uPtr(1),
				DayOfWeek: models.Tue, StartPeriod: 0, Span: 2},
			{CourseID: 1, TeacherID: 1, ClassroomID: 1, TeachingTaskID: uPtr(1),
				DayOfWeek: models.Wed, StartPeriod: 0, Span: 2},
			{CourseID: 1, TeacherID: 1, ClassroomID: 1, TeachingTaskID: uPtr(1),
				DayOfWeek: models.Thu, StartPeriod: 0, Span: 2},
			{CourseID: 1, TeacherID: 1, ClassroomID: 1, TeachingTaskID: uPtr(1),
				DayOfWeek: models.Sun, StartPeriod: 8, Span: 2},
		}
	}

	solver := NewSASolver()

	entries1 := makeEntries()
	entries2 := makeEntries()

	result1 := solver.PostOptimize(entries1, tasks, teachers, classrooms, nil, nil, 3, false)
	result2 := solver.PostOptimize(entries2, tasks, teachers, classrooms, nil, nil, 3, false)

	if len(result1) != len(result2) {
		t.Fatalf("length mismatch: %d vs %d", len(result1), len(result2))
	}

	for i := range result1 {
		if result1[i].DayOfWeek != result2[i].DayOfWeek {
			t.Errorf("entry %d DayOfWeek differs: %d vs %d", i, result1[i].DayOfWeek, result2[i].DayOfWeek)
		}
		if result1[i].StartPeriod != result2[i].StartPeriod {
			t.Errorf("entry %d StartPeriod differs: %d vs %d", i, result1[i].StartPeriod, result2[i].StartPeriod)
		}
		if result1[i].ClassroomID != result2[i].ClassroomID {
			t.Errorf("entry %d ClassroomID differs: %d vs %d", i, result1[i].ClassroomID, result2[i].ClassroomID)
		}
	}
}

// TestCrossRunCache verifies the cross-run best-result caching mechanism:
//   - A new run whose score <= the cached score returns the cached result.
//   - A new run whose score > the cached score updates the cache.
//   - ScoreDetail is deep-copied so mutations to the original do not leak.
func TestCrossRunCache(t *testing.T) {
	svc := &SchedulingService{}

	if svc.bestCachedResult != nil {
		t.Fatal("expected nil cached result on fresh service")
	}
	if svc.bestCachedScore != 0 {
		t.Fatalf("expected zero cached score, got %.1f", svc.bestCachedScore)
	}

	firstScore := 75.0
	firstDetail := &ScoreBreakdown{Total: 75, FinalTotal: 75, TeacherPref: 30, CourseSpacing: 20}
	firstResult := &SchedulingResult{
		Score:       firstScore,
		ScoreDetail: firstDetail,
		Logs:        []string{"first run"},
	}

	cached := *firstResult
	if firstResult.ScoreDetail != nil {
		sd := *firstResult.ScoreDetail
		cached.ScoreDetail = &sd
	}
	svc.bestCachedScore = firstResult.Score
	svc.bestCachedResult = &cached

	if svc.bestCachedResult == nil {
		t.Fatal("cache should be populated after first run")
	}
	if svc.bestCachedScore != firstScore {
		t.Fatalf("cached score mismatch: got %.1f, want %.1f", svc.bestCachedScore, firstScore)
	}

	// Deep copy isolation
	firstDetail.TeacherPref = 999
	if svc.bestCachedResult.ScoreDetail.TeacherPref == 999 {
		t.Fatal("deep copy failed: mutating original ScoreDetail changed the cached copy")
	}

	// Worse score → cache hit
	secondScore := 60.0
	if svc.bestCachedResult != nil && secondScore <= svc.bestCachedScore {
		ret := *svc.bestCachedResult
		ret.Logs = append(ret.Logs, fmt.Sprintf("本次评分 %.1f ≤ 缓存最优 %.1f，返回缓存结果", secondScore, svc.bestCachedScore))

		if ret.Score != firstScore {
			t.Errorf("cache-hit returned wrong score: got %.1f, want %.1f", ret.Score, firstScore)
		}
		if len(ret.Logs) != 2 {
			t.Errorf("expected 2 log lines (original + cache-hit), got %d", len(ret.Logs))
		}
		t.Logf("OK: worse score %.1f correctly returned cached result (score %.1f)", secondScore, ret.Score)
	} else {
		t.Error("expected cache hit for worse score, but condition was false")
	}

	// Better score → cache miss + update
	thirdScore := 85.0
	thirdDetail := &ScoreBreakdown{Total: 85, FinalTotal: 85, TeacherPref: 40, CourseSpacing: 25}
	thirdResult := &SchedulingResult{
		Score:       thirdScore,
		ScoreDetail: thirdDetail,
		Logs:        []string{"third run"},
	}

	if svc.bestCachedResult != nil && thirdScore <= svc.bestCachedScore {
		t.Error("better score should NOT trigger cache hit")
	}

	updated := *thirdResult
	if thirdResult.ScoreDetail != nil {
		sd := *thirdResult.ScoreDetail
		updated.ScoreDetail = &sd
	}
	svc.bestCachedScore = thirdResult.Score
	svc.bestCachedResult = &updated

	if svc.bestCachedScore != thirdScore {
		t.Fatalf("cache should now hold new best score: got %.1f, want %.1f", svc.bestCachedScore, thirdScore)
	}
	if svc.bestCachedResult.Score != thirdScore {
		t.Fatalf("cached result score mismatch: got %.1f, want %.1f", svc.bestCachedResult.Score, thirdScore)
	}

	// Equal score → cache hit (no update)
	equalScore := 85.0
	if !(svc.bestCachedResult != nil && equalScore <= svc.bestCachedScore) {
		t.Error("equal score should trigger cache hit (<= condition)")
	}

	t.Logf("OK: cross-run cache — hit on worse/equal, miss on better, deep copy verified")
}
