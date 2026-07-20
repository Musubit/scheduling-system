package time

import schedtypes "scheduling-system/backend/scheduling/types"

// scoreCache 维护三个时间维度评分所需的增量统计。
// 每次 applyDelta 在 O(1) 更新后，scoreDetail 可在 O(#teachers + #courses + #classGroups) 重建评分。
type scoreCache struct {
	// time 维度: teacher 早晚偏好
	teacherEarly map[uint]int
	teacherLate  map[uint]int
	teacherTotal map[uint]int

	// time 维度: course-day 分散度
	courseDayCount map[uint]*[7]int

	// time 维度: 周末
	weekendSat  int
	weekendSun  int
	totalEntries int

	// time 维度: PE 偏好时段
	peTotal  int
	peAtPref int

	// teacher 维度: 每日上课节数 + 跨天数
	teacherDayCount map[uint]*[7]int

	// student 维度: classGroup × day 占用位图
	classDayBits map[uint]*[7]uint16

	// 常量查找
	teacherByID map[uint]*schedtypes.TeacherView
	taskByID   map[uint]*taskInfo
}

// newScoreCache 创建空缓存并预填充查找表。
func newScoreCache(
	teacherByID map[uint]*schedtypes.TeacherView,
	taskByID map[uint]*taskInfo,
) *scoreCache {
	c := &scoreCache{
		teacherEarly:    make(map[uint]int),
		teacherLate:     make(map[uint]int),
		teacherTotal:    make(map[uint]int),
		courseDayCount:  make(map[uint]*[7]int),
		teacherDayCount: make(map[uint]*[7]int),
		classDayBits:    make(map[uint]*[7]uint16),
		teacherByID:     teacherByID,
		taskByID:        taskByID,
	}
	return c
}

// rebuildFromEntries 清空计数器并从 entries 重建。
func (c *scoreCache) rebuildFromEntries(entries []timeEntry, _ map[uint]*taskInfo, sportsCourseIDs map[uint]bool) {
	c.teacherEarly = make(map[uint]int, len(c.teacherByID))
	c.teacherLate = make(map[uint]int, len(c.teacherByID))
	c.teacherTotal = make(map[uint]int, len(c.teacherByID))
	c.courseDayCount = make(map[uint]*[7]int)
	c.teacherDayCount = make(map[uint]*[7]int)
	c.classDayBits = make(map[uint]*[7]uint16)
	c.weekendSat = 0
	c.weekendSun = 0
	c.totalEntries = 0
	c.peTotal = 0
	c.peAtPref = 0

	for _, e := range entries {
		isSports := sportsCourseIDs[e.CourseID]
		c.applyDelta(+1, e, isSports)
	}
}

// applyDelta 以 sign (+1/-1) 更新所有计数器。
func (c *scoreCache) applyDelta(sign int, e timeEntry, isSports bool) {
	day := e.DayOfWeek
	start := e.StartPeriod
	span := e.Span
	if span < 1 {
		span = 2
	}

	// --- teacher pref / total / day-count ---
	c.teacherTotal[e.TeacherID] += sign
	tv := c.teacherByID[e.TeacherID]
	if tv != nil {
		if tv.PreferNoEarly && start <= 1 {
			c.teacherEarly[e.TeacherID] += sign
		}
		if tv.PreferNoLate && start >= 6 {
			c.teacherLate[e.TeacherID] += sign
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
	if day == 5 { // Saturday
		c.weekendSat += sign
	}
	if day == 6 { // Sunday
		c.weekendSun += sign
	}

	// --- PE preference ---
	if isSports {
		c.peTotal += sign
		if start == 2 || start == 6 {
			c.peAtPref += sign
		}
	}

	// --- student fatigue bitmap ---
	cgIDs := e.ClassGroupIDs
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
