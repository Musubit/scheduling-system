package services

// v0.5.2 Goal 3 continued: cache initialization + apply/undo semantics.

import "scheduling-system/backend/models"

// newScoreCache creates an empty cache and precomputes constant lookups.
func newScoreCache(
	teachers []models.Teacher,
	classrooms []models.Classroom,
	tasks []teachingTaskData,
) *scoreCache {
	c := &scoreCache{
		teacherEarly:      make(map[uint]int),
		teacherLate:       make(map[uint]int),
		teacherTotal:      make(map[uint]int),
		teacherDayCount:   make(map[uint]*[7]int),
		teacherFloorSum:   make(map[uint]float64),
		teacherFloorCount: make(map[uint]int),
		courseDayCount:    make(map[uint]*[7]int),
		classDayBits:      make(map[uint]*[7]uint16),
		teacherByID:       make(map[uint]*models.Teacher, len(teachers)),
		classroomByID:     make(map[uint]*models.Classroom, len(classrooms)),
		taskClassIDs:      make(map[uint][]uint, len(tasks)),
	}
	for i := range teachers {
		c.teacherByID[teachers[i].ID] = &teachers[i]
	}
	c.maxFloor = 1
	for i := range classrooms {
		c.classroomByID[classrooms[i].ID] = &classrooms[i]
		if classrooms[i].Floor > c.maxFloor {
			c.maxFloor = classrooms[i].Floor
		}
	}
	for _, td := range tasks {
		if len(td.ClassIDs) > 0 {
			ids := make([]uint, len(td.ClassIDs))
			copy(ids, td.ClassIDs)
			c.taskClassIDs[td.Task.ID] = ids
		}
	}
	return c
}

// rebuildFromEntries wipes counters and re-applies every entry. Used at solver
// start after buildInitial() and as a sanity re-sync point.
func (c *scoreCache) rebuildFromEntries(entries []models.ScheduleEntry) {
	// Reset counters (keep constant lookups).
	c.teacherEarly = make(map[uint]int, len(c.teacherByID))
	c.teacherLate = make(map[uint]int, len(c.teacherByID))
	c.teacherTotal = make(map[uint]int, len(c.teacherByID))
	c.teacherDayCount = make(map[uint]*[7]int, len(c.teacherByID))
	c.teacherFloorSum = make(map[uint]float64)
	c.teacherFloorCount = make(map[uint]int)
	c.courseDayCount = make(map[uint]*[7]int)
	c.classDayBits = make(map[uint]*[7]uint16)
	c.weekendSat = 0
	c.weekendSun = 0
	c.totalEntries = 0
	c.peTotal = 0
	c.peAtPref = 0
	for _, e := range entries {
		c.applyEntry(+1, e, false)
	}
}

// applyEntry updates all counters by the given sign (+1 add / -1 remove).
// isSports indicates whether the entry's course is a sports course; caller
// looks it up once from sportsCourseIDs.
func (c *scoreCache) applyEntry(sign int, e models.ScheduleEntry, isSports bool) {
	day := int(e.DayOfWeek)
	start := int(e.StartPeriod)
	span := e.Span
	if span < 1 {
		span = 2
	}

	// --- teacher pref / total / day-count ---
	c.teacherTotal[e.TeacherID] += sign
	t := c.teacherByID[e.TeacherID]
	if t != nil {
		if t.PreferNoEarly && start <= 1 {
			c.teacherEarly[e.TeacherID] += sign
		}
		if t.PreferNoLate && start >= 6 {
			c.teacherLate[e.TeacherID] += sign
		}
		// Low-floor stat: only accumulate for teachers who set PreferLowFloor.
		if t.PreferLowFloor {
			if room := c.classroomByID[e.ClassroomID]; room != nil {
				c.teacherFloorSum[e.TeacherID] += float64(sign) * float64(room.Floor)
				c.teacherFloorCount[e.TeacherID] += sign
			}
		}
	}
	if _, ok := c.teacherDayCount[e.TeacherID]; !ok {
		c.teacherDayCount[e.TeacherID] = &[7]int{}
	}
	c.teacherDayCount[e.TeacherID][day] += sign

	// --- course-day count ---
	if _, ok := c.courseDayCount[e.CourseID]; !ok {
		c.courseDayCount[e.CourseID] = &[7]int{}
	}
	c.courseDayCount[e.CourseID][day] += sign

	// --- weekend / total ---
	c.totalEntries += sign
	if day == int(models.Sat) {
		c.weekendSat += sign
	}
	if day == int(models.Sun) {
		c.weekendSun += sign
	}

	// --- PE preference ---
	if isSports {
		c.peTotal += sign
		if start == 2 || start == 6 {
			c.peAtPref += sign
		}
	}

	// --- student fatigue bitmap (per class group × day) ---
	var cgIDs []uint
	if e.TeachingTaskID != nil {
		cgIDs = c.taskClassIDs[*e.TeachingTaskID]
	} else if e.ClassGroupID != nil {
		cgIDs = []uint{*e.ClassGroupID}
	}
	for _, cgID := range cgIDs {
		if _, ok := c.classDayBits[cgID]; !ok {
			c.classDayBits[cgID] = &[7]uint16{}
		}
		bits := &c.classDayBits[cgID][day]
		for p := start; p < start+span; p++ {
			if p < 0 || p > 10 {
				continue
			}
			mask := uint16(1) << uint(p)
			if sign > 0 {
				*bits |= mask
			} else {
				*bits &^= mask
			}
		}
	}
}
