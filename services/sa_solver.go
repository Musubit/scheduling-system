package services

import (
	"fmt"
	"math"
	"math/rand"
	"scheduling-system/models"
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

// schedulingContext holds all the data needed during solving.
type schedulingContext struct {
	courses         []models.Course
	teachers        []models.Teacher
	classrooms      []models.Classroom
	classGroups     []models.ClassGroup
	lockedSlots     []lockedTimeSlot
	constraints     []string
	semester        string
	courseClassGroups map[uint][]models.ClassGroup

	// Mutable state
	entries       []models.ScheduleEntry
	roomOcc       map[string]bool // "day-period-roomID"
	teacherOcc    map[string]bool
	classOcc      map[string]bool

	rng *rand.Rand
}

// Solve runs simulated annealing and returns the best schedule found.
// cancelCh can be used to interrupt the solver early (nil = no interrupt).
// progressFn is called periodically with (iteration, currentScore, bestScore, temperature).
func (s *SASolver) Solve(
	courses []models.Course,
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

	// Build context
	ctx := &schedulingContext{
		courses:         courses,
		teachers:        teachers,
		classrooms:      classrooms,
		classGroups:     classGroups,
		lockedSlots:     lockedSlots,
		constraints:     constraints,
		semester:        semester,
		rng:             rng,
	}

	// Match class groups to courses by department
	ctx.courseClassGroups = make(map[uint][]models.ClassGroup)
	for _, course := range courses {
		targetDept := deptMap[course.Dept]
		var matched []models.ClassGroup
		for _, cg := range classGroups {
			if cg.Dept == targetDept {
				matched = append(matched, cg)
			}
		}
		ctx.courseClassGroups[course.ID] = matched
	}

	// Phase 1: Generate initial solution with greedy construction
	ctx.buildInitial()

	// Score initial solution
	currentScore := ctx.computeScore()
	bestEntries := make([]models.ScheduleEntry, len(ctx.entries))
	copy(bestEntries, ctx.entries)
	bestScore := currentScore

	scheduled := len(ctx.entries)
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
		Scheduled:  scheduled,
		Iterations: totalIterations,
		ElapsedMs:  elapsed,
	}
}

// buildInitial constructs a greedy initial solution.
func (ctx *schedulingContext) buildInitial() {
	ctx.entries = nil
	ctx.roomOcc = make(map[string]bool)
	ctx.teacherOcc = make(map[string]bool)
	ctx.classOcc = make(map[string]bool)

	validStarts := []int{0, 2, 4, 6, 8} // 第1/3/5/7/9节

	for _, course := range ctx.courses {
		placed := false
		// Try days in random order
		days := ctx.rng.Perm(7)

		for _, day := range days {
			if placed {
				break
			}
			// Check if day is completely locked
			dayBlocked := false
			for _, ls := range ctx.lockedSlots {
				if int(ls.DayOfWeek) == day {
					dayBlocked = true
					break
				}
			}
			_ = dayBlocked

			starts := make([]int, len(validStarts))
			copy(starts, validStarts)
			ctx.rng.Shuffle(len(starts), func(i, j int) { starts[i], starts[j] = starts[j], starts[i] })

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

				// Find matching teachers (same department)
				candidates := findTeachersForCourse(course, ctx.teachers)
				ctx.rng.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })

				for _, teacher := range candidates {
					if placed {
						break
					}
					// Check teacher constraints
					if ctx.hasConstraint("teacher_preference") {
						if teacher.PreferNoEarly && start <= 1 {
							continue
						}
						if teacher.PreferNoLate && start >= 6 {
							continue
						}
					}
					// Check teacher busy
					teacherBusy := false
					for p := start; p < start+span; p++ {
						key := fmt.Sprintf("%d-%d-%d", day, p, teacher.ID)
						if ctx.teacherOcc[key] {
							teacherBusy = true
							break
						}
					}
					if teacherBusy {
						continue
					}

					// Try rooms
					rooms := ctx.classrooms
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

						// Pick a class group
						cgs := ctx.courseClassGroups[course.ID]
						var classGroupID *uint
						for _, cg := range cgs {
							cgBusy := false
							for p := start; p < start+span; p++ {
								key := fmt.Sprintf("%d-%d-%d", day, p, cg.ID)
								if ctx.classOcc[key] {
									cgBusy = true
									break
								}
							}
							if !cgBusy {
								cgid := cg.ID
								classGroupID = &cgid
								break
							}
						}

						entry := models.ScheduleEntry{
							CourseID:     course.ID,
							TeacherID:    teacher.ID,
							ClassroomID:  room.ID,
							ClassGroupID: classGroupID,
							Semester:     ctx.semester,
							DayOfWeek:    models.DayOfWeek(day),
							StartPeriod:  models.Period(start),
							Span:         span,
							Weeks:        "1-16",
						}
						ctx.entries = append(ctx.entries, entry)

						for p := start; p < start+span; p++ {
							ctx.roomOcc[fmt.Sprintf("%d-%d-%d", day, p, room.ID)] = true
							ctx.teacherOcc[fmt.Sprintf("%d-%d-%d", day, p, teacher.ID)] = true
							if classGroupID != nil {
								ctx.classOcc[fmt.Sprintf("%d-%d-%d", day, p, *classGroupID)] = true
							}
						}
						placed = true
						break
					}
				}
			}
		}
		// If not placed, skip this course (will be noted in result)
		_ = placed
	}
}

