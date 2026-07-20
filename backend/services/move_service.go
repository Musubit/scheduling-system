package services

import (
	"encoding/json"
	"fmt"
	"scheduling-system/backend/database"
	"scheduling-system/backend/models"
	"strconv"
	"strings"
)

// MoveService validates schedule entry moves for drag-and-drop micro-adjustment.
type MoveService struct {
	db database.DB
}

func NewMoveService(db database.DB) *MoveService {
	return &MoveService{db: db}
}

// MoveConflict describes a conflict found when checking a move.
type MoveConflict struct {
	Type        string `json:"type"`        // "teacher" | "room" | "class" | "locked"
	Description string `json:"description"` // Human-readable
	Entity      string `json:"entity"`      // Conflict entity name
	Suggestion  string `json:"suggestion"`  // Optional suggestion
}

// CheckMoveRequest is the input for CheckMove.
type CheckMoveRequest struct {
	EntryID      uint `json:"entryId"`      // the schedule entry being moved
	NewDay       int  `json:"newDay"`       // 0-6
	NewPeriod    int  `json:"newPeriod"`    // 0-10
	NewSpan      int  `json:"newSpan"`      // usually 2
	NewClassroom uint `json:"newClassroom"` // 0 = keep current
}

// CheckMoveResult is the output of the move validation.
type CheckMoveResult struct {
	Valid     bool           `json:"valid"`
	Conflicts []MoveConflict `json:"conflicts"`
}

// MoveAndScoreResult is the output of MoveEntryAndScore.
type MoveAndScoreResult struct {
	Success     bool            `json:"success"`
	Error       string          `json:"error,omitempty"`
	BeforeScore float64         `json:"beforeScore"`
	NewScore    float64         `json:"newScore"`
	Delta       float64         `json:"delta"`
	ScoreDetail *ScoreBreakdown `json:"scoreDetail,omitempty"`
}

// TODO(v0.6.0): Adapt CheckMove for TimeAssignment+ScheduleEntry split model.
// CheckMove currently accesses old ScheduleEntry fields (DayOfWeek, StartPeriod, Span,
// Teacher, TeachingTask, etc.) that no longer exist after the TA+SE model migration.
// Will be properly adapted when move operations are re-implemented against the new model.
func (s *MoveService) CheckMove(req CheckMoveRequest) *CheckMoveResult {
	_ = req
	return &CheckMoveResult{
		Valid: false,
		Conflicts: []MoveConflict{{
			Type:        "error",
			Description: "v0.6.0 migration in progress — move validation temporarily disabled",
		}},
	}
}

// TODO(v0.6.0): Adapt MoveEntry for TimeAssignment+ScheduleEntry split model.
// MoveEntry updates old ScheduleEntry fields (DayOfWeek, StartPeriod, Span) that no
// longer exist after the TA+SE split. Will be properly adapted when move operations
// are re-implemented against the new model (TimeAssignment for time moves, ScheduleEntry
// for room changes).
func (s *MoveService) MoveEntry(req CheckMoveRequest) error {
	_ = req
	return fmt.Errorf("v0.6.0 migration in progress — move operations temporarily disabled")
}

// TODO(v0.6.0): Adapt MoveEntryAndScore for TimeAssignment+ScheduleEntry split model.
// Depends on CheckMove, MoveEntry, and computeScoreDB — all of which access old
// ScheduleEntry fields removed by the TA+SE split. Will be properly adapted in v0.6.1.
func (s *MoveService) MoveEntryAndScore(req CheckMoveRequest) (*MoveAndScoreResult, error) {
	_ = req
	return &MoveAndScoreResult{
		Success: false,
		Error:   "v0.6.0 migration in progress — move-and-score temporarily disabled",
	}, nil
}

// TODO(v0.6.1): getClassIDs/getClassGroupTotalStudents need access to TeachingTask data
// via TimeAssignment (not ScheduleEntry) after the TA+SE split. Stubbed for now.
func (s *MoveService) getClassIDs(entry models.ScheduleEntry) []uint {
	_ = entry
	return nil
}

