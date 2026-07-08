package services

import (
	"fmt"
	"scheduling-system/database"
	"scheduling-system/models"
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
	if err := s.db.Preload("Course").Preload("Teacher").Preload("Classroom").
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

	// Load locked slots
	var lockedSlots []lockedTimeSlot
	slots := (&SchedulingService{db: s.db}).loadLockedSlots()
	lockedSlots = slots

	// 1. Check locked time slots
	for _, ls := range lockedSlots {
		if int(ls.DayOfWeek) == req.NewDay {
			if periodsOverlapInt(req.NewPeriod, span, int(ls.StartPeriod), ls.Span) {
				result.Valid = false
				result.Conflicts = append(result.Conflicts, MoveConflict{
					Type:        "locked",
					Description: fmt.Sprintf("该时段为全校锁定时间（周%s %d-%d节）",
						models.DayOfWeek(req.NewDay).String(),
						ls.StartPeriod.DisplayNum(),
						int(ls.StartPeriod)+ls.Span),
					Entity: "系统设置",
				})
			}
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
		if periodsOverlapInt(req.NewPeriod, span, int(other.StartPeriod), other.Span) {
			result.Valid = false
			result.Conflicts = append(result.Conflicts, MoveConflict{
				Type:        "teacher",
				Description: fmt.Sprintf("%s在周%s %d-%d节已有课程",
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
		if periodsOverlapInt(req.NewPeriod, span, int(other.StartPeriod), other.Span) {
			var room models.Classroom
			s.db.First(&room, newRoomID)
			result.Valid = false
			result.Conflicts = append(result.Conflicts, MoveConflict{
				Type:        "room",
				Description: fmt.Sprintf("%s在周%s %d-%d节已被占用",
					room.Name,
					models.DayOfWeek(req.NewDay).String(),
					other.StartPeriod.DisplayNum(),
					int(other.StartPeriod)+other.Span),
				Entity: room.Name,
			})
		}
	}

	// 4. Check class group conflict
	if entry.ClassGroupID != nil {
		for _, other := range others {
			if other.ClassGroupID == nil || *other.ClassGroupID != *entry.ClassGroupID {
				continue
			}
			if int(other.DayOfWeek) != req.NewDay {
				continue
			}
			if periodsOverlapInt(req.NewPeriod, span, int(other.StartPeriod), other.Span) {
				var cg models.ClassGroup
				s.db.First(&cg, *entry.ClassGroupID)
				result.Valid = false
				result.Conflicts = append(result.Conflicts, MoveConflict{
					Type:        "class",
					Description: fmt.Sprintf("%s在周%s %d-%d节已有课程",
						cg.Name,
						models.DayOfWeek(req.NewDay).String(),
						other.StartPeriod.DisplayNum(),
						int(other.StartPeriod)+other.Span),
					Entity: cg.Name,
				})
			}
		}
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
