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

// -----------------------------------------------------------------------------
// v0.5.1 Flexible Span Support (HBUT Vocational Teacher College Adaptation)
//
// The teaching day is divided into three blocks:
//   Morning   : 第1-2节 (period 0-1) + 第3-4节 (period 2-3)  → allowed span {2}
//   Afternoon : 第5-6节 (period 4-5) + 第7-8节 (period 6-7)  → allowed span {2}
//   Evening   : 第9-10-11节         (period 8-9-10)         → allowed span {1,2,3}
//
// Sessions must not cross a block boundary. Span 4 is reserved for future use.
// -----------------------------------------------------------------------------

// PeriodBlock is a coarse-grained time region of the teaching day.
type PeriodBlock int

const (
	BlockUnknown PeriodBlock = iota
	Morning
	Afternoon
	Evening
)

// String returns the Chinese display name for the block.
func (b PeriodBlock) String() string {
	switch b {
	case Morning:
		return "上午"
	case Afternoon:
		return "下午"
	case Evening:
		return "晚上"
	default:
		return "未知"
	}
}

// BlockOfStart maps a starting period to its PeriodBlock.
// Period 0-3 → Morning, 4-7 → Afternoon, 8-10 → Evening.
func BlockOfStart(start Period) PeriodBlock {
	switch {
	case start >= 0 && start <= 3:
		return Morning
	case start >= 4 && start <= 7:
		return Afternoon
	case start >= 8 && start <= 10:
		return Evening
	default:
		return BlockUnknown
	}
}

// IsSpanLegal reports whether a (start, span) combination is a legal HBUT session shape.
//
// Rules:
//  1. span must be in {1, 2, 3}. 4 is reserved for future.
//  2. start+span must not exceed 11 (day has 11 periods, 0..10).
//  3. The session must not cross a block boundary: start and start+span-1
//     must belong to the same PeriodBlock.
//  4. In Morning/Afternoon, only span=2 is allowed, and only at the block-aligned
//     starts (0,2 for Morning; 4,6 for Afternoon) — this preserves the traditional
//     "1-2 / 3-4 / 5-6 / 7-8" two-period pairing used across the college.
//  5. In Evening, span 1/2/3 are all allowed at any start within the block.
func IsSpanLegal(start Period, span int) bool {
	if span < 1 || span > 3 {
		return false
	}
	if start < 0 || int(start)+span > 11 {
		return false
	}
	blockStart := BlockOfStart(start)
	blockEnd := BlockOfStart(start + Period(span) - 1)
	if blockStart == BlockUnknown || blockStart != blockEnd {
		return false
	}
	switch blockStart {
	case Morning:
		return span == 2 && (start == 0 || start == 2)
	case Afternoon:
		return span == 2 && (start == 4 || start == 6)
	case Evening:
		return true
	}
	return false
}

// ValidStartsForSpan returns the set of starting periods legal for a given span,
// under the HBUT block-alignment rules encoded in IsSpanLegal.
//
// Returned slice is a fresh copy; callers may shuffle without side effects.
func ValidStartsForSpan(span int) []Period {
	var out []Period
	for start := Period(0); start <= 10; start++ {
		if IsSpanLegal(start, span) {
			out = append(out, start)
		}
	}
	return out
}
