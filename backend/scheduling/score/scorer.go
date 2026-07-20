// Package score 提供 4-bucket 排课评分实现。
//
// Scorer 实现 types.IScorer，是排课结果的最终评分权威。
// 四个维度按 dims 顺序计算: time, teacher, student, resource。
// TIME_ONLY 模式下 resource 维度被禁用（dims 不含 "resource"）。
//
// 每个 bucket 的 Value 约束在 [0, perBucketMax] 内，perBucketMax = 100 / len(dims)。
// 子分数使用 perBucketMax 而非硬编码 100，保证 Value ≤ Max 一致。
package score

import (
	"math"
	"sort"

	schedtypes "scheduling-system/backend/scheduling/types"
)

// Scorer 是 4-bucket 评分器的实现。
type Scorer struct{}

// NewScorer 创建一个 Scorer。
func NewScorer() *Scorer { return &Scorer{} }

// 编译期接口检查。
var _ schedtypes.IScorer = (*Scorer)(nil)

// Score 实现 types.IScorer。
func (sc *Scorer) Score(
	assignments []schedtypes.TimeAssignmentDraft,
	allocations []schedtypes.RoomAllocationDraft,
	teachers []schedtypes.TeacherView,
	classrooms []schedtypes.ClassroomView,
	tasks []schedtypes.TeachingTaskView,
	dims []string,
) schedtypes.ScoreBreakdown {
	if len(dims) == 0 {
		dims = []string{"time", "teacher", "student", "resource"}
	}
	perBucketMax := math.Round((100.0/float64(len(dims)))*100) / 100

	result := schedtypes.ScoreBreakdown{
		EnabledDimensions: dims,
		PlacedSessions:    len(assignments),
		PerBucketMax:      perBucketMax,
	}

	// 预期 session 数 — 使用与 time 包完全一致的解析逻辑
	expected := 0
	for _, t := range tasks {
		plan := resolveSessionPlan(t.CourseHours, t.StartWeek, t.EndWeek, t.MaxHoursPerWeek, t.PreferredSpan)
		expected += len(plan)
	}
	result.ExpectedSessions = expected

	// 构建查找表
	taskByID := make(map[uint]schedtypes.TeachingTaskView, len(tasks))
	for i := range tasks {
		taskByID[tasks[i].ID] = tasks[i]
	}
	teacherByID := make(map[uint]schedtypes.TeacherView, len(teachers))
	for i := range teachers {
		teacherByID[teachers[i].ID] = teachers[i]
	}
	classroomByID := make(map[uint]schedtypes.ClassroomView, len(classrooms))
	for i := range classrooms {
		classroomByID[classrooms[i].ID] = classrooms[i]
	}

	for _, dim := range dims {
		bucket := schedtypes.ScoreBucket{
			Max:     perBucketMax,
			Details: make(map[string]float64),
		}
		switch dim {
		case "time":
			bucket.Value = sc.scoreTimeBucket(assignments, taskByID, teacherByID, perBucketMax)
			result.Time = &bucket
		case "teacher":
			bucket.Value = sc.scoreTeacherBucket(assignments, taskByID, teacherByID, perBucketMax)
			result.Teacher = &bucket
		case "student":
			bucket.Value = sc.scoreStudentBucket(assignments, taskByID, perBucketMax)
			result.Student = &bucket
		case "resource":
			bucket.Value = sc.scoreResourceBucket(assignments, allocations, taskByID, teacherByID, classroomByID, perBucketMax)
			result.Resource = &bucket
		}
	}

	var total float64
	for _, b := range []*schedtypes.ScoreBucket{result.Time, result.Teacher, result.Student, result.Resource} {
		if b != nil {
			total += b.Value
		}
	}
	result.Total = math.Round(total*100) / 100

	if result.ExpectedSessions > 0 {
		ratio := float64(result.PlacedSessions) / float64(result.ExpectedSessions)
		if ratio > 1 {
			ratio = 1
		}
		result.Completeness = math.Round(ratio*10000) / 10000
		factor := ratio * (0.5 + 0.5*ratio)
		result.FinalTotal = math.Round(result.Total*factor*100) / 100
	} else {
		result.Completeness = 1.0
		result.FinalTotal = result.Total
	}

	return result
}

// ---- bucket scoring (all return values in [0, perMax]) ----

