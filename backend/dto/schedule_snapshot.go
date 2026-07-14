package dto

import (
	"time"

	schedtypes "scheduling-system/backend/scheduling/types"
)

// SchemaVersionV055 是 v0.5.5 引入的 DTO 版本号。
// SnapshotService/VersionService 在读旧记录时对 schema_version 判断:
//   - "v0.5.5"       → 反序列化为 ScheduleSnapshotDTO
//   - "v0.5.4" 或空 → 进 legacy readonly 分支(INV-MI2)
const SchemaVersionV055 = "v0.5.5"

// ScheduleSnapshotDTO 是一个排课运行结果的终结序列化值。
// 一旦持久化,内容不再变化 —— 即使 teacher 名字后续改了,DTO 里的
// TeacherName 也保持快照时的原值(INV-F6,Snapshot Display Purity)。
type ScheduleSnapshotDTO struct {
	SchemaVersion     string                    `json:"schemaVersion"`
	SemesterID        uint                      `json:"semesterId"`
	SemesterName      string                    `json:"semesterName"`
	Mode              schedtypes.SchedulingMode `json:"mode"`
	ScheduleVersionID uint                      `json:"scheduleVersionId"`
	CreatedAt         time.Time                 `json:"createdAt"`
	Assignments       []ScheduledAssignmentDTO  `json:"assignments"`
	Score             ScoreBreakdownDTO         `json:"score"`
}

// ScheduledAssignmentDTO 是快照里一条"教学任务被排到的位置"记录。
// Time 层(TeachingTask 反射出的教师/课程/班级 + 时间)总是存在;
// Resource 层(Classroom 相关四字段)是可选指针 —— TIME_ONLY 模式全为 nil。
type ScheduledAssignmentDTO struct {
	// --- Time layer(源自 TimeAssignment)---
	TeachingTaskID  uint     `json:"teachingTaskId"`
	TeacherID       uint     `json:"teacherId"`
	TeacherName     string   `json:"teacherName"`
	CourseID        uint     `json:"courseId"`
	CourseName      string   `json:"courseName"`
	ClassGroupIDs   []uint   `json:"classGroupIds"`
	ClassGroupNames []string `json:"classGroupNames"`
	DayOfWeek       int      `json:"dayOfWeek"`   // 0=Mon..6=Sun
	StartPeriod     int      `json:"startPeriod"` // 0..10
	Span            int      `json:"span"`        // 1..3
	WeekRange       string   `json:"weekRange"`   // "1-16"

	// --- Resource layer(源自 ScheduleEntry,TIME_ONLY 全 nil)---
	ClassroomID    *uint   `json:"classroomId,omitempty"`
	ClassroomName  *string `json:"classroomName,omitempty"`
	ClassroomFloor *int    `json:"classroomFloor,omitempty"`
	ClassroomType  *string `json:"classroomType,omitempty"`
}

// ScoreBreakdownDTO 是快照持久化的评分结果 —— 是 services.ScoreBreakdown 的
// 序列化视图,不引入 services 包依赖(避免 dto→services 反向依赖)。
type ScoreBreakdownDTO struct {
	Total              float64            `json:"total"`
	FinalTotal         float64            `json:"finalTotal"`
	Completeness       float64            `json:"completeness"`
	PlacedSessions     int                `json:"placedSessions"`
	ExpectedSessions   int                `json:"expectedSessions"`
	EnabledDimensions  []string           `json:"enabledDimensions,omitempty"`
	Buckets            *ScoreBucketsDTO   `json:"buckets,omitempty"`
	// v0.5.4 兼容:flat 字段
	TeacherPref    float64 `json:"teacherPref"`
	CourseSpacing  float64 `json:"courseSpacing"`
	TeacherDays    float64 `json:"teacherDays"`
	LowFloorPref   float64 `json:"lowFloorPref"`
	WeekendAvoid   float64 `json:"weekendAvoid"`
	PePeriodPref   float64 `json:"pePeriodPref"`
	StudentFatigue float64 `json:"studentFatigue"`
}

// ScoreBucketsDTO 与 services.ScoreBuckets 结构对齐,允许在 dto 包内独立演进。
// nil bucket = 该维度在此快照下被禁用(如 TIME_ONLY 的 Resource)。
type ScoreBucketsDTO struct {
	Time     *ScoreBucketDTO `json:"time,omitempty"`
	Teacher  *ScoreBucketDTO `json:"teacher,omitempty"`
	Student  *ScoreBucketDTO `json:"student,omitempty"`
	Resource *ScoreBucketDTO `json:"resource,omitempty"`
}

type ScoreBucketDTO struct {
	Value   float64            `json:"value"`
	Max     float64            `json:"max"`
	Details map[string]float64 `json:"details,omitempty"`
}