// Neighbor operation types
type neighborOp struct {
	kind         string // "move" or "swap"
	entryIdx     int    // index in ctx.entries for "move"
	swapIdx1     int    // first index for "swap"
	swapIdx2     int    // second index for "swap"
	oldDay       models.DayOfWeek
	oldStart     models.Period
	oldTeacher   uint
	oldRoom      uint
	oldClassGrp  *uint
	newDay       models.DayOfWeek
	newStart     models.Period
	newTeacher   uint
	newRoom      uint
	newClassGrp  *uint
	applied      bool
}

var lastNeighbor neighborOp

// tryNeighbor attempts a random neighbor move and returns the new score.
// The caller must call undoNeighbor if the move is rejected.
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

// tryMove moves one schedule entry to a new (day, period, teacher, room).
func (ctx *schedulingContext) tryMove(currentScore float64) float64 {
	idx := ctx.rng.Intn(len(ctx.entries))
	entry := ctx.entries[idx]

	// Save old state
	lastNeighbor.kind = "move"
	lastNeighbor.entryIdx = idx
	lastNeighbor.oldDay = entry.DayOfWeek
	lastNeighbor.oldStart = entry.StartPeriod
	lastNeighbor.oldTeacher = entry.TeacherID
	lastNeighbor.oldRoom = entry.ClassroomID
	lastNeighbor.oldClassGrp = entry.ClassGroupID

	// Remove old occupancy
	ctx.removeOccupancy(entry)

	// Try new assignments
	validStarts := []int{0, 2, 4, 6, 8}
	day := ctx.rng.Intn(7)
	start := validStarts[ctx.rng.Intn(len(validStarts))]
	span := entry.Span

	// Check locked slots
	for _, ls := range ctx.lockedSlots {
		if int(ls.DayOfWeek) == day && periodsOverlapInt(start, span, int(ls.StartPeriod), ls.Span) {
			// Restore old occupancy and return
			ctx.restoreOccupancy(entry)
			return currentScore
		}
	}

	// Try to find a free teacher
	candidates := findTeachersForEntry(entry, ctx.teachers, ctx.courseClassGroups)
	ctx.rng.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })

	for _, teacher := range candidates {
		if ctx.hasConstraint("teacher_preference") {
			if teacher.PreferNoEarly && start <= 1 {
				continue
			}
			if teacher.PreferNoLate && start >= 6 {
				continue
			}
		}

		teacherBusy := false
		for p := start; p < start+span; p++ {
			key := fmt.Sprintf("%d-%d-%d", day, p, teacher.ID)
			if ctx.teacherOcc[key] {
				teacherBusy = true
				break
			}
		}
		if teacherBusy {
			continue
		}

		// Try rooms
		rooms := ctx.classrooms
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

			// Pick class group
			cgs := ctx.courseClassGroups[entry.CourseID]
			var classGroupID *uint
			for _, cg := range cgs {
				cgBusy := false
				for p := start; p < start+span; p++ {
					key := fmt.Sprintf("%d-%d-%d", day, p, cg.ID)
					if ctx.classOcc[key] {
						cgBusy = true
						break
					}
				}
				if !cgBusy {
					cgid := cg.ID
					classGroupID = &cgid
					break
				}
			}

			// Apply move
			ctx.entries[idx].DayOfWeek = models.DayOfWeek(day)
			ctx.entries[idx].StartPeriod = models.Period(start)
			ctx.entries[idx].TeacherID = teacher.ID
			ctx.entries[idx].ClassroomID = room.ID
			ctx.entries[idx].ClassGroupID = classGroupID

			lastNeighbor.newDay = models.DayOfWeek(day)
			lastNeighbor.newStart = models.Period(start)
			lastNeighbor.newTeacher = teacher.ID
			lastNeighbor.newRoom = room.ID
			lastNeighbor.newClassGrp = classGroupID
			lastNeighbor.applied = true

			ctx.addOccupancy(ctx.entries[idx])
			return ctx.computeScore()
		}
	}

	// No valid move found, restore
	ctx.restoreOccupancy(entry)
	return currentScore
}