func (sc *Scorer) scoreTimeBucket(
	assignments []schedtypes.TimeAssignmentDraft,
	taskByID map[uint]schedtypes.TeachingTaskView,
	teacherByID map[uint]schedtypes.TeacherView,
	perMax float64,
) float64 {
	if len(assignments) == 0 {
		return perMax
	}
	subScores := make([]float64, 0, 4)

	// 1) teacher_preference
	{
		type tstats struct{ early, late, total int; hasPref bool }
		stats := make(map[uint]*tstats)
		for _, a := range assignments {
			task, ok := taskByID[a.TeachingTaskID]
			if !ok { continue }
			tv := teacherByID[task.TeacherID]
			s := stats[task.TeacherID]
			if s == nil { s = &tstats{}; stats[task.TeacherID] = s }
			s.total++
			if tv.PreferNoEarly && int(a.StartPeriod) <= 1 { s.early++ }
			if tv.PreferNoLate && int(a.StartPeriod) >= 6 { s.late++ }
			if tv.PreferNoEarly || tv.PreferNoLate { s.hasPref = true }
		}
		var penalty float64; prefN := 0
		for _, s := range stats {
			if s.hasPref && s.total > 0 {
				prefN++
				penalty += float64(s.early+s.late) / float64(s.total)
			}
		}
		if prefN == 0 { subScores = append(subScores, perMax) } else {
			v := perMax * (1.0 - penalty/float64(prefN))
			if v < 0 { v = 0 }
			subScores = append(subScores, v)
		}
	}

	// 2) course_dispersed
	{
		courseDays := make(map[uint]*[7]int)
		for _, a := range assignments {
			task, ok := taskByID[a.TeachingTaskID]
			if !ok { continue }
			if _, ex := courseDays[task.CourseID]; !ex { courseDays[task.CourseID] = &[7]int{} }
			courseDays[task.CourseID][int(a.DayOfWeek)]++
		}
		if len(courseDays) == 0 {
			subScores = append(subScores, perMax)
		} else {
			var totalDisp float64
			for _, dc := range courseDays {
				totalSess, days := 0, make([]int, 0, 7)
				for d := 0; d < 7; d++ {
					if dc[d] > 0 { totalSess += dc[d]; days = append(days, d) }
				}
				if totalSess <= 1 { totalDisp += 1.0; continue }
				if len(days) == 1 { totalDisp += 1.0 / float64(totalSess); continue }
				sort.Ints(days)
				gapSum := 0.0
				for i := 0; i < len(days)-1; i++ {
					switch days[i+1] - days[i] {
					case 1: gapSum += 0.4
					case 2: gapSum += 0.8
					default: gapSum += 1.0
					}
				}
				gapScore := gapSum / float64(len(days)-1)
				sameDayExcess, maxDaily := 0, 0
				for _, d := range days {
					if dc[d] > maxDaily { maxDaily = dc[d] }
					if dc[d] > 1 { sameDayExcess += dc[d] - 1 }
				}
				concPen := float64(sameDayExcess) * 0.3
				idealMax := (totalSess + len(days) - 1) / len(days)
				balPen := 0.0
				if maxDaily > idealMax { balPen = float64(maxDaily-idealMax) * 0.15 }
				cs := gapScore * (1.0 - concPen - balPen)
				if cs < 0 { cs = 0 }
				totalDisp += cs
			}
			subScores = append(subScores, perMax*(totalDisp/float64(len(courseDays))))
		}
	}

	// 3) weekend_avoidance
	{
		sat, sun, total := 0, 0, len(assignments)
		for _, a := range assignments {
			if int(a.DayOfWeek) == 5 { sat++ }
			if int(a.DayOfWeek) == 6 { sun++ }
		}
		if total == 0 || (sat+sun == 0) { subScores = append(subScores, perMax) } else {
			v := perMax * (1.0 - float64(sat+sun)/float64(total))
			if v < 0 { v = 0 }
			subScores = append(subScores, v)
		}
	}

	// 4) pe_preferred_periods
	{
		peAtPref, peTotal := 0, 0
		for _, a := range assignments {
			task, ok := taskByID[a.TeachingTaskID]
			if !ok { continue }
			if task.IsSports {
				peTotal++
				start := int(a.StartPeriod)
				if start == 2 || start == 6 { peAtPref++ }
			}
		}
		if peTotal == 0 { subScores = append(subScores, perMax) } else {
			subScores = append(subScores, perMax*float64(peAtPref)/float64(peTotal))
		}
	}

	var sum float64
	for _, v := range subScores { sum += v }
	return math.Round(sum/float64(len(subScores))*100) / 100
}

