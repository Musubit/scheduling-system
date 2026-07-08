package services

import (
	"fmt"
	"scheduling-system/models"
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
