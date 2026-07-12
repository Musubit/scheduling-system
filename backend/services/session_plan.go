package services

// v0.5.1 Session Plan — HBUT flexible span support.
//
// Maps a teaching task's weekly hour budget onto one or more session shapes.
// A "session shape" here is just its span (1/2/3). The concrete (day, start,
// room) placement remains the solver's responsibility; this file only decides
// "how many sessions per week and how many periods each session covers".
//
// This is the single source of truth for span/session-count resolution across
// the Go SA solver and the OR-Tools bridge. solver.py mirrors the same rules
// in is_span_legal + build_session_spans and must stay in sync.

// SessionPlan describes the weekly session shapes for one teaching task.
type SessionPlan struct {
	// Spans is the span of each weekly session, ordered. len(Spans) = sessions per week.
	// e.g. [2,2] for a 4-hour theory course; [3] for a 3-hour evening course.
	Spans []int
}

// SessionsPerWeek is the length of Spans — number of weekly sessions.
func (p SessionPlan) SessionsPerWeek() int { return len(p.Spans) }

// resolveSessionPlan derives a SessionPlan from a teaching task.
//
// Inputs consulted (in priority order):
//  1. task.PreferredSpan — if >0 and produces a valid plan, wins.
//  2. courseHours + week span (from task.StartWeek/EndWeek) — hour-derived default.
//  3. task.MaxHoursPerWeek — clamps total periods per week if >0.
//
// Output invariants:
//   - Every entry in Spans is in {1,2,3}.
//   - len(Spans) is in [1, 4] (cap 4 sessions per week — v0.4 behavior).
//   - Sum(Spans) approximates weekly hours, biased by preferred-span when set.
//
// This function does NOT check whether legal starting periods exist for the
// chosen span — that's the solver's job. When Evening-only spans (=3) are
// preferred, callers should filter starts via IsSpanLegal.
func resolveSessionPlan(courseHours, startWeek, endWeek, maxHoursPerWeek, preferredSpan int) SessionPlan {
	weeks := endWeek - startWeek + 1
	if weeks < 1 {
		weeks = 1
	}
	if courseHours <= 0 {
		// Default: 1 session, 2 periods (matches pre-v0.5.1 behavior when hours unknown)
		return SessionPlan{Spans: []int{2}}
	}

	// Round-half-up weekly hours.
	// Using integer math: weeklyHours ≈ ceil(courseHours / weeks) is too generous;
	// nearest-int matches the v0.4 ceil(x/2.0) rounding used previously.
	// weeklyHoursR = round(courseHours / weeks)
	weeklyHoursR := (courseHours + weeks/2) / weeks
	if weeklyHoursR < 1 {
		weeklyHoursR = 1
	}

	// Cap by maxHoursPerWeek if set.
	if maxHoursPerWeek > 0 && weeklyHoursR > maxHoursPerWeek {
		weeklyHoursR = maxHoursPerWeek
	}

	// Path A: honored PreferredSpan.
	if preferredSpan >= 1 && preferredSpan <= 3 {
		return planFromPreferredSpan(weeklyHoursR, preferredSpan)
	}

	// Path B: hour-derived defaults per Spec §六 Goal 3.
	return planFromWeeklyHours(weeklyHoursR)
}

// planFromWeeklyHours implements the Spec §六 Goal 3 default table:
//
//	weeklyHours=1 → [1]      (rare, evening-only)
//	weeklyHours=2 → [2]      (most common)
//	weeklyHours=3 → [3]      (evening 9-11 三节连排)
//	weeklyHours=4 → [2,2]    (two morning/afternoon slots)
//	weeklyHours=5 → [2,2,1]  (rare mixed)
//	weeklyHours=6 → [2,2,2]
//	weeklyHours=7 → [2,2,2,1] (capped at 4 sessions)
//	weeklyHours≥8 → [2,2,2,2] (cap 4, some hours unschedulable — surfaces as diagnostics)
func planFromWeeklyHours(weeklyHours int) SessionPlan {
	if weeklyHours <= 0 {
		return SessionPlan{Spans: []int{2}}
	}
	switch weeklyHours {
	case 1:
		return SessionPlan{Spans: []int{1}}
	case 2:
		return SessionPlan{Spans: []int{2}}
	case 3:
		return SessionPlan{Spans: []int{3}}
	case 4:
		return SessionPlan{Spans: []int{2, 2}}
	case 5:
		return SessionPlan{Spans: []int{2, 2, 1}}
	case 6:
		return SessionPlan{Spans: []int{2, 2, 2}}
	case 7:
		return SessionPlan{Spans: []int{2, 2, 2, 1}}
	default:
		return SessionPlan{Spans: []int{2, 2, 2, 2}}
	}
}

// planFromPreferredSpan builds a plan by repeatedly fitting `span` into weeklyHours.
// If a remainder < span exists, one shorter session absorbs it (still legal span 1/2/3).
// Result is capped at 4 sessions per week.
func planFromPreferredSpan(weeklyHours, span int) SessionPlan {
	if weeklyHours <= 0 {
		return SessionPlan{Spans: []int{span}}
	}
	spans := []int{}
	remaining := weeklyHours
	for remaining >= span && len(spans) < 4 {
		spans = append(spans, span)
		remaining -= span
	}
	if remaining > 0 && remaining <= 3 && len(spans) < 4 {
		spans = append(spans, remaining)
	}
	if len(spans) == 0 {
		// weeklyHours < span (e.g. prefer=3 but hours=2). Fall back to one shorter session.
		spans = []int{weeklyHours}
		if spans[0] > 3 {
			spans[0] = 3
		}
	}
	return SessionPlan{Spans: spans}
}