func (sc *Scorer) scoreTeacherBucket(
	assignments []schedtypes.TimeAssignmentDraft,
	taskByID map[uint]schedtypes.TeachingTaskView,
	teacherByID map[uint]schedtypes.TeacherView,
	perMax float64,
) float64 {
	if len(assignments) == 0 { return perMax }

	teacherDays := make(map[uint]*[7]int)
	for _, a := range assignments {
		task, ok := taskByID[a.TeachingTaskID]
		if !ok { continue }
		if _, ex := teacherDays[task.TeacherID]; !ex { teacherDays[task.TeacherID] = &[7]int{} }
		teacherDays[task.TeacherID][int(a.DayOfWeek)]++
	}

	var totalQ float64; active := 0
	for tid, dc := range teacherDays {
		actualDays := 0
		for d := 0; d < 7; d++ { if dc[d] > 0 { actualDays++ } }
		if actualDays == 0 { continue }
		active++
		maxDays := 3
		if tv := teacherByID[tid]; tv.MaxDaysPerWeek > 0 { maxDays = tv.MaxDaysPerWeek }
		if actualDays <= maxDays { totalQ += 1.0 } else {
			extra := actualDays - maxDays
			penalty := float64(extra) / float64(maxDays)
			q := 1.0 - penalty
			if q < 0 { q = 0 }
			totalQ += q
		}
	}
	if active == 0 { return perMax }
	return math.Round(perMax*(totalQ/float64(active))*100) / 100
}

func (sc *Scorer) scoreStudentBucket(
	assignments []schedtypes.TimeAssignmentDraft,
	taskByID map[uint]schedtypes.TeachingTaskView,
	perMax float64,
) float64 {
	if len(assignments) == 0 { return perMax }

	classDayBits := make(map[uint]*[7]uint16)
	for _, a := range assignments {
		task, ok := taskByID[a.TeachingTaskID]
		if !ok { continue }
		for _, cgID := range task.ClassGroupIDs {
			if _, ex := classDayBits[cgID]; !ex { classDayBits[cgID] = &[7]uint16{} }
			bits := &classDayBits[cgID][int(a.DayOfWeek)]
			for p := int(a.StartPeriod); p < int(a.StartPeriod)+a.Span; p++ {
				if p >= 0 && p <= 10 { *bits |= uint16(1) << uint(p) }
			}
		}
	}

	maxConsecutive := 0
	for _, days := range classDayBits {
		for d := 0; d < 7; d++ {
			bits := days[d]
			if bits == 0 { continue }
			longest, current := 0, 0
			for p := 0; p <= 10; p++ {
				if bits&(uint16(1)<<uint(p)) != 0 { current++; if current > longest { longest = current }
				} else { current = 0 }
			}
			if longest > maxConsecutive { maxConsecutive = longest }
		}
	}

	threshold := 4
	if maxConsecutive <= threshold { return perMax }
	extra := maxConsecutive - threshold
	if extra > 6 { extra = 6 }
	penaltyFactor := float64(extra) / 6.0
	v := perMax * (1.0 - penaltyFactor)
	if v < 0 { v = 0 }
	return math.Round(v*100) / 100
}

