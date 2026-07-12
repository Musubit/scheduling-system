package services

// v0.5.2 Goal 3: incremental score cache for the SA hot path.
//
// This file implements a delta-score cache maintained on schedulingContext.
// It mirrors ScoreSchedule's semantics exactly for the seven soft categories
// while allowing tryMove/trySwap to update in O(1..few) rather than
// rescanning every entry each iteration.
//
// Correctness contract:
//   scoreFromCache() must return the same FinalTotal as
//   ScoreSchedule(ctx.entries, ...).FinalTotal (up to float rounding).
//   Enforced by TestDeltaScoreMatchesFullScore golden test.
//
// Symmetry contract:
//   applyEntryDelta(+1, e) followed by applyEntryDelta(-1, e) must restore
//   the cache identity. Used to implement undoNeighbor.

import (
	"scheduling-system/backend/models"
)

// scoreCache holds all statistics needed to reconstruct the seven soft-constraint
// subscores without rescanning ctx.entries. Updated incrementally in
// applyEntryDelta so tryMove/trySwap avoid O(N) work per iteration.
type scoreCache struct {
	// Per-teacher: early/late counts + total, plus which days they teach on.
	teacherEarly map[uint]int
	teacherLate  map[uint]int
	teacherTotal map[uint]int
	// teacherDayCount[teacherID][day] = number of sessions on that day.
	teacherDayCount map[uint]*[7]int

	// Per-teacher floor stats (sum of floors + count) for low_floor scoring.
	teacherFloorSum   map[uint]float64
	teacherFloorCount map[uint]int

	// Per-course: day → session count. Drives course_dispersed scoring.
	courseDayCount map[uint]*[7]int

	// Weekend counts (Sat/Sun) + total entries — for avoid_saturday/sunday.
	weekendSat int
	weekendSun int
	totalEntries int

	// PE (sports) counts: total + at preferred starts {2,6}.
	peTotal     int
	peAtPref    int

	// Per-classGroup×day: occupied-period bitmask (11 bits for periods 0..10).
	// Drives student_fatigue: max consecutive periods across all class-day pairs.
	classDayBits map[uint]*[7]uint16

	// Cached max floor across all classrooms (constant for a Solve run).
	maxFloor int

	// Cached teacher lookup for floor prefs and preferNoEarly/Late checks.
	teacherByID   map[uint]*models.Teacher
	classroomByID map[uint]*models.Classroom
	// Task → class group IDs (for student fatigue accounting).
	taskClassIDs map[uint][]uint
}
