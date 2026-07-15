package services

import (
	"fmt"
	"math"
	"scheduling-system/backend/models"
	"sort"
)

const ScoreEpsilon = 0.01

// ScoreEquals determines whether two scores are considered equal within epsilon.
func ScoreEquals(a, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < ScoreEpsilon
}

// ScoreGreater determines whether a is significantly greater than b.
func ScoreGreater(a, b float64) bool {
	return a > b+ScoreEpsilon
}

// ScoringService evaluates a schedule's quality against soft constraints.
type ScoringService struct{}

func NewScoringService() *ScoringService {
	return &ScoringService{}
}

// ScoreBreakdown holds the detailed scoring result.
type ScoreBreakdown struct {
	Total         float64 `json:"total"`
	TeacherPref   float64 `json:"teacherPref"`   // 教师偏好满足度
	CourseSpacing float64 `json:"courseSpacing"` // 课程间隔均匀度
	TeacherDays   float64 `json:"teacherDays"`   // 教师到校天数
	LowFloorPref  float64 `json:"lowFloorPref"`  // 优先低楼层
	WeekendAvoid  float64 `json:"weekendAvoid"`  // 周末避让
	PePeriodPref  float64 `json:"pePeriodPref"`  // 体育课时段偏好
	StudentFatigue       float64            `json:"studentFatigue"`
	PerCategoryMax       float64            `json:"perCategoryMax"`
	EnabledCategoryCount int                `json:"enabledCategoryCount"`
	CategoryMaxes        map[string]float64 `json:"categoryMaxes,omitempty"` // v0.5.6: per-category actual max (weight × perCategoryMax)

	// v0.5.2: placement completeness fields.
	// Total keeps its v0.4 semantics (sum of 7 soft-constraint categories).
	// FinalTotal is Total scaled by a completeness factor so under-placed schedules
	// receive a lower published score. When ExpectedSessions==0 or PlacedSessions
	// equals it, FinalTotal == Total (v0.4-compatible round-trip).
	PlacedSessions   int     `json:"placedSessions,omitempty"`
	ExpectedSessions int     `json:"expectedSessions,omitempty"`
	Completeness     float64 `json:"completeness,omitempty"` // ratio in [0,1]
	FinalTotal       float64 `json:"finalTotal"`

	// v0.5.5 P0 M2: nullable buckets — Disabled 语义靠 nil 表达 (INV-S1/S2/I8)。
	// 与上面的 float64 字段并存作为过渡：读侧优先 buckets（nil = Disabled;
	// non-nil = 已评估分值），旧调用可继续读 float64（Disabled 与 真 0 都是 0）。
	// 一旦所有前端消费方切到 buckets，float64 可在下版本删除。
	Buckets *ScoreBuckets `json:"buckets,omitempty"`

	// EnabledDimensions echoes the mode-derived dimension list (INV-S2)。
	// TIME_ONLY → ["time","teacher","student"]；FULL → [...+"resource"]。
	EnabledDimensions []string `json:"enabledDimensions,omitempty"`
}

// ScoreBuckets 按 spec 2.7 冻结的四桶结构表达当前排课结果分值。
// 每个 bucket 用指针：nil 表示该维度被禁用（如 TIME_ONLY 的 Resource），
// non-nil 但 Value=0 表示"评估了,得 0 分"。这是"Disabled = missing, not 0"的关键。
type ScoreBuckets struct {
	Time     *ScoreBucket `json:"time,omitempty"`
	Teacher  *ScoreBucket `json:"teacher,omitempty"`
	Student  *ScoreBucket `json:"student,omitempty"`
	Resource *ScoreBucket `json:"resource,omitempty"` // TIME_ONLY 下必为 nil
}

// ScoreBucket 一个维度的评分聚合。Details 保留子约束分值以便前端做详情。
type ScoreBucket struct {
	Value   float64            `json:"value"`
	Max     float64            `json:"max"`
	Details map[string]float64 `json:"details,omitempty"`
}