func (sc *Scorer) scoreResourceBucket(
	assignments []schedtypes.TimeAssignmentDraft,
	allocations []schedtypes.RoomAllocationDraft,
	taskByID map[uint]schedtypes.TeachingTaskView,
	teacherByID map[uint]schedtypes.TeacherView,
	classroomByID map[uint]schedtypes.ClassroomView,
	perMax float64,
) float64 {
	if len(allocations) == 0 { return perMax }
	subScores := make([]float64, 0, 2)

	// 1) low_floor_preference
	{
		maxFloor := 1
		for _, c := range classroomByID { if c.Floor > maxFloor { maxFloor = c.Floor } }

		teacherFloorSum := make(map[uint]float64)
		teacherFloorCount := make(map[uint]int)
		for i, a := range assignments {
			alloc, ok := allocationsByRef(allocations, i)
			if !ok { continue }
			task, ok := taskByID[a.TeachingTaskID]
			if !ok { continue }
			tv := teacherByID[task.TeacherID]
			if !tv.PreferLowFloor { continue }
			room, ok := classroomByID[alloc.ClassroomID]
			if !ok { continue }
			teacherFloorSum[task.TeacherID] += float64(room.Floor)
			teacherFloorCount[task.TeacherID]++
		}

		if maxFloor <= 1 || len(teacherFloorCount) == 0 {
			subScores = append(subScores, perMax)
		} else {
			var totalQ float64; activeFloor := 0
			for tid, cnt := range teacherFloorCount {
				if cnt == 0 { continue }
				avgFloor := teacherFloorSum[tid] / float64(cnt)
				q := 1.0 - (avgFloor-1.0)/float64(maxFloor-1)
				if q < 0 { q = 0 }
				if q > 1.0 { q = 1.0 }
				totalQ += q
				activeFloor++
			}
			if activeFloor == 0 { subScores = append(subScores, perMax) } else {
				subScores = append(subScores, perMax*(totalQ/float64(activeFloor)))
			}
		}
	}

	// 2) capacity_fit
	{
		totalWaste, totalCapacity := 0, 0
		for i, a := range assignments {
			alloc, ok := allocationsByRef(allocations, i)
			if !ok { continue }
			room, ok := classroomByID[alloc.ClassroomID]
			if !ok { continue }
			task, ok := taskByID[a.TeachingTaskID]
			if !ok { continue }
			if room.Capacity > 0 {
				totalCapacity += room.Capacity
				waste := room.Capacity - task.TotalStudents
				if waste > 0 { totalWaste += waste }
			}
		}
		if totalCapacity == 0 || totalWaste <= 0 { subScores = append(subScores, perMax) } else {
			ratio := 1.0 - float64(totalWaste)/float64(totalCapacity)
			if ratio < 0 { ratio = 0 }
			subScores = append(subScores, perMax*ratio)
		}
	}

	var sum float64
	for _, v := range subScores { sum += v }
	return math.Round(sum/float64(len(subScores))*100) / 100
}

func allocationsByRef(allocations []schedtypes.RoomAllocationDraft, ref int) (schedtypes.RoomAllocationDraft, bool) {
	for _, a := range allocations { if a.LocalRef == ref { return a, true } }
	return schedtypes.RoomAllocationDraft{}, false
}

// ---- Session Plan（与 scheduling/time/session_plan.go 完全一致）----

func resolveSessionPlan(courseHours, startWeek, endWeek, maxHoursPerWeek, preferredSpan int) []int {
	weeks := endWeek - startWeek + 1
	if weeks < 1 { weeks = 1 }
	if courseHours <= 0 { return []int{2} }
	weeklyHoursR := (courseHours + weeks/2) / weeks
	if weeklyHoursR < 1 { weeklyHoursR = 1 }
	if maxHoursPerWeek > 0 && weeklyHoursR > maxHoursPerWeek { weeklyHoursR = maxHoursPerWeek }
	if preferredSpan >= 1 && preferredSpan <= 3 { return planFromPreferredSpan(weeklyHoursR, preferredSpan) }
	return planFromWeeklyHours(weeklyHoursR)
}

func planFromWeeklyHours(wh int) []int {
	if wh <= 0 { return []int{2} }
	switch wh {
	case 1: return []int{1}
	case 2: return []int{2}
	case 3: return []int{3}
	case 4: return []int{2, 2}
	case 5: return []int{2, 2, 1}
	case 6: return []int{2, 2, 2}
	case 7: return []int{2, 2, 2, 1}
	default: return []int{2, 2, 2, 2}
	}
}

func planFromPreferredSpan(wh, span int) []int {
	if wh <= 0 { return []int{span} }
	spans := make([]int, 0, 4)
	remaining := wh
	for remaining >= span && len(spans) < 4 { spans = append(spans, span); remaining -= span }
	if remaining > 0 && remaining <= 3 && len(spans) < 4 { spans = append(spans, remaining) }
	if len(spans) == 0 {
		s := wh; if s > 3 { s = 3 }; spans = []int{s}
	}
	return spans
}
