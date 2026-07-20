package services

import (
	"scheduling-system/backend/models"
)

// TODO(v0.6.1): Replace with scheduling/score/ 4-bucket scorer.
// The current scoring service is deeply coupled to old ScheduleEntry fields
// (DayOfWeek, StartPeriod, Span, TeacherID, CourseID, TeachingTaskID, etc.)
// that were removed in the TA+SE model split. All scoring methods are stubbed
// for v0.6.0 compilation; the proper 4-bucket scorer will replace this entire
// file in v0.6.1 (Task 10).

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

// ScoreSchedule evaluates a full schedule against soft constraints.
// TODO(v0.6.1): Replace with scheduling/score/ 4-bucket scorer.
// Stubbed for v0.6.0 — returns zero ScoreBreakdown.
func (s *ScoringService) ScoreSchedule(entries []models.ScheduleEntry, teachers []models.Teacher, classrooms []models.Classroom, ctx ScoringContext) ScoreBreakdown {
	_ = entries
	_ = teachers
	_ = classrooms
	_ = ctx
	return ScoreBreakdown{}
}

// AnalyzeTeacherWorkload computes per-teacher workload balance from schedule entries.
// TODO(v0.6.1): Rebuild for TimeAssignment+ScheduleEntry split model.
// Stubbed for v0.6.0 — returns empty slice.
func (s *ScoringService) AnalyzeTeacherWorkload(entries []models.ScheduleEntry, teachers []models.Teacher) []TeacherWorkloadInfo {
	_ = entries
	_ = teachers
	return nil
}
