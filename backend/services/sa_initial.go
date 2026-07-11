package services

import (
	"fmt"
	"scheduling-system/backend/models"
	"strings"
)

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

		// Calculate sessions per week based on task's actual week span
		sessionsPerWeek := 1
		if td.CourseHours > 0 {
			weeks := td.Task.EndWeek - td.Task.StartWeek + 1
			if weeks < 1 {
				weeks = 1
			}
			periodsPerWeek := weeks * 2 // 2 periods per day
			sessionsPerWeek = (td.CourseHours + periodsPerWeek - 1) / periodsPerWeek
			if sessionsPerWeek < 1 {
				sessionsPerWeek = 1
			}
			if sessionsPerWeek > 4 {
				sessionsPerWeek = 4
			}
		}

		// Determine required room type for this task
		requiredRoomType := ""
		courseName := td.Task.Course.Name
		if models.IsSportsCourse(courseName) {
			requiredRoomType = "体育馆"
		} else if strings.Contains(courseName, "实验") {
			requiredRoomType = "实验室"
		} else if strings.Contains(courseName, "上机") {
			requiredRoomType = "机房"
		}

		for s := 0; s < sessionsPerWeek; s++ {
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

			// Phase 1: Try with teacher preference enforced (soft — skip conflicting starts)
			placed = ctx.tryPlaceSession(td, days, validStarts, requiredRoomType, true)

			// Phase 2: Relax teacher preference if not placed
			if !placed && ctx.hasConstraint("teacher_preference") {
				placed = ctx.tryPlaceSession(td, days, validStarts, requiredRoomType, false)
			}

			_ = placed
		} // end sessionsPerWeek loop
	}
}

// tryPlaceSession attempts to place one session of a teaching task.
// If enforcePref is true, teacher preference conflicts cause the start to be skipped.
// If false, teacher preferences are ignored (relaxed phase).
func (ctx *schedulingContext) tryPlaceSession(td teachingTaskData, days []int, starts []int, requiredRoomType string, enforcePref bool) bool {
	for _, day := range days {
		// Shuffle starts for randomness, but sports courses prefer specific starts
		var orderedStarts []int
		if ctx.hasConstraint("pe_preferred_periods") && ctx.sportsCourseIDs[td.Task.CourseID] {
			prefer := []int{2, 6}
			other := []int{0, 4, 8}
			ctx.rng.Shuffle(len(prefer), func(i, j int) { prefer[i], prefer[j] = prefer[j], prefer[i] })
			ctx.rng.Shuffle(len(other), func(i, j int) { other[i], other[j] = other[j], other[i] })
			orderedStarts = append(prefer, other...)
		} else {
			orderedStarts = make([]int, len(starts))
			copy(orderedStarts, starts)
			ctx.rng.Shuffle(len(orderedStarts), func(i, j int) { orderedStarts[i], orderedStarts[j] = orderedStarts[j], orderedStarts[i] })
		}

		for _, start := range orderedStarts {
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

			// Check teacher unavailable slots (always hard)
			if ctx.isTeacherUnavailable(td.Task.TeacherID, day, start, span) {
				continue
			}

			// Check teacher preferences (soft in relaxed mode)
			if enforcePref && ctx.hasConstraint("teacher_preference") {
				prefConflict := false
				for _, t := range ctx.teachers {
					if t.ID == td.Task.TeacherID {
						if t.PreferNoEarly && start <= 1 {
							prefConflict = true
						}
						if t.PreferNoLate && start >= 6 {
							prefConflict = true
						}
						break
					}
				}
				if prefConflict {
					continue
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

			// Check not overlapping with already-placed sessions of same task
			taskSelfBusy := false
			for _, existing := range ctx.entries {
				if existing.TeachingTaskID != nil && *existing.TeachingTaskID == td.Task.ID {
					if int(existing.DayOfWeek) == day && periodsOverlapInt(start, span, int(existing.StartPeriod), existing.Span) {
						taskSelfBusy = true
						break
					}
				}
			}
			if taskSelfBusy {
				continue
			}

			// Check all class groups are free
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

			// Try rooms (with room type filtering)
			rooms := make([]models.Classroom, len(ctx.classrooms))
			copy(rooms, ctx.classrooms)
			ctx.rng.Shuffle(len(rooms), func(i, j int) { rooms[i], rooms[j] = rooms[j], rooms[i] })

			for _, room := range rooms {
			// Check room type
			if requiredRoomType != "" {
				if room.Type != requiredRoomType {
					continue
				}
			} else if room.Type == "体育馆" || room.Type == "实验室" || room.Type == "机房" {
				continue // regular courses cannot use specialty rooms
			}
				// Check room capacity
				if !ctx.canRoomFitCapacity(room, &td) {
					continue
				}

			// Check room conflict (skip for shared venues like 体育馆)
			if room.Type != "体育馆" {
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
			}

				// All constraints satisfied, create entry
				entry := models.ScheduleEntry{
					CourseID:       td.Task.CourseID,
					TeacherID:      td.Task.TeacherID,
					ClassroomID:    room.ID,
					TeachingTaskID: &td.Task.ID,
					ClassGroupID:   nil,
					Semester:       ctx.semester,
					DayOfWeek:      models.DayOfWeek(day),
					StartPeriod:    models.Period(start),
					Span:           span,
					Weeks:          fmt.Sprintf("%d-%d", td.Task.StartWeek, td.Task.EndWeek),
				}
				ctx.entries = append(ctx.entries, entry)

		// Occupy room (skip for shared venues), teacher, and all class groups
		for p := start; p < start+span; p++ {
			if room.Type != "体育馆" {
				ctx.roomOcc[fmt.Sprintf("%d-%d-%d", day, p, room.ID)] = true
			}
			ctx.teacherOcc[fmt.Sprintf("%d-%d-%d", day, p, td.Task.TeacherID)] = true
				}
				for _, cid := range td.ClassIDs {
					for p := start; p < start+span; p++ {
						ctx.classOcc[fmt.Sprintf("%d-%d-%d", day, p, cid)] = true
					}
				}
				return true
			}
		}
	}
	return false
}

// Neighbor operation types