// ScoreSchedule evaluates a full schedule against soft constraints.
// ctx provides enabled constraints, sports course IDs, and teaching tasks.
// Returns a score from 0-100 with detailed breakdown.
func (s *ScoringService) ScoreSchedule(entries []models.ScheduleEntry, teachers []models.Teacher, classrooms []models.Classroom, ctx ScoringContext) ScoreBreakdown {
	breakdown := ScoreBreakdown{}

	enabledConstraints := ctx.EnabledConstraints
	ttList := ctx.TeachingTasks
	sportsCourseIDs := ctx.SportsCourseIDs

	if len(enabledConstraints) == 0 {
		// Default: enable all known constraints
		enabledConstraints = FullDefaultConstraints()
	}

	enabled := make(map[string]bool)
	for _, c := range enabledConstraints {
		enabled[c] = true
	}

	// v0.5.5 P0 M2: 结构化维度门控 —— 从 ctx.Mode.EnabledScoreDimensions() 派生
	// 允许评估的维度集合，禁用的维度对应 sub-constraints 一律关闭。这样
	// TIME_ONLY 模式下无论 EnabledConstraints 是否包含 low_floor_preference，
	// 资源维度都不会被评分（对齐 INV-S1/S2/I8：Disabled = missing, not 0）。
	dims := make(map[string]bool)
	for _, d := range ctx.EffectiveMode().EnabledScoreDimensions() {
		dims[d] = true
	}
	if !dims["resource"] {
		enabled["low_floor_preference"] = false
	}
	// I8 前瞻：一旦未来新增 resource 子约束（capacity/type/equipment），
	// 在此一次性禁用即可，无需散布在各评分函数内。

	// Weighted scoring: each category's max is proportional to its constraint weights.
	// When ConstraintWeights is nil (legacy), fall back to equal weighting.
	weights := ctx.ConstraintWeights
	getWeight := func(key string) int {
		if weights != nil {
			if w, ok := weights[key]; ok {
				return w
			}
			return 0 // explicit weights map, key not present = disabled
		}
		return 1 // no weights configured = equal weight
	}

	// Compute per-category weight and total
	type categoryWeight struct {
		weight int
		keys   []string // sub-constraint keys contributing to this category
	}
	categories := []categoryWeight{
		{keys: []string{"teacher_preference"}},
		{keys: []string{"course_dispersed"}},
		{keys: []string{"teacher_days_limit"}},
		{keys: []string{"low_floor_preference"}},
		{keys: []string{"avoid_saturday", "avoid_sunday"}},
		{keys: []string{"pe_preferred_periods"}},
		{keys: []string{"student_fatigue"}},
	}
	// Apply enablement: zero out disabled categories
	if !enabled["teacher_preference"] {
		categories[0].keys = nil
	}
	if !enabled["course_dispersed"] {
		categories[1].keys = nil
	}
	if !enabled["teacher_days_limit"] {
		categories[2].keys = nil
	}
	if !enabled["low_floor_preference"] {
		categories[3].keys = nil
	}
	if !enabled["avoid_saturday"] && !enabled["avoid_sunday"] {
		categories[4].keys = nil
	}
	if !enabled["pe_preferred_periods"] || sportsCourseIDs == nil {
		categories[5].keys = nil
	}
	if !enabled["student_fatigue"] || len(ttList) == 0 {
		categories[6].keys = nil
	}

	// Compute per-category weight (max of sub-constraint weights) and total
	totalWeight := 0
	for i := range categories {
		maxW := 0
		for _, k := range categories[i].keys {
			if w := getWeight(k); w > maxW {
				maxW = w
			}
		}
		categories[i].weight = maxW
		totalWeight += maxW
	}

	enabledCount := 0
	for _, c := range categories {
		if c.weight > 0 {
			enabledCount++
		}
	}

	// Per-category max: proportional to weight, or equal if no weights configured
	var perCategoryMax float64
	if enabledCount == 0 {
		perCategoryMax = 25.0
	} else if weights == nil || totalWeight == 0 {
		// Legacy equal weighting (or all weights explicitly zero)
		perCategoryMax = 100.0 / float64(enabledCount)
	} else {
		perCategoryMax = 100.0 / float64(totalWeight)
	}
	breakdown.PerCategoryMax = math.Round(perCategoryMax*100) / 100
	breakdown.EnabledCategoryCount = enabledCount

	// Build lookup maps
	teacherMap := make(map[uint]models.Teacher)
	for _, t := range teachers {
		teacherMap[t.ID] = t
	}
	classroomMap := make(map[uint]models.Classroom)
	for _, c := range classrooms {
		classroomMap[c.ID] = c
	}

	// Weighted category maxes: each category's max = its weight × perCategoryMax
	teacherMax := float64(categories[0].weight) * perCategoryMax
	courseMax := float64(categories[1].weight) * perCategoryMax
	daysMax := float64(categories[2].weight) * perCategoryMax
	floorMax := float64(categories[3].weight) * perCategoryMax
	weekendMax := float64(categories[4].weight) * perCategoryMax
	peMax := float64(categories[5].weight) * perCategoryMax
	fatigueMax := float64(categories[6].weight) * perCategoryMax

	// Store actual per-category maxes for frontend percentage display
	breakdown.CategoryMaxes = map[string]float64{
		"teacherPref":   math.Round(teacherMax*100) / 100,
		"courseSpacing": math.Round(courseMax*100) / 100,
		"teacherDays":   math.Round(daysMax*100) / 100,
		"lowFloorPref":  math.Round(floorMax*100) / 100,
		"weekendAvoid":  math.Round(weekendMax*100) / 100,
		"pePeriodPref":  math.Round(peMax*100) / 100,
		"studentFatigue": math.Round(fatigueMax*100) / 100,
	}

	// 1. Teacher preference scoring
	if enabled["teacher_preference"] {
		breakdown.TeacherPref = s.scoreTeacherPreferences(entries, teacherMap, teacherMax)
	}

	// 2. Course spacing scoring
	if enabled["course_dispersed"] {
		breakdown.CourseSpacing = s.scoreCourseSpacing(entries, courseMax)
	}

	// 3. Teacher days per week scoring
	if enabled["teacher_days_limit"] {
		breakdown.TeacherDays = s.scoreTeacherDays(entries, teacherMap, daysMax)
	}

	// 4. Low floor preference scoring
	if enabled["low_floor_preference"] {
		breakdown.LowFloorPref = s.scoreLowFloorPref(entries, teacherMap, classroomMap, floorMax)
	}

	// 5. Weekend avoidance scoring
	if enabled["avoid_saturday"] || enabled["avoid_sunday"] {
		breakdown.WeekendAvoid = s.scoreWeekendAvoid(entries, enabled, weekendMax)
	}

	// 6. Sports period preference
	if enabled["pe_preferred_periods"] && sportsCourseIDs != nil {
		breakdown.PePeriodPref = s.scorePePeriodPref(entries, sportsCourseIDs, peMax)
	}

	// 7. Student fatigue scoring (requires teaching task data)
	if enabled["student_fatigue"] && len(ttList) > 0 {
		breakdown.StudentFatigue = s.scoreStudentFatigue(entries, ttList, fatigueMax)
	}

	breakdown.Total = math.Round((breakdown.TeacherPref + breakdown.CourseSpacing + breakdown.TeacherDays + breakdown.LowFloorPref + breakdown.WeekendAvoid + breakdown.PePeriodPref + breakdown.StudentFatigue)*100) / 100

	// v0.5.2: placement completeness scaling.
	// Total keeps its v0.4 semantics (sum of soft categories). FinalTotal exposes
	// a completeness-scaled published score: schedules that placed only half their
	// expected sessions are penalized so they can't win by "just placing fewer".
	//
	// Curve (β): factor = ratio × (0.5 + 0.5 × ratio) — quadratic-linear blend.
	//   ratio=1.0 → 1.0      (full placement leaves Total unchanged)
	//   ratio=0.5 → 0.375    (50% completeness produces ≤37.5% × Total ≤ 60 pts)
	//   ratio=0.0 → 0.0
	//
	// When ExpectedTotalSessions is 0 (not supplied by caller — legacy path)
	// we fall through to factor=1.0 to preserve v0.4 semantic round-trip.
	breakdown.PlacedSessions = len(entries)
	if ctx.ExpectedTotalSessions > 0 {
		breakdown.ExpectedSessions = ctx.ExpectedTotalSessions
		ratio := float64(breakdown.PlacedSessions) / float64(ctx.ExpectedTotalSessions)
		if ratio > 1 {
			ratio = 1
		}
		if ratio < 0 {
			ratio = 0
		}
		breakdown.Completeness = math.Round(ratio*10000) / 10000
		factor := ratio * (0.5 + 0.5*ratio)
		breakdown.FinalTotal = math.Round(breakdown.Total*factor*100) / 100
	} else {
		breakdown.ExpectedSessions = breakdown.PlacedSessions
		breakdown.Completeness = 1.0
		breakdown.FinalTotal = breakdown.Total
	}

	// v0.5.5 P0 M2: 构建 nullable buckets (spec 2.7 冻结的 4 桶分配)。
	// 未启用维度对应 bucket = nil (Disabled 语义); 启用但子约束全 disabled
	// 对应 bucket = non-nil{Value:0}(评估了得 0 分)。
	breakdown.EnabledDimensions = ctx.EffectiveMode().EnabledScoreDimensions()
	dimSet := make(map[string]bool, len(breakdown.EnabledDimensions))
	for _, d := range breakdown.EnabledDimensions {
		dimSet[d] = true
	}
	buckets := &ScoreBuckets{}
	// Time bucket: course_dispersed + avoid_saturday/sunday + pe_preferred_periods
	if dimSet["time"] {
		details := map[string]float64{}
		val := 0.0
		if enabled["course_dispersed"] {
			details["course_dispersed"] = breakdown.CourseSpacing
			val += breakdown.CourseSpacing
		}
		if enabled["avoid_saturday"] || enabled["avoid_sunday"] {
			details["weekend_avoid"] = breakdown.WeekendAvoid
			val += breakdown.WeekendAvoid
		}
		if enabled["pe_preferred_periods"] {
			details["pe_preferred_periods"] = breakdown.PePeriodPref
			val += breakdown.PePeriodPref
		}
		buckets.Time = &ScoreBucket{Value: val, Max: courseMax + weekendMax + peMax, Details: details}
	}
	// Teacher bucket: teacher_preference + teacher_days_limit
	if dimSet["teacher"] {
		details := map[string]float64{}
		val := 0.0
		if enabled["teacher_preference"] {
			details["teacher_preference"] = breakdown.TeacherPref
			val += breakdown.TeacherPref
		}
		if enabled["teacher_days_limit"] {
			details["teacher_days_limit"] = breakdown.TeacherDays
			val += breakdown.TeacherDays
		}
		buckets.Teacher = &ScoreBucket{Value: val, Max: teacherMax + daysMax, Details: details}
	}
	// Student bucket: student_fatigue
	if dimSet["student"] {
		details := map[string]float64{}
		if enabled["student_fatigue"] {
			details["student_fatigue"] = breakdown.StudentFatigue
		}
		buckets.Student = &ScoreBucket{Value: breakdown.StudentFatigue, Max: fatigueMax, Details: details}
	}
	// Resource bucket: low_floor_preference (spec §2.7)
	// TIME_ONLY 时 dimSet["resource"] == false → Resource 保持 nil (INV-S1: Disabled = nil)
	if dimSet["resource"] {
		details := map[string]float64{}
		if enabled["low_floor_preference"] {
			details["low_floor_preference"] = breakdown.LowFloorPref
		}
		buckets.Resource = &ScoreBucket{Value: breakdown.LowFloorPref, Max: floorMax, Details: details}
	}
	breakdown.Buckets = buckets

	return breakdown
}

