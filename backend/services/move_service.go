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

// CheckMove validates whether a schedule entry can be moved to a new time/room.
func (s *MoveService) CheckMove(req CheckMoveRequest) *CheckMoveResult {
	result := &CheckMoveResult{Valid: true}

	// Load the entry being moved
	var entry models.ScheduleEntry
	if err := s.db.Preload("Course").Preload("Teacher").Preload("Classroom").Preload("TeachingTask").
		First(&entry, req.EntryID).Error(); err != nil {
		result.Valid = false
		result.Conflicts = append(result.Conflicts, MoveConflict{
			Type: "error", Description: fmt.Sprintf("课表条目不存在: ID=%d", req.EntryID),
		})
		return result
	}

	// Determine new classroom
	newRoomID := entry.ClassroomID
	if req.NewClassroom > 0 {
		newRoomID = req.NewClassroom
	}

	span := entry.Span
	if req.NewSpan > 0 {
		span = req.NewSpan
	}

	// Load all other entries for the same semester (excluding this one)
	var others []models.ScheduleEntry
	s.db.Where("semester = ? AND id != ?", entry.Semester, entry.ID).Find(&others)

	// Load locked slots via shared package-level function
	lockedSlots := loadLockedSlotsDB(s.db)

		// 1. Check locked time slots
		for _, ls := range lockedSlots {
			if int(ls.DayOfWeek) == req.NewDay {
				if periodsOverlapInt(req.NewPeriod, span, int(ls.StartPeriod), ls.Span) {
					result.Valid = false
					result.Conflicts = append(result.Conflicts, MoveConflict{
						Type:        "locked",
						Description: fmt.Sprintf("该时段为全校锁定时间（%s %d-%d节）",
							models.DayOfWeek(req.NewDay).String(),
							ls.StartPeriod.DisplayNum(),
							int(ls.StartPeriod)+ls.Span),
						Entity: "系统设置",
					})
				}
			}
		}

		// 1b. Check teacher unavailable slots
		if entry.Teacher.ID > 0 && entry.Teacher.UnavailableSlots != "" {
			var unavailSlots []LockedTimeSlot
			if err := json.Unmarshal([]byte(entry.Teacher.UnavailableSlots), &unavailSlots); err == nil {
				for _, u := range unavailSlots {
					if int(u.DayOfWeek) == req.NewDay && periodsOverlapInt(req.NewPeriod, span, int(u.StartPeriod), u.Span) {
						result.Valid = false
						result.Conflicts = append(result.Conflicts, MoveConflict{
							Type:        "teacher",
							Description: fmt.Sprintf("%s在%s的%d-%d节有不可用时间设置",
								entry.Teacher.Name,
								models.DayOfWeek(req.NewDay).String(),
								u.StartPeriod.DisplayNum(),
								int(u.StartPeriod)+u.Span),
							Entity: entry.Teacher.Name,
						})
					}
				}
			}
		}

		// 1c. Check room capacity
		if req.NewClassroom > 0 {
			var newRoom models.Classroom
			s.db.First(&newRoom, req.NewClassroom)
			totalStudents := s.getClassGroupTotalStudents(entry)
			if totalStudents > 0 && newRoom.Capacity < totalStudents {
				result.Valid = false
				result.Conflicts = append(result.Conflicts, MoveConflict{
					Type:        "room",
					Description: fmt.Sprintf("%s容量不足（需%d人，仅%d座）", newRoom.Name, totalStudents, newRoom.Capacity),
					Entity:      newRoom.Name,
				})
			}
		}

		// 2. Check teacher conflict
	for _, other := range others {
		if other.TeacherID != entry.TeacherID {
			continue
		}
		if int(other.DayOfWeek) != req.NewDay {
			continue
		}
		// Skip entries whose Weeks range never overlaps with the moved entry —
		// they never coexist in the same teaching week and cannot conflict.
		if !weeksOverlap(entry.Weeks, other.Weeks) {
			continue
		}
		if periodsOverlapInt(req.NewPeriod, span, int(other.StartPeriod), other.Span) {
			result.Valid = false
			result.Conflicts = append(result.Conflicts, MoveConflict{
				Type:        "teacher",
				Description: fmt.Sprintf("%s在%s %d-%d节已有课程",
					entry.Teacher.Name,
					models.DayOfWeek(req.NewDay).String(),
					other.StartPeriod.DisplayNum(),
					int(other.StartPeriod)+other.Span),
				Entity: entry.Teacher.Name,
			})
		}
	}

	// 3. Check room conflict
	for _, other := range others {
		if other.ClassroomID != newRoomID {
			continue
		}
		if int(other.DayOfWeek) != req.NewDay {
			continue
		}
		if !weeksOverlap(entry.Weeks, other.Weeks) {
			continue
		}
		if periodsOverlapInt(req.NewPeriod, span, int(other.StartPeriod), other.Span) {
			var room models.Classroom
			s.db.First(&room, newRoomID)
			result.Valid = false
			result.Conflicts = append(result.Conflicts, MoveConflict{
				Type:        "room",
				Description: fmt.Sprintf("%s在%s %d-%d节已被占用",
					room.Name,
					models.DayOfWeek(req.NewDay).String(),
					other.StartPeriod.DisplayNum(),
					int(other.StartPeriod)+other.Span),
				Entity: room.Name,
			})
		}
	}

	// 4. Check class group conflict
	// Check using TeachingTask if available, otherwise fall back to ClassGroupID
	var entryClassIDs []uint
	if entry.TeachingTaskID != nil && entry.TeachingTask != nil {
		var ttClasses []models.TeachingTaskClass
		s.db.Where("teaching_task_id = ?", *entry.TeachingTaskID).Find(&ttClasses)
		for _, tc := range ttClasses {
			entryClassIDs = append(entryClassIDs, tc.ClassGroupID)
		}
	} else if entry.ClassGroupID != nil {
		entryClassIDs = append(entryClassIDs, *entry.ClassGroupID)
	}

	for _, cid := range entryClassIDs {
		for _, other := range others {
			// Check if other entry shares any class group
			otherClassIDs := s.getClassIDs(other)
			for _, otherCID := range otherClassIDs {
				if otherCID != cid {
					continue
				}
				if int(other.DayOfWeek) != req.NewDay {
					continue
				}
				if !weeksOverlap(entry.Weeks, other.Weeks) {
					continue
				}
				if periodsOverlapInt(req.NewPeriod, span, int(other.StartPeriod), other.Span) {
					var cg models.ClassGroup
					s.db.First(&cg, cid)
					result.Valid = false
					result.Conflicts = append(result.Conflicts, MoveConflict{
						Type:        "class",
						Description: fmt.Sprintf("%s在%s %d-%d节已有课程",
							cg.Name,
							models.DayOfWeek(req.NewDay).String(),
							other.StartPeriod.DisplayNum(),
							int(other.StartPeriod)+other.Span),
						Entity: cg.Name,
					})
					goto nextClass
				}
			}
		}
	nextClass:
	}

	return result
}