// trySwap swaps the time slots of two entries (keeping teacher/room assignments).
func (ctx *schedulingContext) trySwap(currentScore float64) float64 {
	i1 := ctx.rng.Intn(len(ctx.entries))
	i2 := ctx.rng.Intn(len(ctx.entries))
	if i1 == i2 {
		return currentScore
	}

	e1, e2 := ctx.entries[i1], ctx.entries[i2]

	// Check if the swap would cause conflicts at the new positions
	// e1 moving to e2's slot, e2 moving to e1's slot

	// Remove both from occupancy
	ctx.removeOccupancy(e1)
	ctx.removeOccupancy(e2)

	// Check e1 at e2's position
	conflict1 := ctx.hasConflict(e1.CourseID, e1.TeacherID, e1.ClassroomID, int(e2.DayOfWeek), int(e2.StartPeriod), e1.Span, e1.ClassGroupID)
	// Check e2 at e1's position
	conflict2 := ctx.hasConflict(e2.CourseID, e2.TeacherID, e2.ClassroomID, int(e1.DayOfWeek), int(e1.StartPeriod), e2.Span, e2.ClassGroupID)

	if conflict1 || conflict2 {
		// Restore
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
	lastNeighbor.oldTeacher = e1.TeacherID
	lastNeighbor.oldRoom = e1.ClassroomID
	lastNeighbor.oldClassGrp = e1.ClassGroupID
	lastNeighbor.newDay = e2.DayOfWeek
	lastNeighbor.newStart = e2.StartPeriod
	lastNeighbor.newTeacher = e2.TeacherID
	lastNeighbor.newRoom = e2.ClassroomID
	lastNeighbor.newClassGrp = e2.ClassGroupID

	// Swap day/period
	ctx.entries[i1].DayOfWeek, ctx.entries[i2].DayOfWeek = e2.DayOfWeek, e1.DayOfWeek
	ctx.entries[i1].StartPeriod, ctx.entries[i2].StartPeriod = e2.StartPeriod, e1.StartPeriod

	lastNeighbor.applied = true
	ctx.addOccupancy(ctx.entries[i1])
	ctx.addOccupancy(ctx.entries[i2])
	return ctx.computeScore()
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
		ctx.entries[idx].TeacherID = lastNeighbor.oldTeacher
		ctx.entries[idx].ClassroomID = lastNeighbor.oldRoom
		ctx.entries[idx].ClassGroupID = lastNeighbor.oldClassGrp
		ctx.addOccupancy(ctx.entries[idx])

	case "swap":
		i1, i2 := lastNeighbor.swapIdx1, lastNeighbor.swapIdx2
		ctx.removeOccupancy(ctx.entries[i1])
		ctx.removeOccupancy(ctx.entries[i2])
		ctx.entries[i1].DayOfWeek = lastNeighbor.oldDay
		ctx.entries[i1].StartPeriod = lastNeighbor.oldStart
		ctx.entries[i2].DayOfWeek = lastNeighbor.newDay
		ctx.entries[i2].StartPeriod = lastNeighbor.newStart
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
	breakdown := (&ScoringService{}).ScoreSchedule(ctx.entries, ctx.teachers, ctx.classrooms, ctx.constraints)
	return breakdown.Total
}

// ---- Occupancy helpers ----

func (ctx *schedulingContext) removeOccupancy(e models.ScheduleEntry) {
	day, start, span := int(e.DayOfWeek), int(e.StartPeriod), e.Span
	for p := start; p < start+span; p++ {
		delete(ctx.roomOcc, fmt.Sprintf("%d-%d-%d", day, p, e.ClassroomID))
		delete(ctx.teacherOcc, fmt.Sprintf("%d-%d-%d", day, p, e.TeacherID))
		if e.ClassGroupID != nil {
			delete(ctx.classOcc, fmt.Sprintf("%d-%d-%d", day, p, *e.ClassGroupID))
		}
	}
}

func (ctx *schedulingContext) addOccupancy(e models.ScheduleEntry) {
	day, start, span := int(e.DayOfWeek), int(e.StartPeriod), e.Span
	for p := start; p < start+span; p++ {
		ctx.roomOcc[fmt.Sprintf("%d-%d-%d", day, p, e.ClassroomID)] = true
		ctx.teacherOcc[fmt.Sprintf("%d-%d-%d", day, p, e.TeacherID)] = true
		if e.ClassGroupID != nil {
			ctx.classOcc[fmt.Sprintf("%d-%d-%d", day, p, *e.ClassGroupID)] = true
		}
	}
}

func (ctx *schedulingContext) restoreOccupancy(e models.ScheduleEntry) {
	ctx.addOccupancy(e)
}

func (ctx *schedulingContext) hasConflict(courseID, teacherID, roomID uint, day, start, span int, classGroupID *uint) bool {
	// Check locked slots
	for _, ls := range ctx.lockedSlots {
		if int(ls.DayOfWeek) == day && periodsOverlapInt(start, span, int(ls.StartPeriod), ls.Span) {
			return true
		}
	}
	// Check teacher
	for p := start; p < start+span; p++ {
		key := fmt.Sprintf("%d-%d-%d", day, p, teacherID)
		if ctx.teacherOcc[key] {
			return true
		}
	}
	// Check room
	for p := start; p < start+span; p++ {
		key := fmt.Sprintf("%d-%d-%d", day, p, roomID)
		if ctx.roomOcc[key] {
			return true
		}
	}
	// Check class group
	if classGroupID != nil {
		for p := start; p < start+span; p++ {
			key := fmt.Sprintf("%d-%d-%d", day, p, *classGroupID)
			if ctx.classOcc[key] {
				return true
			}
		}
	}
	return false
}

func (ctx *schedulingContext) hasConstraint(key string) bool {
	for _, c := range ctx.constraints {
		if c == key {
			return true
		}
	}
	return false
}

// ---- Helpers ----

func findTeachersForCourse(course models.Course, teachers []models.Teacher) []models.Teacher {
	targetDept := deptMap[course.Dept]
	var same, other []models.Teacher
	for _, t := range teachers {
		if t.Dept == targetDept {
			same = append(same, t)
		} else {
			other = append(other, t)
		}
	}
	result := append(same, other...)
	return result
}

func findTeachersForEntry(entry models.ScheduleEntry, teachers []models.Teacher, courseClassGroups map[uint][]models.ClassGroup) []models.Teacher {
	// Find the course to determine department
	// We don't have courses directly, but we match by teacher department via the class groups
	// Fall back to all teachers
	return teachers
}

func periodsOverlapInt(start1, span1, start2, span2 int) bool {
	end1 := start1 + span1
	end2 := start2 + span2
	return start1 < end2 && start2 < end1
}
