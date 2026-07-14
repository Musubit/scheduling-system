package services

import (
	"encoding/json"
	"math"
	"math/rand"
	"scheduling-system/backend/models"
	"scheduling-system/backend/scheduling/matcher"
	"time"
)

// SASolver implements Simulated Annealing for course scheduling.
// Pure Go, zero external dependencies beyond the standard library.
type SASolver struct{}

func NewSASolver() *SASolver {
	return &SASolver{}
}

// SAConfig holds parameters for the simulated annealing run.
type SAConfig struct {
	InitialTemp       float64 // starting temperature (default 10.0)
	CoolingRate       float64 // multiplicative cooling per step (default 0.95)
	IterationsPerTemp int     // neighbor moves per temperature level (default 200)
	MinTemp           float64 // stop when temperature drops below this (default 0.1)
	MaxTimeSeconds    float64 // maximum solve time (0 = unlimited, default 60)
	Seed              int64   // random seed (0 = time-based)
}

func defaultSAConfig() SAConfig {
	return SAConfig{
		InitialTemp:       10.0,
		CoolingRate:       0.95,
		IterationsPerTemp: 200,
		MinTemp:           0.1,
		MaxTimeSeconds:    60,
	}
}

// SAResult holds the SA solver output.
type SAResult struct {
	Entries    []models.ScheduleEntry
	Score      float64
	Scheduled  int
	Iterations int
	ElapsedMs  int64
}

// teachingTaskData holds pre-loaded data for one teaching task.
type teachingTaskData struct {
	Task           models.TeachingTask
	ClassIDs       []uint // all ClassGroup IDs in this task
	TotalStudents  int    // total students across all class groups (for capacity check)
	CourseHours    int    // course total hours, used to calculate sessions per week
}

// schedulingContext holds all the data needed during solving.
type schedulingContext struct {
	teachingTasks   []teachingTaskData
	teachers        []models.Teacher
	classrooms      []models.Classroom
	classGroups     []models.ClassGroup
	lockedSlots     []LockedTimeSlot
	constraints     []string
	semesterID      uint
	sportsCourseIDs map[uint]bool // course IDs for sports courses

	// v0.5.2: total sessions the solver expects to place across all tasks.
	// Used by ScoreSchedule to compute FinalTotal via completeness scaling.
	expectedTotalSessions int

	// v0.5.2 Goal 3: pre-cached scoring inputs. Rebuilt once at solver start,
	// reused across every computeScore() call to eliminate per-iteration
	// map/slice allocations on the SA hot path.
	cachedTTList  []models.TeachingTask
	cachedScorer  *ScoringService
	cachedScoreCtx ScoringContext

	// v0.5.2 Goal 3 delta-score cache: maintained incrementally in
	// tryMove/trySwap/undoNeighbor so computeScore avoids full ScoreSchedule
	// scans. See sa_scorecache.go. When nil, computeScore falls back to full
	// ScoreSchedule (safe path for legacy callers).
	sCache        *scoreCache
	enabledMap    map[string]bool // memoized enabled set for scoreFromCache

	// Per-teacher unavailable time slots (keyed by teacher ID)
	teacherUnavailable map[uint][]LockedTimeSlot
	// Per-class-group student count (keyed by class group ID)
	classGroupStudents map[uint]int

	// Mutable state
	entries    []models.ScheduleEntry
	// v0.5.2 Goal 3: int-keyed occupancy replaces fmt.Sprintf("%d-%d-%d") keys
	// to eliminate per-iteration string allocation on the SA hot path.
	// Key = uint64(day)<<48 | uint64(period)<<40 | uint64(resourceID)
	roomOcc    map[uint64]bool
	teacherOcc map[uint64]bool
	classOcc   map[uint64]bool

	rng *rand.Rand

	// Per-step undo buffer (was package-level, now per-context for reentrancy)
	lastNeighbor neighborOp
}