// scoreTeacherPreferences evaluates how well teacher time preferences are met.
// Each teacher with PreferNoEarly should not have entries in periods 0-1 (第1-2节).
// Each teacher with PreferNoLate should not have entries in periods 6+ (第7节及以后).
func (s *ScoringService) scoreTeacherPreferences(entries []models.ScheduleEntry, teacherMap map[uint]models.Teacher, maxScore float64) float64 {
	type teacherStat struct {
		hasPref    bool
		earlyCount int
		lateCount  int
		totalCount int
	}
	stats := make(map[uint]*teacherStat)

	for _, e := range entries {
		t, ok := teacherMap[e.TeacherID]
		if !ok {
			continue
		}
		st, exists := stats[e.TeacherID]
		if !exists {
			st = &teacherStat{
				hasPref: t.PreferNoEarly || t.PreferNoLate,
			}
			stats[e.TeacherID] = st
		}
		st.totalCount++
		if t.PreferNoEarly && e.StartPeriod <= 1 {
			st.earlyCount++
		}
		if t.PreferNoLate && e.StartPeriod >= 6 {
			st.lateCount++
		}
	}

	if len(stats) == 0 {
		return maxScore
	}

	totalPenalty := 0.0
	prefTeacherCount := 0
	for _, st := range stats {
		if !st.hasPref {
			continue
		}
		prefTeacherCount++
		if st.totalCount > 0 {
			penalty := float64(st.earlyCount+st.lateCount) / float64(st.totalCount)
			totalPenalty += penalty
		}
	}

	if prefTeacherCount == 0 {
		return maxScore
	}

	// Average across preference teachers, scale to 0-maxScore
	avgPenalty := totalPenalty / float64(prefTeacherCount)
		score := maxScore * (1.0 - avgPenalty)
		if score < 0 {
			score = 0
		}
		return score
}

