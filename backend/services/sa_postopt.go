package services

import (
	"scheduling-system/backend/models"
)

// TODO(v0.6.1): SA solver being replaced by pure scheduling/time/ implementation.
// PostOptimize accesses old ScheduleEntry fields (DayOfWeek, StartPeriod, Span,
// TeacherID, TeachingTaskID, etc.) removed in the TA+SE split. Stubbed for compilation.
func (s *SASolver) PostOptimize(
	entries []models.ScheduleEntry,
	teachingTasks []models.TeachingTask,
	teachers []models.Teacher,
	classrooms []models.Classroom,
	lockedSlots []LockedTimeSlot,
	constraints []string,
	topN int,
	timeOnly bool,
) []models.ScheduleEntry {
	_ = teachingTasks
	_ = teachers
	_ = classrooms
	_ = lockedSlots
	_ = constraints
	_ = topN
	_ = timeOnly
	return entries
}
