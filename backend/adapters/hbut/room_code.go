// Package hbut provides HBUT (Hubei University of Technology) specific
// adapters for interpreting institution-conventional identifiers.
//
// This package is an ADAPTER layer — it encodes HBUT's local rules for
// classroom codes, building partitions, and lab-building conventions.
// The Core Domain (backend/models) is deliberately unaware of these
// rules; anything school-specific belongs here.
//
// Import invariants:
//   - hbut MAY import backend/models (only for RoomType constants, used
//     as string values in RoomTypeHint output).
//   - hbut MUST NOT import backend/services, backend/database, or
//     backend/adapters/<other-schools>/.
//   - No package under backend/models or backend/services (except
//     explicitly permitted call sites like Excel import / UI hints) may
//     import backend/adapters/hbut.
//
// HBUT room code rules (as of 2026-07):
//   - Codes take the shape "<BuildingCode>-<RoomNumber>", separated by a
//     single "-".
//   - BuildingCode may include letters denoting a partition ("5A", "7B").
//   - The first digit after "-" is the floor number.
//   - Codes with floor == 0 (i.e., "1-001", "7B-001") mark auditorium /
//     lecture-hall style rooms located on ground floor.
//   - BuildingCode starting with "6" is the lab building; rooms there
//     default to RoomType = LAB.
//   - RoomTypeHint precedence: floor == 0 (LECTURE) > 6-prefix (LAB) >
//     "" (no hint).
package hbut

import (
	"fmt"
	"strings"

	"scheduling-system/backend/models"
)

// ParsedRoomCode is the structured breakdown of an HBUT room code.
//
// Semantics of empty/zero fields:
//   - BuildingCode == "" means the code had no "-" separator; RoomNumber
//     holds the raw input for caller fallback logic.
//   - Floor == 0 with RoomTypeHint == LECTURE is a normal auditorium.
//   - Floor == 0 with RoomTypeHint != LECTURE indicates unparseable
//     floor digit; Warnings will contain the reason.
//   - Confidence in [0.0, 1.0]. A fully well-formed code (e.g. "1-302")
//     returns 1.0; partial matches lower the value proportionally.
type ParsedRoomCode struct {
	BuildingCode string   // e.g. "1", "5A", "6A", "7B"
	Floor        int      // e.g. 3 for "1-302", 0 for "1-001"
	RoomNumber   string   // room number part sans building prefix ("302", "001", "422")
	RoomTypeHint string   // one of models.RoomType* or ""; caller may override
	Confidence   float64  // 0.0 (unparseable) to 1.0 (fully well-formed)
	Warnings     []string // non-fatal issues; never blocks the caller
}