// scoreCourseSpacing evaluates how evenly each course's sessions are distributed
// across the week, with emphasis on day-gap quality.
// Single-session courses get full marks. Multi-session courses are scored by
// the gaps between consecutive occupied days: gap≥3→1.0, gap=2→0.8, gap=1→0.4.
func (s *ScoringService) scoreCourseSpacing(entries []models.ScheduleEntry, maxScore float64) float64 {
	type courseInfo struct {
		dayCounts map[int]int // day -> number of sessions on that day
	}
	courses := make(map[uint]*courseInfo)

	for _, e := range entries {
		ci, exists := courses[e.CourseID]
		if !exists {
			ci = &courseInfo{dayCounts: make(map[int]int)}
			courses[e.CourseID] = ci
		}
		ci.dayCounts[int(e.DayOfWeek)]++
	}

	if len(courses) == 0 {
		return maxScore
	}

	totalScore := 0.0
	for _, ci := range courses {
		totalSessions := 0
		for _, cnt := range ci.dayCounts {
			totalSessions += cnt
		}

		// Single-session courses: no spacing concern
		if totalSessions <= 1 {
			totalScore += 1.0
			continue
		}

		// Collect and sort unique occupied days
		days := make([]int, 0, len(ci.dayCounts))
		for d := range ci.dayCounts {
			days = append(days, d)
		}
		sort.Ints(days)

		// All sessions on the same day: penalize by session count
		if len(days) == 1 {
			totalScore += 1.0 / float64(totalSessions)
			continue
		}

		// Score gaps between consecutive occupied days
		gapSum := 0.0
		for i := 0; i < len(days)-1; i++ {
			gap := days[i+1] - days[i]
			switch {
			case gap >= 3:
				gapSum += 1.0 // ideal: Mon+Thu, Mon+Fri, Tue+Fri
			case gap == 2:
				gapSum += 0.8 // good: Mon+Wed, Tue+Thu, Wed+Fri
			case gap == 1:
				gapSum += 0.4 // consecutive: Mon+Tue, Tue+Wed, etc.
			}
		}
		gapScore := gapSum / float64(len(days)-1)

		// Same-day concentration penalty: extra sessions on same day reduce score
		sameDayExcess := 0
		maxDaily := 0
		for _, cnt := range ci.dayCounts {
			if cnt > maxDaily {
				maxDaily = cnt
			}
			if cnt > 1 {
				sameDayExcess += cnt - 1
			}
		}
		concentrationPenalty := float64(sameDayExcess) * 0.3

		// Daily balance penalty: max daily sessions exceeding ideal spread
		idealMax := (totalSessions + len(days) - 1) / len(days) // ceil(total/occupied_days)
		balancePenalty := 0.0
		if maxDaily > idealMax {
			balancePenalty = float64(maxDaily-idealMax) * 0.15
		}

		courseScore := gapScore * (1.0 - concentrationPenalty - balancePenalty)
		if courseScore < 0 {
			courseScore = 0
		}
		totalScore += courseScore
	}

	avgScore := totalScore / float64(len(courses))
	return maxScore * avgScore
}

