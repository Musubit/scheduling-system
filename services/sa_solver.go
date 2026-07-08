package services

import (
	"fmt"
	"math"
	"math/rand"
	"scheduling-system/models"
	"strings"
	"time"
)

// SASolver implements Simulated Annealing for course scheduling.
// Pure Go, zero external dependencies beyond the standard library.
type SASolver struct {
	scorer *ScoringService
}

func NewSASolver() *SASolver {
	return &SASolver{scorer: NewScoringService()}
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
	Task        models.TeachingTask
	ClassIDs    []uint // all ClassGroup IDs in this task
}

// schedulingContext holds all the data needed during solving.
type schedulingContext struct {
	teachingTasks   []teachingTaskData
	teachers        []models.Teacher
	classrooms      []models.Classroom
	classGroups     []models.ClassGroup
	lockedSlots     []lockedTimeSlot
	constraints     []string
	semester        string
	sportsCourseIDs map[uint]bool // course IDs for sports courses

	// Mutable state
	entries    []models.ScheduleEntry
	roomOcc    map[string]bool // "day-period-roomID"
	teacherOcc map[string]bool
	classOcc   map[string]bool

	rng *rand.Rand
}

// Solve runs simulated annealing and returns the best schedule found.
// cancelCh can be used to interrupt the solver early (nil = no interrupt).
// progressFn is called periodically with (iteration, currentScore, bestScore, temperature).
func (s *SASolver) Solve(
	teachingTasks []models.TeachingTask,
	teachers []models.Teacher,
	classrooms []models.Classroom,
	classGroups []models.ClassGroup,
	lockedSlots []lockedTimeSlot,
	constraints []string,
	semester string,
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

	// Pre-load teaching task data
	taskData := make([]teachingTaskData, len(teachingTasks))
	for i, tt := range teachingTasks {
		classIDs := make([]uint, len(tt.Classes))
		for j, c := range tt.Classes {
			classIDs[j] = c.ClassGroupID
		}
		taskData[i] = teachingTaskData{
			Task:     tt,
			ClassIDs: classIDs,
		}
	}

	// Build context
	sportsCourseIDs := make(map[uint]bool)
	for _, tt := range teachingTasks {
		if strings.Contains(tt.Course.Name, "体育") {
			sportsCourseIDs[tt.CourseID] = true
		}
	}

	ctx := &schedulingContext{
		teachingTasks:   taskData,
		teachers:        teachers,
		classrooms:      classrooms,
		classGroups:     classGroups,
		lockedSlots:     lockedSlots,
		constraints:     constraints,
		semester:        semester,
		sportsCourseIDs: sportsCourseIDs,
		rng:             rng,
	}

	// Phase 1: Generate initial solution with greedy construction
	ctx.buildInitial()

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
func (ctx *schedulingContext) buildInitial() {
	ctx.entries = nil
	ctx.roomOcc = make(map[string]bool)
	ctx.teacherOcc = make(map[string]bool)
	ctx.classOcc = make(map[string]bool)

	validStarts := []int{0, 2, 4, 6, 8} // 第1/3/5/7/9节

	// Shuffle teaching tasks for randomness
	taskOrder := make([]int, len(ctx.teachingTasks))
	for i := range taskOrder {
		taskOrder[i] = i
	}
	ctx.rng.Shuffle(len(taskOrder), func(i, j int) { taskOrder[i], taskOrder[j] = taskOrder[j], taskOrder[i] })

	for _, ti := range taskOrder {
		td := ctx.teachingTasks[ti]
		placed := false

		// Try days in random order, but push weekends to end if avoidance is on
		baseDays := ctx.rng.Perm(7)
		var days []int
		if ctx.hasConstraint("avoid_saturday") || ctx.hasConstraint("avoid_sunday") {
			prefer, avoid := []int{}, []int{}
			for _, d := range baseDays {
				isAvoid := false
				if ctx.hasConstraint("avoid_saturday") && d == int(models.Sat) {
					isAvoid = true
				}
				if ctx.hasConstraint("avoid_sunday") && d == int(models.Sun) {
					isAvoid = true
				}
				if isAvoid {
					avoid = append(avoid, d)
				} else {
					prefer = append(prefer, d)
				}
			}
			ctx.rng.Shuffle(len(prefer), func(i, j int) { prefer[i], prefer[j] = prefer[j], prefer[i] })
			days = append(prefer, avoid...)
		} else {
			days = baseDays
		}

		for _, day := range days {
			if placed {
				break
			}

			starts := make([]int, len(validStarts))
			copy(starts, validStarts)
			// For sports courses, prefer start=2 (3-4节) and start=6 (7-8节)
			if ctx.hasConstraint("pe_preferred_periods") && ctx.sportsCourseIDs[td.Task.CourseID] {
				prefer := []int{2, 6}
				other := []int{0, 4, 8}
				ctx.rng.Shuffle(len(prefer), func(i, j int) { prefer[i], prefer[j] = prefer[j], prefer[i] })
				ctx.rng.Shuffle(len(other), func(i, j int) { other[i], other[j] = other[j], other[i] })
				starts = append(prefer, other...)
			} else {
				ctx.rng.Shuffle(len(starts), func(i, j int) { starts[i], starts[j] = starts[j], starts[i] })
			}

			for _, start := range starts {
				if placed {
					break
				}
				span := 2

				// Check locked slots
				locked := false
				for _, ls := range ctx.lockedSlots {
					if int(ls.DayOfWeek) == day && periodsOverlapInt(start, span, int(ls.StartPeriod), ls.Span) {
						locked = true
						break
					}
				}
				if locked {
					continue
				}

				// Check teacher preferences
				if ctx.hasConstraint("teacher_preference") {
					for _, t := range ctx.teachers {
						if t.ID == td.Task.TeacherID {
							if t.PreferNoEarly && start <= 1 {
								continue
							}
							if t.PreferNoLate && start >= 6 {
								continue
							}
							break
						}
					}
				}

				// Check teacher busy
				teacherBusy := false
				for p := start; p < start+span; p++ {
					key := fmt.Sprintf("%d-%d-%d", day, p, td.Task.TeacherID)
					if ctx.teacherOcc[key] {
						teacherBusy = true
						break
					}
				}
				if teacherBusy {
					continue
				}

				// Check all class groups are free (combined class support)
				classBusy := false
				for _, cid := range td.ClassIDs {
					for p := start; p < start+span; p++ {
						key := fmt.Sprintf("%d-%d-%d", day, p, cid)
						if ctx.classOcc[key] {
							classBusy = true
							break
						}
					}
					if classBusy {
						break
					}
				}
				if classBusy {
					continue
				}

				// Try rooms
				rooms := make([]models.Classroom, len(ctx.classrooms))
				copy(rooms, ctx.classrooms)
				ctx.rng.Shuffle(len(rooms), func(i, j int) { rooms[i], rooms[j] = rooms[j], rooms[i] })

				for _, room := range rooms {
					roomBusy := false
					for p := start; p < start+span; p++ {
						key := fmt.Sprintf("%d-%d-%d", day, p, room.ID)
						if ctx.roomOcc[key] {
							roomBusy = true
							break
						}
					}
					if roomBusy {
						continue
					}

					// All constraints satisfied, create entry
					entry := models.ScheduleEntry{
						CourseID:       td.Task.CourseID,
						TeacherID:      td.Task.TeacherID,
						ClassroomID:    room.ID,
						TeachingTaskID: &td.Task.ID,
						ClassGroupID:   nil, // controlled by TeachingTask now
						Semester:       ctx.semester,
						DayOfWeek:      models.DayOfWeek(day),
						StartPeriod:    models.Period(start),
						Span:           span,
						Weeks:          "1-16",
					}
					ctx.entries = append(ctx.entries, entry)

					// Occupy room, teacher, and all class groups
					for p := start; p < start+span; p++ {
						ctx.roomOcc[fmt.Sprintf("%d-%d-%d", day, p, room.ID)] = true
						ctx.teacherOcc[fmt.Sprintf("%d-%d-%d", day, p, td.Task.TeacherID)] = true
					}
					for _, cid := range td.ClassIDs {
						for p := start; p < start+span; p++ {
							ctx.classOcc[fmt.Sprintf("%d-%d-%d", day, p, cid)] = true
						}
					}
					placed = true
					break
				}
			}
		}
		if !placed {
			// Teaching task could not be placed — will be noted in result
			_ = placed
		}
	}
}

// Neighbor operation types
type neighborOp struct {
	kind        string // "move" or "swap"
	entryIdx    int    // index in ctx.entries for "move"
	swapIdx1    int    // first index for "swap"
	swapIdx2    int    // second index for "swap"
	oldDay      models.DayOfWeek
	oldStart    models.Period
	oldRoom     uint
	newDay      models.DayOfWeek
	newStart    models.Period
	newRoom     uint
	applied     bool
}

var lastNeighbor neighborOp

// tryNeighbor attempts a random neighbor move and returns the new score.
func (ctx *schedulingContext) tryNeighbor(currentScore float64) float64 {
	if len(ctx.entries) == 0 {
		return currentScore
	}

	// Reset last operation
	lastNeighbor = neighborOp{}

	// Choose operation: 70% move, 30% swap
	if ctx.rng.Float64() < 0.7 || len(ctx.entries) < 2 {
		return ctx.tryMove(currentScore)
	}
	return ctx.trySwap(currentScore)
}

// tryMove moves one schedule entry to a new (day, period, room).
// Teacher and teaching task (class groups) remain fixed — only time and room change.
func (ctx *schedulingContext) tryMove(currentScore float64) float64 {
	idx := ctx.rng.Intn(len(ctx.entries))
	entry := ctx.entries[idx]

	// Save old state
	lastNeighbor.kind = "move"
	lastNeighbor.entryIdx = idx
	lastNeighbor.oldDay = entry.DayOfWeek
	lastNeighbor.oldStart = entry.StartPeriod
	lastNeighbor.oldRoom = entry.ClassroomID

	// Remove old occupancy
	ctx.removeOccupancy(entry)

	// Try new assignments
	validStarts := []int{0, 2, 4, 6, 8}
	// Pick day: prefer non-weekend if avoidance is on (80% chance weekdays)
	day := ctx.rng.Intn(7)
	if ctx.hasConstraint("avoid_saturday") || ctx.hasConstraint("avoid_sunday") {
		if ctx.rng.Float64() < 0.8 {
			day = ctx.rng.Intn(5) // weekday only
		}
	}
	// Pick start period, with bias for sports courses
	var start int
	if ctx.hasConstraint("pe_preferred_periods") && ctx.sportsCourseIDs[entry.CourseID] {
		// 70% chance to pick preferred periods (2 or 6), 30% any
		if ctx.rng.Float64() < 0.7 {
			prefer := []int{2, 6}
			start = prefer[ctx.rng.Intn(len(prefer))]
		} else {
			start = validStarts[ctx.rng.Intn(len(validStarts))]
		}
	} else {
		start = validStarts[ctx.rng.Intn(len(validStarts))]
	}
	span := entry.Span

	// Check locked slots
	for _, ls := range ctx.lockedSlots {
		if int(ls.DayOfWeek) == day && periodsOverlapInt(start, span, int(ls.StartPeriod), ls.Span) {
			ctx.restoreOccupancy(entry)
			return currentScore
		}
	}

	// Check teacher preferences
	if ctx.hasConstraint("teacher_preference") {
		for _, t := range ctx.teachers {
			if t.ID == entry.TeacherID {
				if t.PreferNoEarly && start <= 1 {
					ctx.restoreOccupancy(entry)
					return currentScore
				}
				if t.PreferNoLate && start >= 6 {
					ctx.restoreOccupancy(entry)
					return currentScore
				}
				break
			}
		}
	}

	// Check teacher busy at new position
	teacherBusy := false
	for p := start; p < start+span; p++ {
		key := fmt.Sprintf("%d-%d-%d", day, p, entry.TeacherID)
		if ctx.teacherOcc[key] {
			teacherBusy = true
			break
		}
	}
	if teacherBusy {
		ctx.restoreOccupancy(entry)
		return currentScore
	}

	// Check class groups busy (from teaching task)
	classBusy := ctx.classGroupsBusy(entry, day, start, span)
	if classBusy {
		ctx.restoreOccupancy(entry)
		return currentScore
	}

	// Try rooms
	rooms := make([]models.Classroom, len(ctx.classrooms))
	copy(rooms, ctx.classrooms)
	ctx.rng.Shuffle(len(rooms), func(i, j int) { rooms[i], rooms[j] = rooms[j], rooms[i] })

	for _, room := range rooms {
		roomBusy := false
		for p := start; p < start+span; p++ {
			key := fmt.Sprintf("%d-%d-%d", day, p, room.ID)
			if ctx.roomOcc[key] {
				roomBusy = true
				break
			}
		}
		if roomBusy {
			continue
		}

		// Apply move
		ctx.entries[idx].DayOfWeek = models.DayOfWeek(day)
		ctx.entries[idx].StartPeriod = models.Period(start)
		ctx.entries[idx].ClassroomID = room.ID

		lastNeighbor.newDay = models.DayOfWeek(day)
		lastNeighbor.newStart = models.Period(start)
		lastNeighbor.newRoom = room.ID
		lastNeighbor.applied = true

		ctx.addOccupancy(ctx.entries[idx])
		return ctx.computeScore()
	}

	// No valid move found, restore
	ctx.restoreOccupancy(entry)
	return currentScore
}

// trySwap swaps the time slots of two entries (keeping teaching task and room assignments).
func (ctx *schedulingContext) trySwap(currentScore float64) float64 {
	i1 := ctx.rng.Intn(len(ctx.entries))
	i2 := ctx.rng.Intn(len(ctx.entries))
	if i1 == i2 {
		return currentScore
	}

	e1, e2 := ctx.entries[i1], ctx.entries[i2]

	// Remove both from occupancy
	ctx.removeOccupancy(e1)
	ctx.removeOccupancy(e2)

	// Check e1 at e2's position
	conflict1 := ctx.checkPositionConflict(e1, int(e2.DayOfWeek), int(e2.StartPeriod))
	// Check e2 at e1's position
	conflict2 := ctx.checkPositionConflict(e2, int(e1.DayOfWeek), int(e1.StartPeriod))

	if conflict1 || conflict2 {
		ctx.addOccupancy(e1)
		ctx.addOccupancy(e2)
		return currentScore
	}

	// Save operation for undo
	lastNeighbor.kind = "swap"
	lastNeighbor.swapIdx1 = i1
	lastNeighbor.swapIdx2 = i2
	lastNeighbor.oldDay = e1.DayOfWeek
	lastNeighbor.oldStart = e1.StartPeriod
	lastNeighbor.oldRoom = e1.ClassroomID
	lastNeighbor.newDay = e2.DayOfWeek
	lastNeighbor.newStart = e2.StartPeriod
	lastNeighbor.newRoom = e2.ClassroomID

	// Swap day/period (keep teacher, room, teaching task)
	ctx.entries[i1].DayOfWeek, ctx.entries[i2].DayOfWeek = e2.DayOfWeek, e1.DayOfWeek
	ctx.entries[i1].StartPeriod, ctx.entries[i2].StartPeriod = e2.StartPeriod, e1.StartPeriod

	lastNeighbor.applied = true
	ctx.addOccupancy(ctx.entries[i1])
	ctx.addOccupancy(ctx.entries[i2])
	return ctx.computeScore()
}

// checkPositionConflict checks if an entry would conflict at a new (day, start) position.
func (ctx *schedulingContext) checkPositionConflict(e models.ScheduleEntry, day, start int) bool {
	span := e.Span

	// Check locked slots
	for _, ls := range ctx.lockedSlots {
		if int(ls.DayOfWeek) == day && periodsOverlapInt(start, span, int(ls.StartPeriod), ls.Span) {
			return true
		}
	}

	// Check teacher busy
	for p := start; p < start+span; p++ {
		key := fmt.Sprintf("%d-%d-%d", day, p, e.TeacherID)
		if ctx.teacherOcc[key] {
			return true
		}
	}

	// Check room busy
	for p := start; p < start+span; p++ {
		key := fmt.Sprintf("%d-%d-%d", day, p, e.ClassroomID)
		if ctx.roomOcc[key] {
			return true
		}
	}

	// Check class groups busy
	return ctx.classGroupsBusy(e, day, start, span)
}

// classGroupsBusy checks if any class group in the entry's teaching task is occupied.
func (ctx *schedulingContext) classGroupsBusy(e models.ScheduleEntry, day, start, span int) bool {
	if e.TeachingTaskID == nil {
		return false
	}
	// Find the teaching task data
	for _, td := range ctx.teachingTasks {
		if td.Task.ID == *e.TeachingTaskID {
			for _, cid := range td.ClassIDs {
				for p := start; p < start+span; p++ {
					key := fmt.Sprintf("%d-%d-%d", day, p, cid)
					if ctx.classOcc[key] {
						return true
					}
				}
			}
			return false
		}
	}
	return false
}

// undoNeighbor reverts the last neighbor operation.
func (ctx *schedulingContext) undoNeighbor() {
	if !lastNeighbor.applied {
		return
	}

	switch lastNeighbor.kind {
	case "move":
		idx := lastNeighbor.entryIdx
		ctx.removeOccupancy(ctx.entries[idx])
		ctx.entries[idx].DayOfWeek = lastNeighbor.oldDay
		ctx.entries[idx].StartPeriod = lastNeighbor.oldStart
		ctx.entries[idx].ClassroomID = lastNeighbor.oldRoom
		ctx.addOccupancy(ctx.entries[idx])

	case "swap":
		i1, i2 := lastNeighbor.swapIdx1, lastNeighbor.swapIdx2
		ctx.removeOccupancy(ctx.entries[i1])
		ctx.removeOccupancy(ctx.entries[i2])
		ctx.entries[i1].DayOfWeek = lastNeighbor.oldDay
		ctx.entries[i1].StartPeriod = lastNeighbor.oldStart
		ctx.entries[i1].ClassroomID = lastNeighbor.oldRoom
		ctx.entries[i2].DayOfWeek = lastNeighbor.newDay
		ctx.entries[i2].StartPeriod = lastNeighbor.newStart
		ctx.entries[i2].ClassroomID = lastNeighbor.newRoom
		ctx.addOccupancy(ctx.entries[i1])
		ctx.addOccupancy(ctx.entries[i2])
	}

	lastNeighbor.applied = false
}

// computeScore scores the current schedule using ScoringService.
func (ctx *schedulingContext) computeScore() float64 {
	if len(ctx.entries) == 0 {
		return 0
	}
	breakdown := (&ScoringService{}).ScoreSchedule(ctx.entries, ctx.teachers, ctx.classrooms, ctx.constraints, ctx.sportsCourseIDs)
	return breakdown.Total
}

// ---- Occupancy helpers ----

func (ctx *schedulingContext) removeOccupancy(e models.ScheduleEntry) {
	day, start, span := int(e.DayOfWeek), int(e.StartPeriod), e.Span
	for p := start; p < start+span; p++ {
		delete(ctx.roomOcc, fmt.Sprintf("%d-%d-%d", day, p, e.ClassroomID))
		delete(ctx.teacherOcc, fmt.Sprintf("%d-%d-%d", day, p, e.TeacherID))
	}
	// Remove class group occupancy for all classes in the teaching task
	if e.TeachingTaskID != nil {
		for _, td := range ctx.teachingTasks {
			if td.Task.ID == *e.TeachingTaskID {
				for _, cid := range td.ClassIDs {
					for p := start; p < start+span; p++ {
						delete(ctx.classOcc, fmt.Sprintf("%d-%d-%d", day, p, cid))
					}
				}
				break
			}
		}
	}
}

func (ctx *schedulingContext) addOccupancy(e models.ScheduleEntry) {
	day, start, span := int(e.DayOfWeek), int(e.StartPeriod), e.Span
	for p := start; p < start+span; p++ {
		ctx.roomOcc[fmt.Sprintf("%d-%d-%d", day, p, e.ClassroomID)] = true
		ctx.teacherOcc[fmt.Sprintf("%d-%d-%d", day, p, e.TeacherID)] = true
	}
	// Add class group occupancy for all classes in the teaching task
	if e.TeachingTaskID != nil {
		for _, td := range ctx.teachingTasks {
			if td.Task.ID == *e.TeachingTaskID {
				for _, cid := range td.ClassIDs {
					for p := start; p < start+span; p++ {
						ctx.classOcc[fmt.Sprintf("%d-%d-%d", day, p, cid)] = true
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
	lockedSlots []lockedTimeSlot,
	constraints []string,
	semester string,
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
			lockedSlots, constraints, semester, runConfig, cancelCh, nil)

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
func (s *SASolver) PostOptimize(
	entries []models.ScheduleEntry,
	teachingTasks []models.TeachingTask,
	teachers []models.Teacher,
	classrooms []models.Classroom,
	lockedSlots []lockedTimeSlot,
	constraints []string,
	topN int,
) []models.ScheduleEntry {
	if len(entries) == 0 || topN <= 0 {
		return entries
	}

	// Build teaching task lookup
	taskMap := make(map[uint]teachingTaskData)
	for _, tt := range teachingTasks {
		classIDs := make([]uint, len(tt.Classes))
		for j, c := range tt.Classes {
			classIDs[j] = c.ClassGroupID
		}
		taskMap[tt.ID] = teachingTaskData{Task: tt, ClassIDs: classIDs}
	}

	validStarts := []int{0, 2, 4, 6, 8}

	// Score each entry individually to find the worst ones
	type entryScore struct {
		idx   int
		entry models.ScheduleEntry
	}
	// Use a simple heuristic: entries on weekends or with early/late periods score lower
	var scored []entryScore
	for i, e := range entries {
		scored = append(scored, entryScore{idx: i, entry: e})
	}

	// Sort by a simple quality heuristic (weekend = worse, early/late = worse)
	// We'll just try all entries up to topN
	limit := topN
	if limit > len(entries) {
		limit = len(entries)
	}

	// Shuffle entries to avoid bias
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(entries), func(i, j int) { entries[i], entries[j] = entries[j], entries[i] })

	// Build current occupancy from all OTHER entries (excluding the one being optimized)
	buildOcc := func(excludeIdx int) (roomOcc, teacherOcc, classOcc map[string]bool) {
		roomOcc = make(map[string]bool)
		teacherOcc = make(map[string]bool)
		classOcc = make(map[string]bool)
		for i, e := range entries {
			if i == excludeIdx {
				continue
			}
			day, start, span := int(e.DayOfWeek), int(e.StartPeriod), e.Span
			for p := start; p < start+span; p++ {
				roomOcc[fmt.Sprintf("%d-%d-%d", day, p, e.ClassroomID)] = true
				teacherOcc[fmt.Sprintf("%d-%d-%d", day, p, e.TeacherID)] = true
			}
			if td, ok := taskMap[*e.TeachingTaskID]; ok {
				for _, cid := range td.ClassIDs {
					for p := start; p < start+span; p++ {
						classOcc[fmt.Sprintf("%d-%d-%d", day, p, cid)] = true
					}
				}
			}
		}
		return
	}

	improved := 0
	for _, es := range scored {
		if improved >= limit {
			break
		}

		e := es.entry
		eIdx := es.idx
		span := e.Span
		originalDay, originalStart, originalRoom := int(e.DayOfWeek), int(e.StartPeriod), e.ClassroomID

		roomOcc, teacherOcc, classOcc := buildOcc(eIdx)

		bestDay, bestStart, bestRoom := originalDay, originalStart, originalRoom
		bestPenalty := 999 // lower is better for this heuristic

		// Simple penalty: early/late/wekkend
		calcPenalty := func(day, start int) int {
			p := 0
			if day >= 5 {
				p += 3 // weekend penalty
			}
			if start <= 1 {
				p += 1 // early
			}
			if start >= 6 {
				p += 1 // late
			}
			return p
		}

		for _, day := range rng.Perm(7) {
			// Check locked slots
			lsBlocked := false
			for _, ls := range lockedSlots {
				if int(ls.DayOfWeek) == day && periodsOverlapInt(0, 11, int(ls.StartPeriod), ls.Span) {
					// Check if any start period would overlap
					for _, start := range validStarts {
						if periodsOverlapInt(start, span, int(ls.StartPeriod), ls.Span) {
							lsBlocked = true
							break
						}
					}
				}
			}
			if lsBlocked {
				continue
			}

			for _, start := range validStarts {
				penalty := calcPenalty(day, start)
				if penalty > bestPenalty {
					continue
				}

				// Check teacher busy
				teacherBusy := false
				for p := start; p < start+span; p++ {
					if teacherOcc[fmt.Sprintf("%d-%d-%d", day, p, e.TeacherID)] {
						teacherBusy = true
						break
					}
				}
				if teacherBusy {
					continue
				}

				// Check class groups busy
				classBusy := false
				if td, ok := taskMap[*e.TeachingTaskID]; ok {
					for _, cid := range td.ClassIDs {
						for p := start; p < start+span; p++ {
							if classOcc[fmt.Sprintf("%d-%d-%d", day, p, cid)] {
								classBusy = true
								break
							}
						}
						if classBusy {
							break
						}
					}
				}
				if classBusy {
					continue
				}

				// Try rooms
				for _, room := range classrooms {
					roomBusy := false
					for p := start; p < start+span; p++ {
						if roomOcc[fmt.Sprintf("%d-%d-%d", day, p, room.ID)] {
							roomBusy = true
							break
						}
					}
					if roomBusy {
						continue
					}

					if penalty < bestPenalty || (penalty == bestPenalty && day == originalDay && start == originalStart && room.ID == originalRoom) {
						bestDay, bestStart, bestRoom = day, start, room.ID
						bestPenalty = penalty
						if penalty == 0 {
							goto found
						}
					}
				}
			}
		}
	found:
		if bestDay != originalDay || bestStart != originalStart || bestRoom != originalRoom {
			e.DayOfWeek = models.DayOfWeek(bestDay)
			e.StartPeriod = models.Period(bestStart)
			e.ClassroomID = bestRoom
			entries[eIdx] = e
			improved++
		}
	}

	return entries
}

// ---- Helpers ----

func periodsOverlapInt(start1, span1, start2, span2 int) bool {
	end1 := start1 + span1
	end2 := start2 + span2
	return start1 < end2 && start2 < end1
}