// Parse interprets an HBUT classroom code and returns a best-effort
// ParsedRoomCode. It never returns an error — problematic inputs are
// surfaced through Warnings so upstream import flows are not blocked.
//
// Examples:
//
//	"1-302"  → {BuildingCode:"1",  Floor:3, RoomNumber:"02",  RoomTypeHint:""}
//	"1-001"  → {BuildingCode:"1",  Floor:0, RoomNumber:"01",  RoomTypeHint:LECTURE}
//	"5A-201" → {BuildingCode:"5A", Floor:2, RoomNumber:"01",  RoomTypeHint:""}
//	"6A-422" → {BuildingCode:"6A", Floor:4, RoomNumber:"22",  RoomTypeHint:LAB}
//	"6A-213" → {BuildingCode:"6A", Floor:2, RoomNumber:"13",  RoomTypeHint:LAB}
//	"7A-303" → {BuildingCode:"7A", Floor:3, RoomNumber:"03",  RoomTypeHint:""}
//	"7B-301" → {BuildingCode:"7B", Floor:3, RoomNumber:"01",  RoomTypeHint:""}
//
// Edge cases:
//
//	""       → Confidence=0, Warnings=["empty code"]
//	"foobar" → BuildingCode="foobar", Warnings=["missing '-' separator"], Confidence=0.2
//	"1-"     → BuildingCode="1", Warnings=["missing room number"], Confidence=0.3
//	"1-abc"  → BuildingCode="1", RoomNumber="abc", Warnings=["floor digit not numeric"], Confidence=0.4
func Parse(code string) ParsedRoomCode {
	code = strings.TrimSpace(code)
	if code == "" {
		return ParsedRoomCode{
			Warnings:   []string{"empty code"},
			Confidence: 0,
		}
	}

	// Split on the first "-". HBUT convention allows only one separator.
	idx := strings.Index(code, "-")
	if idx < 0 {
		return ParsedRoomCode{
			BuildingCode: code,
			RoomNumber:   "",
			Warnings:     []string{"missing '-' separator"},
			Confidence:   0.2,
		}
	}

	buildingCode := strings.TrimSpace(code[:idx])
	rest := strings.TrimSpace(code[idx+1:])

	result := ParsedRoomCode{
		BuildingCode: buildingCode,
		Confidence:   1.0,
	}

	if buildingCode == "" {
		result.Warnings = append(result.Warnings, "missing building code")
		result.Confidence -= 0.5
	}

	if rest == "" {
		result.Warnings = append(result.Warnings, "missing room number")
		result.Confidence = 0.3
		// Still allow the 6-prefix / partition hint to fire below.
		result.RoomTypeHint = classifyByBuildingCode(buildingCode)
		return result
	}

	// First rune after "-" is the floor digit (HBUT convention).
	floorRune := rest[0]
	if floorRune < '0' || floorRune > '9' {
		result.RoomNumber = rest
		result.Warnings = append(result.Warnings, fmt.Sprintf("floor digit not numeric: %q", string(floorRune)))
		result.Confidence = 0.4
		// Still apply building-code hint even without a floor.
		result.RoomTypeHint = classifyByBuildingCode(buildingCode)
		return result
	}
	result.Floor = int(floorRune - '0')
	result.RoomNumber = rest[1:]

	// Hint precedence: LECTURE (floor 0) > LAB (6-prefix) > none.
	// Rationale: an auditorium in the lab building is still an auditorium.
	if result.Floor == 0 {
		result.RoomTypeHint = models.RoomTypeLecture
	} else {
		result.RoomTypeHint = classifyByBuildingCode(buildingCode)
	}

	return result
}

// classifyByBuildingCode maps a BuildingCode prefix to a RoomType hint.
// Only building-level defaults live here; floor-level rules (e.g. floor
// zero → LECTURE) are handled by the caller in Parse.
//
// HBUT convention: buildings whose code starts with "6" are the lab
// building complex (6A, 6B, 6C, …); everything inside defaults to LAB
// unless overridden by a stronger signal like floor == 0.
func classifyByBuildingCode(buildingCode string) string {
	if buildingCode == "" {
		return ""
	}
	if strings.HasPrefix(buildingCode, "6") {
		return models.RoomTypeLab
	}
	return ""
}

// BuildingPartition strips any trailing letter partition ("A", "B", …)
// from a BuildingCode, returning the numeric base and the partition
// suffix separately. This is useful when grouping rooms across
// partitions of the same physical building (5A + 5B → base "5").
//
// Examples:
//
//	"1"  → ("1",  "")
//	"5A" → ("5",  "A")
//	"7B" → ("7",  "B")
//	"6A" → ("6",  "A")
//	""   → ("",   "")
//	"XA" → ("X",  "A")  // non-numeric base still returned as-is
func BuildingPartition(buildingCode string) (base, partition string) {
	if buildingCode == "" {
		return "", ""
	}
	// Trailing uppercase letter denotes a partition per HBUT convention.
	last := buildingCode[len(buildingCode)-1]
	if last >= 'A' && last <= 'Z' {
		return buildingCode[:len(buildingCode)-1], string(last)
	}
	return buildingCode, ""
}