// MoveEntry executes a validated move by updating the schedule entry.
func (s *MoveService) MoveEntry(req CheckMoveRequest) error {
	var entry models.ScheduleEntry
	if err := s.db.First(&entry, req.EntryID).Error(); err != nil {
		return fmt.Errorf("课表条目不存在: %w", err)
	}

	entry.DayOfWeek = models.DayOfWeek(req.NewDay)
	entry.StartPeriod = models.Period(req.NewPeriod)
	if req.NewSpan > 0 {
		entry.Span = req.NewSpan
	}
	if req.NewClassroom > 0 {
		entry.ClassroomID = req.NewClassroom
	}

	return s.db.Save(&entry).Error()
}

// getClassIDs returns all class group IDs associated with a schedule entry.
func (s *MoveService) getClassIDs(entry models.ScheduleEntry) []uint {
	if entry.TeachingTaskID != nil {
		var ttClasses []models.TeachingTaskClass
		s.db.Where("teaching_task_id = ?", *entry.TeachingTaskID).Find(&ttClasses)
		ids := make([]uint, len(ttClasses))
		for i, tc := range ttClasses {
			ids[i] = tc.ClassGroupID
		}
		return ids
	}
	if entry.ClassGroupID != nil {
		return []uint{*entry.ClassGroupID}
	}
	return nil
}

// getClassGroupTotalStudents returns the total number of students across all class groups
// associated with a schedule entry.
func (s *MoveService) getClassGroupTotalStudents(entry models.ScheduleEntry) int {
	cgIDs := s.getClassIDs(entry)
	if len(cgIDs) == 0 {
		return 0
	}
	var total int
	for _, cgID := range cgIDs {
		var cg models.ClassGroup
		if err := s.db.First(&cg, cgID).Error(); err == nil {
			total += cg.Students
		}
	}
	return total
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