// Solve runs simulated annealing and returns the best schedule found.
// cancelCh can be used to interrupt the solver early (nil = no interrupt).
// progressFn is called periodically with (iteration, currentScore, bestScore, temperature).
func (s *SASolver) Solve(
	teachingTasks []models.TeachingTask,
	teachers []models.Teacher,
	classrooms []models.Classroom,
	classGroups []models.ClassGroup,
	lockedSlots []LockedTimeSlot,
	constraints []string,
	semesterID uint,
	config SAConfig,
	cancelCh <-chan struct{},
	progressFn func(iter, total int, currentScore, bestScore, temp float64),
) *SAResult {
	if config.InitialTemp <= 0 {
		cfg := defaultSAConfig()
		config = cfg
	}
	if config.Seed == 0 {
		config.Seed = time.Now().UnixNano()
	}

	rng := rand.New(rand.NewSource(config.Seed))
	startTime := time.Now()

	// Build per-class-group student map
	classGroupStudents := make(map[uint]int, len(classGroups))
	for _, cg := range classGroups {
		classGroupStudents[cg.ID] = cg.Students
	}

	// Build per-teacher unavailable slots map
	teacherUnavailable := make(map[uint][]LockedTimeSlot)
	for _, t := range teachers {
		if t.UnavailableSlots != "" {
			var slots []LockedTimeSlot
			if err := json.Unmarshal([]byte(t.UnavailableSlots), &slots); err == nil && len(slots) > 0 {
				teacherUnavailable[t.ID] = slots
			}
		}
	}

	// Pre-load teaching task data
	taskData := make([]teachingTaskData, len(teachingTasks))
	expectedTotalSessions := 0
	for i, tt := range teachingTasks {
		classIDs := make([]uint, len(tt.Classes))
		totalStudents := 0
		for j, c := range tt.Classes {
			classIDs[j] = c.ClassGroupID
			totalStudents += classGroupStudents[c.ClassGroupID]
		}
		courseHours := tt.TotalHours
		if courseHours <= 0 {
			courseHours = tt.Course.Hours // fallback to course default
		}
		taskData[i] = teachingTaskData{
			Task:          tt,
			ClassIDs:      classIDs,
			TotalStudents: totalStudents,
			CourseHours:   courseHours,
		}
		// v0.5.2: expected sessions from resolveSessionPlan (v0.5.1 authoritative source).
		plan := resolveSessionPlan(courseHours, tt.StartWeek, tt.EndWeek, tt.MaxHoursPerWeek, tt.PreferredSpan)
		expectedTotalSessions += plan.SessionsPerWeek()
	}

	// Build context
	sportsCourseIDs := make(map[uint]bool)
	for _, tt := range teachingTasks {
		if models.IsSportsCourse(tt.Course.Name) {
			sportsCourseIDs[tt.CourseID] = true
		}
	}

	ctx := &schedulingContext{
		teachingTasks:         taskData,
		teachers:              teachers,
		classrooms:            classrooms,
		classGroups:           classGroups,
		lockedSlots:           lockedSlots,
		constraints:           constraints,
		semesterID:            semesterID,
		sportsCourseIDs:       sportsCourseIDs,
		teacherUnavailable:    teacherUnavailable,
		classGroupStudents:    classGroupStudents,
		expectedTotalSessions: expectedTotalSessions,
		rng:                   rng,
	}

	// v0.5.2 Goal 3: pre-build the ScoringContext once. computeScore() reuses
	// this cached instance to avoid per-neighbor map/slice allocations that
	// dominated pprof samples.
	ctx.cachedTTList = make([]models.TeachingTask, len(taskData))
	for i, td := range taskData {
		ctx.cachedTTList[i] = td.Task
	}
	ctx.cachedScorer = NewScoringService()
	ctx.cachedScoreCtx = NewScoringContextWithExpected(constraints, sportsCourseIDs, ctx.cachedTTList, expectedTotalSessions)

	// v0.5.2 Goal 3 delta-score: build cache & memoize enabled set.
	ctx.sCache = newScoreCache(teachers, classrooms, taskData)
	ctx.enabledMap = make(map[string]bool, len(constraints))
	{
		set := constraints
		if len(set) == 0 {
			set = FullDefaultConstraints()
		}
		for _, c := range set {
			ctx.enabledMap[c] = true
		}
	}

	// Phase 1: Generate initial solution with greedy construction
	ctx.buildInitial()

	// v0.5.2 Goal 3: seed the delta cache from the initial solution.
	// From this point on the cache is authoritative — tryMove/trySwap/undo
	// keep it in sync via applyDelta.
	ctx.sCache.rebuildFromEntries(ctx.entries)
	// Also patch isSports flag on cache (rebuildFromEntries didn't have context).
	// applyEntry needs sportsCourseIDs lookup — do a symmetric replay:
	ctx.sCache.rebuildFromEntries(nil)
	for _, e := range ctx.entries {
		ctx.sCache.applyEntry(+1, e, ctx.sportsCourseIDs[e.CourseID])
	}

	// Score initial solution
	currentScore := ctx.computeScore()
	bestEntries := make([]models.ScheduleEntry, len(ctx.entries))
	copy(bestEntries, ctx.entries)
	bestScore := currentScore

	temperature := config.InitialTemp
	totalIterations := 0

	// Estimated total iterations for progress reporting
	estimatedTempSteps := int(math.Log(config.MinTemp/config.InitialTemp) / math.Log(config.CoolingRate))
	if estimatedTempSteps < 0 {
		estimatedTempSteps = 100
	}
	estimatedTotal := estimatedTempSteps * config.IterationsPerTemp

	for temperature > config.MinTemp {
		// Check time limit
		if config.MaxTimeSeconds > 0 {
			elapsed := time.Since(startTime).Seconds()
			if elapsed >= config.MaxTimeSeconds {
				break
			}
		}

		// Check cancel
		select {
		case <-cancelCh:
			goto done
		default:
		}

		for i := 0; i < config.IterationsPerTemp; i++ {
			totalIterations++

			// Generate neighbor
			neighborScore := ctx.tryNeighbor(currentScore)
			delta := neighborScore - currentScore

			// Accept if better, or with probability if worse
			if delta > 0 || (temperature > 0 && rng.Float64() < math.Exp(delta/temperature)) {
				currentScore = neighborScore
				if currentScore > bestScore {
					bestScore = currentScore
					bestEntries = make([]models.ScheduleEntry, len(ctx.entries))
					copy(bestEntries, ctx.entries)
				}
			} else {
				// Reject: undo the neighbor move
				ctx.undoNeighbor()
			}

			// Check cancel frequently
			if i%10 == 0 {
				select {
				case <-cancelCh:
					goto done
				default:
				}
			}
		}

		// Report progress
		if progressFn != nil {
			progressFn(totalIterations, estimatedTotal, currentScore, bestScore, temperature)
		}

		temperature *= config.CoolingRate
	}

done:
	elapsed := time.Since(startTime).Milliseconds()

	return &SAResult{
		Entries:    bestEntries,
		Score:      bestScore,
		Scheduled:  len(bestEntries),
		Iterations: totalIterations,
		ElapsedMs:  elapsed,
	}
}