// scoreTeacherDays evaluates how many distinct days each teacher comes to campus.
// Target: ≤3 days per week = full score. Each extra day reduces score.
func (s *ScoringService) scoreTeacherDays(entries []models.ScheduleEntry, teacherMap map[uint]models.Teacher, maxScore float64) float64 {
	teacherDays := make(map[uint]map[int]bool)

	for _, e := range entries {
		days, exists := teacherDays[e.TeacherID]
		if !exists {
			days = make(map[int]bool)
			teacherDays[e.TeacherID] = days
		}
		days[int(e.DayOfWeek)] = true
	}

	if len(teacherDays) == 0 {
		return maxScore
	}

	totalScore := 0.0
	for tid, days := range teacherDays {
		actualDays := len(days)
		maxDays := 3 // default
		if t, ok := teacherMap[tid]; ok && t.MaxDaysPerWeek > 0 {
			maxDays = t.MaxDaysPerWeek
		}
		if actualDays <= maxDays {
			totalScore += 1.0
		} else {
			// Penalty: each extra day reduces score proportionally
			extra := actualDays - maxDays
			penalty := float64(extra) / float64(maxDays)
			score := 1.0 - penalty
			if score < 0 {
				score = 0
			}
			totalScore += score
		}
	}

	avgScore := totalScore / float64(len(teacherDays))
	return maxScore * avgScore
}

