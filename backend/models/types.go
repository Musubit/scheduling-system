package models

// DayOfWeek represents a day in the teaching week (0 = Monday, 6 = Sunday).
// This is the canonical internal representation throughout the system.
// Display conversion (0→"周一", etc.) is handled by the frontend DAY_NAMES mapping.
type DayOfWeek int

const (
	Mon DayOfWeek = iota
	Tue
	Wed
	Thu
	Fri
	Sat
	Sun
)

// String returns the Chinese display name for the day.
func (d DayOfWeek) String() string {
	names := []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}
	if d < 0 || int(d) >= len(names) {
		return "未知"
	}
	return names[int(d)]
}

// Period represents a teaching period slot (0-indexed internal, 1-indexed for display).
// Period 0 = 第1节, Period 9 = 第10节, etc.
type Period int

// ValidStartPeriods are the valid starting periods for a course.
// Courses typically start at period 0, 2, 4, 6, or 8 (第1/3/5/7/9节).
var ValidStartPeriods = []Period{0, 2, 4, 6, 8}

// DisplayNum returns the 1-indexed display number (0→1, 2→3, etc.).
func (p Period) DisplayNum() int { return int(p) + 1 }

// End returns the period where a span ending at this start period would finish.
func (p Period) End(span int) Period { return p + Period(span) }

// Overlaps returns true if two period ranges overlap.
func (p Period) Overlaps(span int, other Period, otherSpan int) bool {
	end := p + Period(span)
	otherEnd := other + Period(otherSpan)
	return p < otherEnd && other < end
}
