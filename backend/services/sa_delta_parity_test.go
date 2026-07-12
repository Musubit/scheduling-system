package services

import (
	"math/rand"
	"scheduling-system/backend/models"
	"testing"
)

// TestDeltaScoreMatchesFullScore proves the incremental cache maintains
// perfect parity with ScoreSchedule across a randomized neighbor sequence.
// If any move or undo drifts the cache, this test fails.
func TestDeltaScoreMatchesFullScore(t *testing.T) {
	tasks, teachers, classrooms, classGroups := buildBenchmarkFixture()
	config := SAConfig{
		InitialTemp: 10.0, CoolingRate: 0.9,
		IterationsPerTemp: 10, MinTemp: 5.0,
		MaxTimeSeconds: 1, Seed: 777,
	}
	solver := NewSASolver()
	result := solver.Solve(tasks, teachers, classrooms, classGroups,
		nil, FullDefaultConstraints(), "2025-S2", config, nil, nil)
	if len(result.Entries) == 0 {
		t.Fatalf("no entries produced")
	}

	// Build a standalone context to drive tryMove/undoNeighbor deterministically.
	ctx := freshContextFromEntries(result.Entries, tasks, teachers, classrooms, classGroups)

	// Build a reference ScoringContext to invoke the full ScoreSchedule after each op.
	expected := 0
	for _, tt := range tasks {
		th := tt.TotalHours
		if th <= 0 {
			th = tt.Course.Hours
		}
		plan := resolveSessionPlan(th, tt.StartWeek, tt.EndWeek, tt.MaxHoursPerWeek, tt.PreferredSpan)
		expected += plan.SessionsPerWeek()
	}
	sports := make(map[uint]bool)
	for _, tt := range tasks {
		if models.IsSportsCourse(tt.Course.Name) {
			sports[tt.CourseID] = true
		}
	}
	ttList := make([]models.TeachingTask, len(tasks))
	copy(ttList, tasks)
	refCtx := NewScoringContextWithExpected(FullDefaultConstraints(), sports, ttList, expected)
	refScorer := NewScoringService()

	assertParity := func(tag string) {
		cache := ctx.sCache.scoreFromCache(ctx.enabledMap, ctx.sportsCourseIDs, ctx.expectedTotalSessions)
		full := refScorer.ScoreSchedule(ctx.entries, teachers, classrooms, refCtx)
		if diff := cache.FinalTotal - full.FinalTotal; diff > 0.02 || diff < -0.02 {
			t.Fatalf("%s: FinalTotal drift cache=%.4f full=%.4f Δ=%.4f", tag, cache.FinalTotal, full.FinalTotal, diff)
		}
	}

	assertParity("initial")

	rng := rand.New(rand.NewSource(1234))
	ctx.rng = rng
	for i := 0; i < 300; i++ {
		before := ctx.computeScore()
		after := ctx.tryNeighbor(before)
		assertParity("after tryNeighbor #" + itoa(i))
		// Randomly reject and undo, exercising the undo path.
		if rng.Float64() < 0.5 && after != before {
			ctx.undoNeighbor()
			assertParity("after undo #" + itoa(i))
		}
	}
}

// freshContextFromEntries builds a schedulingContext seeded with the given
// entries, mirroring what SASolver.Solve does before the SA loop.
func freshContextFromEntries(
	entries []models.ScheduleEntry,
	tasks []models.TeachingTask,
	teachers []models.Teacher,
	classrooms []models.Classroom,
	classGroups []models.ClassGroup,
) *schedulingContext {
	classGroupStudents := make(map[uint]int, len(classGroups))
	for _, cg := range classGroups {
		classGroupStudents[cg.ID] = cg.Students
	}
	taskData := make([]teachingTaskData, len(tasks))
	expected := 0
	for i, tt := range tasks {
		classIDs := make([]uint, len(tt.Classes))
		total := 0
		for j, c := range tt.Classes {
			classIDs[j] = c.ClassGroupID
			total += classGroupStudents[c.ClassGroupID]
		}
		hours := tt.TotalHours
		if hours <= 0 {
			hours = tt.Course.Hours
		}
		taskData[i] = teachingTaskData{Task: tt, ClassIDs: classIDs, TotalStudents: total, CourseHours: hours}
		plan := resolveSessionPlan(hours, tt.StartWeek, tt.EndWeek, tt.MaxHoursPerWeek, tt.PreferredSpan)
		expected += plan.SessionsPerWeek()
	}
	sports := make(map[uint]bool)
	for _, tt := range tasks {
		if models.IsSportsCourse(tt.Course.Name) {
			sports[tt.CourseID] = true
		}
	}
	ctx := &schedulingContext{
		teachingTasks:         taskData,
		teachers:              teachers,
		classrooms:            classrooms,
		classGroups:           classGroups,
		constraints:           FullDefaultConstraints(),
		semester:              "2025-S2",
		sportsCourseIDs:       sports,
		teacherUnavailable:    make(map[uint][]LockedTimeSlot),
		classGroupStudents:    classGroupStudents,
		expectedTotalSessions: expected,
		entries:               make([]models.ScheduleEntry, len(entries)),
		roomOcc:               make(map[uint64]bool),
		teacherOcc:            make(map[uint64]bool),
		classOcc:              make(map[uint64]bool),
		rng:                   rand.New(rand.NewSource(1)),
	}
	copy(ctx.entries, entries)
	ctx.cachedTTList = make([]models.TeachingTask, len(taskData))
	for i, td := range taskData {
		ctx.cachedTTList[i] = td.Task
		for _, e := range ctx.entries {
			ctx.addOccupancy(e)
			break // only the first iter primes maps to real seed later
		}
	}
	// Reset occupancy and re-seed properly
	ctx.roomOcc = make(map[uint64]bool)
	ctx.teacherOcc = make(map[uint64]bool)
	ctx.classOcc = make(map[uint64]bool)
	for i := range ctx.entries {
		ctx.addOccupancy(ctx.entries[i])
	}
	ctx.cachedScorer = NewScoringService()
	ctx.cachedScoreCtx = NewScoringContextWithExpected(ctx.constraints, sports, ctx.cachedTTList, expected)
	ctx.sCache = newScoreCache(teachers, classrooms, taskData)
	ctx.enabledMap = make(map[string]bool)
	for _, c := range ctx.constraints {
		ctx.enabledMap[c] = true
	}
	// seed cache
	for _, e := range ctx.entries {
		ctx.sCache.applyEntry(+1, e, ctx.sportsCourseIDs[e.CourseID])
	}
	return ctx
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := make([]byte, 0, 12)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