// buildInitial constructs a greedy initial solution from teaching tasks.
func (ctx *schedulingContext) removeOccupancy(e models.ScheduleEntry) {
	day, start, span := int(e.DayOfWeek), int(e.StartPeriod), e.Span
	for p := start; p < start+span; p++ {
		delete(ctx.roomOcc, occKey(day, p, e.ClassroomID))
		delete(ctx.teacherOcc, occKey(day, p, e.TeacherID))
	}
	// Remove class group occupancy for all classes in the teaching task
	if e.TeachingTaskID != nil {
		for _, td := range ctx.teachingTasks {
			if td.Task.ID == *e.TeachingTaskID {
				for _, cid := range td.ClassIDs {
					for p := start; p < start+span; p++ {
						delete(ctx.classOcc, occKey(day, p, cid))
					}
				}
				break
			}
		}
	}
}

func (ctx *schedulingContext) addOccupancy(e models.ScheduleEntry) {
	day, start, span := int(e.DayOfWeek), int(e.StartPeriod), e.Span
	// Find the classroom to check if it's a shared venue (体育馆)
	isShared := false
	for _, room := range ctx.classrooms {
		if room.ID == e.ClassroomID && room.RoomType == models.RoomTypeGym {
			isShared = true
			break
		}
	}
	if !isShared {
		for p := start; p < start+span; p++ {
			ctx.roomOcc[occKey(day, p, e.ClassroomID)] = true
		}
	}
	for p := start; p < start+span; p++ {
		ctx.teacherOcc[occKey(day, p, e.TeacherID)] = true
	}
	// Add class group occupancy for all classes in the teaching task
	if e.TeachingTaskID != nil {
		for _, td := range ctx.teachingTasks {
			if td.Task.ID == *e.TeachingTaskID {
				for _, cid := range td.ClassIDs {
					for p := start; p < start+span; p++ {
						ctx.classOcc[occKey(day, p, cid)] = true
					}
				}
				break
			}
		}
	}
}

