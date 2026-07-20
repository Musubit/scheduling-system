package services

import (
	"scheduling-system/backend/models"
)

// TODO(v0.6.1): SA solver being replaced by pure scheduling/time/ implementation.
// buildInitial is stubbed — it creates models.ScheduleEntry with old fields
// (CourseID, TeacherID, DayOfWeek, etc.) that no longer exist after TA+SE split.
func (ctx *schedulingContext) buildInitial() {
	ctx.entries = nil
	ctx.roomOcc = make(map[uint64]bool)
	ctx.teacherOcc = make(map[uint64]bool)
	ctx.classOcc = make(map[uint64]bool)
}

// toIntSlice converts a []models.Period to []int for internal solver loops.
func toIntSlice(ps []models.Period) []int {
	out := make([]int, len(ps))
	for i, p := range ps {
		out[i] = int(p)
	}
	return out
}

// TODO(v0.6.1): SA solver being replaced by pure scheduling/time/ implementation.
// tryPlaceSession is stubbed — it creates models.ScheduleEntry with old fields
// (CourseID, TeacherID, TeachingTaskID, DayOfWeek, StartPeriod, Span, Weeks)
// that no longer exist after TA+SE split.
func (ctx *schedulingContext) tryPlaceSession(td teachingTaskData, days []int, starts []int, span int, enforcePref bool) bool {
	_ = td
	_ = days
	_ = starts
	_ = span
	_ = enforcePref
	return false
}

// Neighbor operation types