// scoreLowFloorPref evaluates whether teachers who prefer low floors
// are assigned to classrooms on lower floors.
func (s *ScoringService) scoreLowFloorPref(entries []models.ScheduleEntry, teacherMap map[uint]models.Teacher, classroomMap map[uint]models.Classroom, maxScore float64) float64 {
	type floorStat struct {
		totalFloor   float64
		count        int
	}
	stats := make(map[uint]*floorStat)

	for _, e := range entries {
		if e.IsVirtual {
			continue // TIME_ONLY: virtual classroom, skip floor scoring
		}
		t, ok := teacherMap[e.TeacherID]
		if !ok || !t.PreferLowFloor {
			continue
		}
		c, ok := classroomMap[e.ClassroomID]
		if !ok {
			continue
		}
		st, exists := stats[e.TeacherID]
		if !exists {
			st = &floorStat{}
			stats[e.TeacherID] = st
		}
		st.totalFloor += float64(c.Floor)
		st.count++
	}

	if len(stats) == 0 {
		return maxScore
	}

	// Assume max floor across all classrooms for normalization
	maxFloor := 1
	for _, c := range classroomMap {
		if c.Floor > maxFloor {
			maxFloor = c.Floor
		}
	}
	if maxFloor <= 1 {
		return maxScore
	}

	totalScore := 0.0
	for _, st := range stats {
		avgFloor := st.totalFloor / float64(st.count)
		// Score: floor 1 = 1.0, floor maxFloor = 0.0
		score := 1.0 - (avgFloor-1.0)/float64(maxFloor-1)
		if score < 0 {
			score = 0
		}
		if score > 1.0 {
			score = 1.0
		}
		totalScore += score
	}

	avgScore := totalScore / float64(len(stats))
	return maxScore * avgScore
}

// scoreWeekendAvoid penalizes entries placed on Saturday and/or Sunday.
// Full score = no weekend entries. Each weekend entry reduces the score proportionally.
func (s *ScoringService) scoreWeekendAvoid(entries []models.ScheduleEntry, enabled map[string]bool, maxScore float64) float64 {
	if len(entries) == 0 {
		return maxScore
	}

	avoidSaturday := enabled["avoid_saturday"]
	avoidSunday := enabled["avoid_sunday"]

	saturdayCount := 0
	sundayCount := 0
	for _, e := range entries {
		if avoidSaturday && e.DayOfWeek == models.Sat {
			saturdayCount++
		}
		if avoidSunday && e.DayOfWeek == models.Sun {
			sundayCount++
		}
	}

	totalWeekend := saturdayCount + sundayCount
	if totalWeekend == 0 {
		return maxScore
	}

	// Penalty: each weekend entry loses 1/N of the max score, where N is total entries
	penalty := float64(totalWeekend) / float64(len(entries))
		score := maxScore * (1.0 - penalty)
		if score < 0 {
			score = 0
		}
		return score
}