func (s *MoveService) getClassGroupTotalStudents(entry models.ScheduleEntry) int {
	_ = entry
	return 0
}

// parseWeeksRange parses a Weeks string like "1-16" / "9-16" / "3" into a
// closed integer interval [start, end]. Whitespace tolerated. If the input
// is empty, malformed, or unparseable, returns the permissive full range
// [1, 20] so callers treat "unknown" as "possibly overlaps everything" —
// preserving pre-fix behavior for legacy rows without a Weeks value.
//
// A single number "N" is treated as [N, N].
func parseWeeksRange(w string) (int, int) {
	const fullStart, fullEnd = 1, 20
	s := strings.TrimSpace(w)
	if s == "" {
		return fullStart, fullEnd
	}
	if idx := strings.Index(s, "-"); idx >= 0 {
		a, errA := strconv.Atoi(strings.TrimSpace(s[:idx]))
		b, errB := strconv.Atoi(strings.TrimSpace(s[idx+1:]))
		if errA != nil || errB != nil {
			return fullStart, fullEnd
		}
		if a > b {
			a, b = b, a
		}
		return a, b
	}
	// Single number
	n, err := strconv.Atoi(s)
	if err != nil {
		return fullStart, fullEnd
	}
	return n, n
}

// weeksOverlap reports whether two Weeks strings (e.g. "1-8" and "9-16")
// share at least one teaching week. Used by CheckMove to skip conflicts
// between entries that never coexist within the same week — the schedule
// grid the user sees is week-scoped, so two "same day/period" entries in
// disjoint week ranges are not a real conflict.
//
// Malformed / empty Weeks defaults to the full teaching range [1,20] —
// preserving pre-fix behavior for legacy rows that lack a Weeks value.
func weeksOverlap(a, b string) bool {
	as, ae := parseWeeksRange(a)
	bs, be := parseWeeksRange(b)
	return as <= be && bs <= ae
}

// TODO(v0.6.0): Adapt computeScore/computeScoreDB for TimeAssignment+ScheduleEntry split.
// The scoring pipeline (ScoreSchedule) is being replaced by the 4-bucket scorer in v0.6.1.
func (s *MoveService) computeScore(semesterID uint) (*ScoreBreakdown, error) {
	_ = semesterID
	return &ScoreBreakdown{}, nil
}

// computeScoreDB is the transaction-aware variant of computeScore.
// TODO(v0.6.0): will be properly adapted in v0.6.1.
func (s *MoveService) computeScoreDB(db database.DB, semesterID uint) (*ScoreBreakdown, error) {
	_ = db
	_ = semesterID
	return &ScoreBreakdown{}, nil
}

// loadLatestConstraints reads the constraint list from the most recent
// auto-snapshot for the given semester. Falls back to FullDefaultConstraints
// when no snapshot exists.
func (s *MoveService) loadLatestConstraints(semesterID uint) []string {
	return s.loadLatestConstraintsDB(s.db, semesterID)
}

// loadLatestConstraintsDB is the transaction-aware variant of loadLatestConstraints.
func (s *MoveService) loadLatestConstraintsDB(db database.DB, semesterID uint) []string {
	var ver models.ScheduleVersion
	if err := db.Where("semester_id = ?", semesterID).
		Order("created_at DESC").First(&ver).Error(); err == nil && ver.EnabledConstraints != "" {
		var constraints []string
		if err := json.Unmarshal([]byte(ver.EnabledConstraints), &constraints); err == nil && len(constraints) > 0 {
			return constraints
		}
	}
	return FullDefaultConstraints()
}

// buildSportsCourseIDs identifies PE course IDs from teaching tasks.
// A course is considered a sports course when its name contains "体育".
func (s *MoveService) buildSportsCourseIDs(teachingTasks []models.TeachingTask) map[uint]bool {
	ids := make(map[uint]bool)
	for _, tt := range teachingTasks {
		if tt.CourseID > 0 && models.IsSportsCourse(tt.Course.Name) {
			ids[tt.CourseID] = true
		}
	}
	return ids
}