func (ctx *schedulingContext) restoreOccupancy(e models.ScheduleEntry) {
	ctx.addOccupancy(e)
}

func (ctx *schedulingContext) hasConstraint(key string) bool {
	for _, c := range ctx.constraints {
		if c == key {
			return true
		}
	}
	return false
}

// SolveMultiRun runs SA multiple times with different random seeds and returns the best result.
// This implements the "multi-restart" strategy (algorithm C).
func (s *SASolver) SolveMultiRun(
	teachingTasks []models.TeachingTask,
	teachers []models.Teacher,
	classrooms []models.Classroom,
	classGroups []models.ClassGroup,
	lockedSlots []LockedTimeSlot,
	constraints []string,
	semesterID uint,
	config SAConfig,
	runs int,
	cancelCh <-chan struct{},
	progressFn func(iter, total int, currentScore, bestScore, temp float64),
) *SAResult {
	if runs <= 0 {
		runs = 3 // default: 3 runs
	}

	// Distribute time budget across runs
	timePerRun := config.MaxTimeSeconds / float64(runs)
	runConfig := config
	runConfig.MaxTimeSeconds = timePerRun

	var bestResult *SAResult
	totalIterations := 0

	for i := 0; i < runs; i++ {
		// Check cancel
		select {
		case <-cancelCh:
			break
		default:
		}

		// Use different seed per run
		runConfig.Seed = time.Now().UnixNano() + int64(i*31337)

		result := s.Solve(teachingTasks, teachers, classrooms, classGroups,
			lockedSlots, constraints, semesterID, runConfig, cancelCh, nil)

		totalIterations += result.Iterations

		if bestResult == nil || result.Score > bestResult.Score {
			bestResult = result
		}
	}

	bestResult.Iterations = totalIterations
	return bestResult
}

// PostOptimize performs greedy post-optimization on the best solution (algorithm D).
// It identifies the N lowest-scoring entries and tries to improve them via exhaustive
// local search of all feasible (day × period × room) combinations.
func periodsOverlapInt(start1, span1, start2, span2 int) bool {
	end1 := start1 + span1
	end2 := start2 + span2
	return start1 < end2 && start2 < end1
}

// isTeacherUnavailable checks if a teacher has an unavailable time slot overlapping (day, start, span).
func (ctx *schedulingContext) isTeacherUnavailable(teacherID uint, day, start, span int) bool {
	slots, ok := ctx.teacherUnavailable[teacherID]
	if !ok {
		return false
	}
	for _, u := range slots {
		if int(u.DayOfWeek) == day && periodsOverlapInt(start, span, int(u.StartPeriod), u.Span) {
			return true
		}
	}
	return false
}

// canRoomFitCapacity checks if a classroom's capacity is sufficient for the given teaching task.
// Sports venues (体育馆) are shared spaces that can accommodate any number of students.
func (ctx *schedulingContext) canRoomFitCapacity(classroom models.Classroom, taskData *teachingTaskData) bool {
	if taskData.TotalStudents <= 0 {
		return true
	}
	if classroom.RoomType == models.RoomTypeGym {
		return true // sports venues have unlimited effective capacity
	}
	return classroom.Capacity >= taskData.TotalStudents
}

// occKey encodes (day, period, resourceID) into a single uint64 for O(1) map ops
// without fmt.Sprintf allocation. day∈[0,6], period∈[0,10], resourceID:uint32-safe.
func occKey(day, period int, id uint) uint64 {
	return uint64(uint32(day))<<48 | uint64(uint32(period))<<40 | uint64(id)
}

// findTaskDataByEntry finds the teachingTaskData for a given schedule entry.
func (ctx *schedulingContext) findTaskDataByEntry(e models.ScheduleEntry) *teachingTaskData {
	if e.TeachingTaskID == nil {
		return nil
	}
	for i := range ctx.teachingTasks {
		if ctx.teachingTasks[i].Task.ID == *e.TeachingTaskID {
			return &ctx.teachingTasks[i]
		}
	}
	return nil
}

// getRequiredRoomType is deprecated. Use ResourceMatcher.Match() or InferRoomType() instead.
// Kept for backward compatibility; will be removed in v0.6.
func (ctx *schedulingContext) getRequiredRoomType(courseName string) string {
	return matcher.InferRoomTypeByName(courseName)
}