// scorePePeriodPref evaluates whether sports courses are placed at preferred periods
// (3-4节 = startPeriod 2, 7-8节 = startPeriod 6).
func (s *ScoringService) scorePePeriodPref(entries []models.ScheduleEntry, sportsCourseIDs map[uint]bool, maxScore float64) float64 {
	preferredStarts := map[int]bool{2: true, 6: true} // startPeriod 2 (3-4节), 6 (7-8节)

	sportsCount := 0
	preferredCount := 0

	for _, e := range entries {
		if !sportsCourseIDs[e.CourseID] {
			continue
		}
		sportsCount++
		if preferredStarts[int(e.StartPeriod)] {
			preferredCount++
		}
	}

	if sportsCount == 0 {
		return maxScore
	}

	ratio := float64(preferredCount) / float64(sportsCount)
	return maxScore * ratio
	}

	// scoreStudentFatigue evaluates whether students have excessive consecutive periods.
	// For each class group, find the longest consecutive period span on a single day.
	// Full score (perCategoryMax) if no class exceeds 4 consecutive periods (2 course blocks).
	// Penalty increases proportionally for each extra consecutive period.
	// Requires teaching task data to map entries to class groups.
	func (s *ScoringService) scoreStudentFatigue(entries []models.ScheduleEntry, teachingTasks []models.TeachingTask, maxScore float64) float64 {
		// Build entry -> class group IDs map from teaching tasks
		type classDayInfo struct {
			periods map[int]bool // period numbers occupied for this (classGroup, day)
		}

		// Build teaching task lookup
		taskClassMap := make(map[uint][]uint) // teaching task ID -> class group IDs
		for _, tt := range teachingTasks {
			ids := make([]uint, len(tt.Classes))
			for j, c := range tt.Classes {
				ids[j] = c.ClassGroupID
			}
			taskClassMap[tt.ID] = ids
		}

		// Build class-day occupancy: for each class group, which periods are occupied per day
		type dayPeriods map[int]bool
		classDay := make(map[uint]map[int]dayPeriods) // classGroupID -> day -> set of periods

		for _, e := range entries {
			var cgIDs []uint
			if e.TeachingTaskID != nil {
				cgIDs = taskClassMap[*e.TeachingTaskID]
			} else if e.ClassGroupID != nil {
				cgIDs = []uint{*e.ClassGroupID}
			}
			if len(cgIDs) == 0 {
				continue
			}
			day := int(e.DayOfWeek)
			for p := int(e.StartPeriod); p < int(e.StartPeriod)+e.Span; p++ {
				for _, cgID := range cgIDs {
					if classDay[cgID] == nil {
						classDay[cgID] = make(map[int]dayPeriods)
					}
					if classDay[cgID][day] == nil {
						classDay[cgID][day] = make(dayPeriods)
					}
					classDay[cgID][day][p] = true
				}
			}
		}

		if len(classDay) == 0 {
			return maxScore
		}

		// For each class group, find the longest consecutive span
		maxConsecutiveAcrossAll := 0
		for _, days := range classDay {
			for _, periods := range days {
				// Find longest consecutive run of occupied periods
				longest := 0
				current := 0
				for p := 0; p <= 10; p++ { // periods 0-10
					if periods[p] {
						current++
						if current > longest {
							longest = current
						}
					} else {
						current = 0
					}
				}
				if longest > maxConsecutiveAcrossAll {
					maxConsecutiveAcrossAll = longest
				}
			}
		}

		// Threshold: 4 consecutive periods is acceptable (2 course blocks).
		// Up to 6 is somewhat tiring (3 blocks), beyond that is severe.
		threshold := 4
		if maxConsecutiveAcrossAll <= threshold {
			return maxScore
		}

		// Penalty: each extra period beyond threshold reduces score.
		// Max penalty at 10 consecutive periods (threshold + 6).
		extra := maxConsecutiveAcrossAll - threshold
		maxPenaltyRange := 6 // 10 - 4
		if extra > maxPenaltyRange {
			extra = maxPenaltyRange
		}
		penaltyFactor := float64(extra) / float64(maxPenaltyRange)
		score := maxScore * (1.0 - penaltyFactor)
		if score < 0 {
			score = 0
		}
		return score
	}

