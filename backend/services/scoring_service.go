package services

// scoring_service.go — 评分类型定义。
//
// ScoreBreakdown / ScoreBucket / ScoreBuckets 是 services 包内部的
// 评分结果类型，用于持久化和展示。排课引擎的评分由调度层的 IScorer 接口
// （scheduling/score/ 包实现 4-bucket 评分器）负责计算。
//
// ScoreGreater / ScoreEquals 是比较两个评分的辅助函数。

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

// ScoreBreakdown holds the detailed scoring result.
type ScoreBreakdown struct {
	Total         float64 `json:"total"`
	TeacherPref   float64 `json:"teacherPref"`
	CourseSpacing float64 `json:"courseSpacing"`
	TeacherDays   float64 `json:"teacherDays"`
	LowFloorPref  float64 `json:"lowFloorPref"`
	WeekendAvoid  float64 `json:"weekendAvoid"`
	PePeriodPref  float64 `json:"pePeriodPref"`
	StudentFatigue       float64            `json:"studentFatigue"`
	PerCategoryMax       float64            `json:"perCategoryMax"`
	EnabledCategoryCount int                `json:"enabledCategoryCount"`
	CategoryMaxes        map[string]float64 `json:"categoryMaxes,omitempty"`

	PlacedSessions   int     `json:"placedSessions,omitempty"`
	ExpectedSessions int     `json:"expectedSessions,omitempty"`
	Completeness     float64 `json:"completeness,omitempty"`
	FinalTotal       float64 `json:"finalTotal"`

	Buckets *ScoreBuckets `json:"buckets,omitempty"`

	EnabledDimensions []string `json:"enabledDimensions,omitempty"`
}

// ScoreBuckets 按 spec 2.7 冻结的四桶结构表达当前排课结果分值。
type ScoreBuckets struct {
	Time     *ScoreBucket `json:"time,omitempty"`
	Teacher  *ScoreBucket `json:"teacher,omitempty"`
	Student  *ScoreBucket `json:"student,omitempty"`
	Resource *ScoreBucket `json:"resource,omitempty"`
}

// ScoreBucket 一个维度的评分聚合。
type ScoreBucket struct {
	Value   float64            `json:"value"`
	Max     float64            `json:"max"`
	Details map[string]float64 `json:"details,omitempty"`
}

// TeacherWorkloadInfo holds per-teacher workload analysis data.
type TeacherWorkloadInfo struct {
	TeacherID         uint    `json:"teacherId"`
	TeacherName       string  `json:"teacherName"`
	TotalSessions     int     `json:"totalSessions"`
	DailyDistribution []int   `json:"dailyDistribution"`
	BusyDays          int     `json:"busyDays"`
	MaxDaily          int     `json:"maxDaily"`
	MinDaily          int     `json:"minDaily"`
	BalanceScore      float64 `json:"balanceScore"`
	Suggestion        string  `json:"suggestion"`
}
