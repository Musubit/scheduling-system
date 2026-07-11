package services

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"scheduling-system/backend/models"
	"strings"
	"time"
)

func (s *SASolver) PostOptimize(
	entries []models.ScheduleEntry,
	teachingTasks []models.TeachingTask,
	teachers []models.Teacher,
	classrooms []models.Classroom,
	lockedSlots []LockedTimeSlot,
	constraints []string,
	topN int,
) []models.ScheduleEntry {
	if len(entries) == 0 || topN <= 0 {
		return entries
	}

	// Build per-class-group student map
	classGroupStudents := make(map[uint]int)
	for _, tt := range teachingTasks {
		for _, c := range tt.Classes {
			if c.ClassGroup.ID > 0 {
				classGroupStudents[c.ClassGroupID] = c.ClassGroup.Students
			}
		}
	}

	// Build teaching task lookup with total students
	taskMap := make(map[uint]teachingTaskData)
	for _, tt := range teachingTasks {
		classIDs := make([]uint, len(tt.Classes))
		totalStudents := 0
		for j, c := range tt.Classes {
			classIDs[j] = c.ClassGroupID
			totalStudents += classGroupStudents[c.ClassGroupID]
		}
		taskMap[tt.ID] = teachingTaskData{
			Task:          tt,
			ClassIDs:      classIDs,
			TotalStudents: totalStudents,
		}
	}

	// Build teacher unavailable map
	teacherUnavailable := make(map[uint][]LockedTimeSlot)
	for _, t := range teachers {
		if t.UnavailableSlots != "" {
			var slots []LockedTimeSlot
			if err := json.Unmarshal([]byte(t.UnavailableSlots), &slots); err == nil && len(slots) > 0 {
				teacherUnavailable[t.ID] = slots
			}
		}
	}

	// Helper: check teacher unavailable
	isTeacherUnavailable := func(teacherID uint, day, start, span int) bool {
		slots, ok := teacherUnavailable[teacherID]
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

	// Helper: check room capacity (sports venues have unlimited capacity)
	canRoomFit := func(room models.Classroom, td teachingTaskData) bool {
		if room.Type == "体育馆" {
			return true
		}
		return td.TotalStudents <= 0 || room.Capacity >= td.TotalStudents
	}

	validStarts := []int{0, 2, 4, 6, 8}

	// Score each entry individually to find the worst ones
	type entryScore struct {
		idx   int
		entry models.ScheduleEntry
	}
	var scored []entryScore
	for i, e := range entries {
		scored = append(scored, entryScore{idx: i, entry: e})
	}

	limit := topN
	if limit > len(entries) {
		limit = len(entries)
	}

	// Shuffle scored to avoid bias
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(scored), func(i, j int) { scored[i], scored[j] = scored[j], scored[i] })

	// Build a quick lookup for shared rooms (体育馆 have unlimited capacity and can be shared)
	sharedRoom := make(map[uint]bool)
	for _, room := range classrooms {
		if room.Type == "体育馆" {
			sharedRoom[room.ID] = true
		}
	}

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
			if !sharedRoom[e.ClassroomID] {
				for p := start; p < start+span; p++ {
					roomOcc[fmt.Sprintf("%d-%d-%d", day, p, e.ClassroomID)] = true
				}
			}
			for p := start; p < start+span; p++ {
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
		bestPenalty := 999

		// Simple penalty: early/late/weekend
		calcPenalty := func(day, start int) int {
			p := 0
			if day >= 5 {
				p += 3
			}
			if start <= 1 {
				p += 1
			}
			if start >= 6 {
				p += 1
			}
			return p
		}

		for _, day := range rng.Perm(7) {
			// Check locked slots (at day level)
			lsBlocked := false
			for _, ls := range lockedSlots {
				if int(ls.DayOfWeek) == day && periodsOverlapInt(0, 11, int(ls.StartPeriod), ls.Span) {
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
				// Check teacher unavailable (at specific day+start)
				if isTeacherUnavailable(e.TeacherID, day, start, span) {
					continue
				}

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
				td, tdOk := taskMap[*e.TeachingTaskID]
				// Determine required room type (same logic as sa_initial.go)
				requiredRoomType := ""
				if tdOk {
					requiredRoomType = roomTypeForCourse(td.Task.Course.Name)
				}
				for _, room := range classrooms {
					// Check room type
					if requiredRoomType != "" && room.Type != requiredRoomType {
						continue
					}
					// Check room capacity
					if tdOk && !canRoomFit(room, td) {
						continue
					}

				roomBusy := false
				if room.Type != "体育馆" {
					for p := start; p < start+span; p++ {
						if roomOcc[fmt.Sprintf("%d-%d-%d", day, p, room.ID)] {
							roomBusy = true
							break
						}
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

// roomTypeForCourse determines the required room type from course name.
// Same logic as schedulingContext.getRequiredRoomType in sa_solver.go.
func roomTypeForCourse(courseName string) string {
	if models.IsSportsCourse(courseName) {
		return "体育馆"
	}
	if strings.Contains(courseName, "实验") {
		return "实验室"
	}
	if strings.Contains(courseName, "上机") {
		return "机房"
	}
	return ""
}
