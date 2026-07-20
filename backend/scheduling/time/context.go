package time

import (
	"math/rand"

	schedtypes "scheduling-system/backend/scheduling/types"
)

// timeEntry 是求解器内部的排课条目表示。
// 它扩展 TimeAssignmentDraft，携带评分和冲突检测所需的 CourseID/TeacherID。
type timeEntry struct {
	TeachingTaskID uint
	CourseID       uint
	TeacherID      uint
	ClassGroupIDs  []uint
	DayOfWeek      int // 0=Mon .. 6=Sun
	StartPeriod    int // 0-10
	Span           int // 1-4
}

// taskInfo 是教学任务的内部摘要。
type taskInfo struct {
	CourseID      uint
	TeacherID     uint
	ClassGroupIDs []uint
	TotalStudents int
	CourseHours   int
	StartWeek     int
	EndWeek       int
	MaxPerWeek    int
	PreferredSpan int
	IsSports      bool
}

// timeContext 持有一次求解运行的全部可变状态。
type timeContext struct {
	entries    []timeEntry
	taskByID   map[uint]*taskInfo
	teacherByID map[uint]*schedtypes.TeacherView

	lockedSlots    []schedtypes.LockedTimeSlot
	constraints    []string
	semesterID     uint
	sportsCourseIDs map[uint]bool

	// 占用图（仅时间和师生维度，无教室）
	teacherOcc map[uint64]bool // key = occKey(day, period, teacherID)
	classOcc   map[uint64]bool // key = occKey(day, period, classGroupID)

	// 教师不可用时段（keyed by teacher ID）
	teacherUnavailable map[uint][]schedtypes.LockedTimeSlot

	// 评分
	enabledMap            map[string]bool
	sCache                *scoreCache
	expectedTotalSessions int

	// 邻域回滚
	lastNeighbor neighborOp

	rng *rand.Rand
}

// newTimeContext 从 TimeSchedulingInput 构建内部上下文。
func newTimeContext(input schedtypes.TimeSchedulingInput, rng *rand.Rand) *timeContext {
	taskByID := make(map[uint]*taskInfo, len(input.Tasks))
	teacherByID := make(map[uint]*schedtypes.TeacherView, len(input.Teachers))
	sportsCourseIDs := make(map[uint]bool)

	expectedTotalSessions := 0
	for _, t := range input.Tasks {
		hours := t.CourseHours
		if hours <= 0 {
			hours = t.TotalStudents // fallback — shouldn't happen
		}
		plan := resolveSessionPlan(hours, t.StartWeek, t.EndWeek, t.MaxHoursPerWeek, t.PreferredSpan)
		expectedTotalSessions += len(plan)

		taskByID[t.ID] = &taskInfo{
			CourseID:      t.CourseID,
			TeacherID:     t.TeacherID,
			ClassGroupIDs: t.ClassGroupIDs,
			TotalStudents: t.TotalStudents,
			CourseHours:   t.CourseHours,
			StartWeek:     t.StartWeek,
			EndWeek:       t.EndWeek,
			MaxPerWeek:    t.MaxHoursPerWeek,
			PreferredSpan: t.PreferredSpan,
			IsSports:      t.IsSports,
		}
		if t.IsSports {
			sportsCourseIDs[t.CourseID] = true
		}
	}

	for i := range input.Teachers {
		tv := &input.Teachers[i]
		teacherByID[tv.ID] = tv
	}

	// 构建教师不可用索引
	teacherUnavailable := make(map[uint][]schedtypes.LockedTimeSlot)
	for _, tv := range input.Teachers {
		if len(tv.UnavailableSlots) > 0 {
			teacherUnavailable[tv.ID] = tv.UnavailableSlots
		}
	}

	// 构建约束 enabled map
	enabledMap := make(map[string]bool)
	constraints := input.Constraints
	if len(constraints) == 0 {
		constraints = defaultConstraints()
	}
	for _, c := range constraints {
		enabledMap[c] = true
	}

	sctx := &timeContext{
		entries:             make([]timeEntry, 0, expectedTotalSessions),
		taskByID:            taskByID,
		teacherByID:         teacherByID,
		lockedSlots:         input.LockedSlots,
		constraints:         constraints,
		semesterID:          input.SemesterID,
		sportsCourseIDs:     sportsCourseIDs,
		teacherOcc:          make(map[uint64]bool),
		classOcc:            make(map[uint64]bool),
		teacherUnavailable:  teacherUnavailable,
		enabledMap:          enabledMap,
		expectedTotalSessions: expectedTotalSessions,
		rng:                 rng,
	}

	// 预构建评分缓存
	sctx.sCache = newScoreCache(teacherByID, taskByID)

	return sctx
}

// defaultConstraints 返回默认启用的约束集。
func defaultConstraints() []string {
	return []string{
		"teacher_preference",
		"course_dispersed",
		"teacher_days_limit",
		"avoid_saturday",
		"avoid_sunday",
		"pe_preferred_periods",
		"student_fatigue",
	}
}

// occKey 将 (day, period, resourceID) 编码为单个 uint64。
// day∈[0,6], period∈[0,10], id 使用低 40 位。
func occKey(day, period int, id uint) uint64 {
	return uint64(uint32(day))<<48 | uint64(uint32(period))<<40 | uint64(id)
}

// hasConstraint 检查约束集中是否启用了 key。
func (sctx *timeContext) hasConstraint(key string) bool {
	return sctx.enabledMap[key]
}

// entriesToDrafts 将 timeEntry 切片转为 TimeAssignmentDraft 切片。
func entriesToDrafts(entries []timeEntry) []schedtypes.TimeAssignmentDraft {
	drafts := make([]schedtypes.TimeAssignmentDraft, len(entries))
	for i, e := range entries {
		drafts[i] = schedtypes.TimeAssignmentDraft{
			TeachingTaskID: e.TeachingTaskID,
			DayOfWeek:      schedtypes.DayOfWeek(e.DayOfWeek),
			StartPeriod:    schedtypes.Period(e.StartPeriod),
			Span:           e.Span,
		}
	}
	return drafts
}