// TeacherWorkloadInfo holds per-teacher workload analysis (post-hoc, does not affect scoring).
type TeacherWorkloadInfo struct {
	TeacherID         uint    `json:"teacherId"`
	TeacherName       string  `json:"teacherName"`
	TotalSessions     int     `json:"totalSessions"`
	DailyDistribution []int   `json:"dailyDistribution"` // 7 elements, sessions per day Mon-Sun
	BusyDays          int     `json:"busyDays"`
	MaxDaily          int     `json:"maxDaily"`
	MinDaily          int     `json:"minDaily"` // min across all 7 days (may be 0)
	BalanceScore      float64 `json:"balanceScore"` // 0-100
	Suggestion        string  `json:"suggestion"`
}

// AnalyzeTeacherWorkload computes per-teacher workload balance from schedule entries.
// Pure analysis — does not affect scoring or solver output.
func (s *ScoringService) AnalyzeTeacherWorkload(entries []models.ScheduleEntry, teachers []models.Teacher) []TeacherWorkloadInfo {
	teacherMap := make(map[uint]string) // ID -> Name
	for _, t := range teachers {
		teacherMap[t.ID] = t.Name
	}

	// Build per-teacher per-day session counts
	type teacherData struct {
		name     string
		dayCount [7]int
	}
	data := make(map[uint]*teacherData)

	for _, e := range entries {
		td, ok := data[e.TeacherID]
		if !ok {
			name := teacherMap[e.TeacherID]
			if name == "" {
				name = fmt.Sprintf("教师#%d", e.TeacherID)
			}
			td = &teacherData{name: name}
			data[e.TeacherID] = td
		}
		td.dayCount[int(e.DayOfWeek)]++
	}

	result := make([]TeacherWorkloadInfo, 0, len(data))
	for tid, td := range data {
		total := 0
		maxD, minD := 0, 999
		busy := 0
		dist := make([]int, 7)
		for d := 0; d < 7; d++ {
			cnt := td.dayCount[d]
			dist[d] = cnt
			total += cnt
			if cnt > maxD {
				maxD = cnt
			}
			if cnt < minD {
				minD = cnt
			}
			if cnt > 0 {
				busy++
			}
		}
		if total == 0 {
			continue
		}

		// Balance score: penalize excess beyond ceil(total/7) on any day
		ideal := (total + 6) / 7 // ceil(total / 7)
		excess := 0
		for _, cnt := range dist {
			if cnt > ideal {
				excess += cnt - ideal
			}
		}
		balanceScore := math.Round((1.0-float64(excess)/math.Max(float64(total), 1))*10000) / 100
		if balanceScore < 0 {
			balanceScore = 0
		}
		if balanceScore > 100 {
			balanceScore = 100
		}

		// Generate suggestion
		suggestion := ""
		if maxD >= 5 {
			suggestion = fmt.Sprintf("单日课时过多(%d节)，建议控制在4节以内", maxD)
		} else if balanceScore < 50 {
			suggestion = "课程分布不均，建议分散到更多日期"
		} else if busy == 1 && total > 2 {
			suggestion = "所有课时集中在同一天"
		}

		result = append(result, TeacherWorkloadInfo{
			TeacherID:         tid,
			TeacherName:       td.name,
			TotalSessions:     total,
			DailyDistribution: dist,
			BusyDays:          busy,
			MaxDaily:          maxD,
			MinDaily:          minD,
			BalanceScore:      balanceScore,
			Suggestion:        suggestion,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].BalanceScore < result[j].BalanceScore
	})

	return result
}
