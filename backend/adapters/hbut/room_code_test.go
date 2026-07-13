package hbut

import (
	"strings"
	"testing"

	"scheduling-system/backend/models"
)

// ============ Parse — happy path ============

func TestParse_TypicalCodes(t *testing.T) {
	cases := []struct {
		code            string
		wantBuilding    string
		wantFloor       int
		wantRoomNumber  string
		wantHint        string
		wantConfidence  float64
		wantNoWarnings  bool
	}{
		{"1-302", "1", 3, "02", "", 1.0, true},
		{"5A-201", "5A", 2, "01", "", 1.0, true},
		{"6A-422", "6A", 4, "22", models.RoomTypeLab, 1.0, true},
		{"6A-213", "6A", 2, "13", models.RoomTypeLab, 1.0, true},
		{"7A-303", "7A", 3, "03", "", 1.0, true},
		{"7B-301", "7B", 3, "01", "", 1.0, true},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			got := Parse(c.code)
			if got.BuildingCode != c.wantBuilding {
				t.Errorf("BuildingCode = %q, want %q", got.BuildingCode, c.wantBuilding)
			}
			if got.Floor != c.wantFloor {
				t.Errorf("Floor = %d, want %d", got.Floor, c.wantFloor)
			}
			if got.RoomNumber != c.wantRoomNumber {
				t.Errorf("RoomNumber = %q, want %q", got.RoomNumber, c.wantRoomNumber)
			}
			if got.RoomTypeHint != c.wantHint {
				t.Errorf("RoomTypeHint = %q, want %q", got.RoomTypeHint, c.wantHint)
			}
			if got.Confidence != c.wantConfidence {
				t.Errorf("Confidence = %f, want %f", got.Confidence, c.wantConfidence)
			}
			if c.wantNoWarnings && len(got.Warnings) > 0 {
				t.Errorf("Warnings = %v, want none", got.Warnings)
			}
		})
	}
}

// ============ Parse — LECTURE hint (floor 0) ============

func TestParse_FloorZero_LectureHint(t *testing.T) {
	cases := []struct {
		code         string
		wantBuilding string
		wantHint     string
	}{
		{"1-001", "1", models.RoomTypeLecture},
		{"7B-001", "7B", models.RoomTypeLecture},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			got := Parse(c.code)
			if got.Floor != 0 {
				t.Errorf("Floor = %d, want 0", got.Floor)
			}
			if got.BuildingCode != c.wantBuilding {
				t.Errorf("BuildingCode = %q, want %q", got.BuildingCode, c.wantBuilding)
			}
			if got.RoomTypeHint != c.wantHint {
				t.Errorf("RoomTypeHint = %q, want %q", got.RoomTypeHint, c.wantHint)
			}
			if got.Confidence != 1.0 {
				t.Errorf("Confidence = %f, want 1.0", got.Confidence)
			}
		})
	}
}

// ============ Parse — precedence: floor 0 > 6-prefix ============

func TestParse_FloorZero_OverridesLabHint(t *testing.T) {
	// An auditorium in the lab building should still classify as LECTURE.
	got := Parse("6A-001")
	if got.RoomTypeHint != models.RoomTypeLecture {
		t.Errorf("RoomTypeHint = %q, want LECTURE (floor 0 must override lab building)", got.RoomTypeHint)
	}
	if got.BuildingCode != "6A" {
		t.Errorf("BuildingCode = %q, want %q", got.BuildingCode, "6A")
	}
}

// ============ Parse — non-6 buildings do not receive lab hint ============

func TestParse_NonLabBuildings_NoHint(t *testing.T) {
	for _, code := range []string{"1-302", "2-201", "5A-201", "7A-303", "7B-301"} {
		got := Parse(code)
		if got.RoomTypeHint != "" {
			t.Errorf("Parse(%q).RoomTypeHint = %q, want empty (only 6-prefix or floor-0 emit hints)",
				code, got.RoomTypeHint)
		}
	}
}

// ============ Parse — degraded inputs (warning-only, no error) ============

func TestParse_EmptyCode(t *testing.T) {
	got := Parse("")
	if got.BuildingCode != "" {
		t.Errorf("BuildingCode = %q, want empty", got.BuildingCode)
	}
	if got.Confidence != 0 {
		t.Errorf("Confidence = %f, want 0", got.Confidence)
	}
	if len(got.Warnings) == 0 || !strings.Contains(got.Warnings[0], "empty") {
		t.Errorf("Warnings = %v, want [empty*]", got.Warnings)
	}
}

func TestParse_WhitespaceCode(t *testing.T) {
	got := Parse("   ")
	if got.Confidence != 0 {
		t.Errorf("Confidence = %f, want 0", got.Confidence)
	}
	if len(got.Warnings) == 0 {
		t.Error("expected warnings for whitespace-only input")
	}
}

func TestParse_NoSeparator(t *testing.T) {
	got := Parse("foobar")
	if got.BuildingCode != "foobar" {
		t.Errorf("BuildingCode = %q, want %q", got.BuildingCode, "foobar")
	}
	if got.RoomNumber != "" {
		t.Errorf("RoomNumber = %q, want empty", got.RoomNumber)
	}
	if got.Confidence >= 1.0 {
		t.Errorf("Confidence = %f, want < 1.0", got.Confidence)
	}
	if len(got.Warnings) == 0 {
		t.Error("expected 'missing separator' warning")
	}
}

func TestParse_MissingRoomNumber(t *testing.T) {
	got := Parse("1-")
	if got.BuildingCode != "1" {
		t.Errorf("BuildingCode = %q, want %q", got.BuildingCode, "1")
	}
	if got.Confidence >= 1.0 {
		t.Errorf("Confidence = %f, want < 1.0", got.Confidence)
	}
	if len(got.Warnings) == 0 {
		t.Error("expected 'missing room number' warning")
	}
}

func TestParse_NonNumericFloor(t *testing.T) {
	got := Parse("1-abc")
	if got.BuildingCode != "1" {
		t.Errorf("BuildingCode = %q, want %q", got.BuildingCode, "1")
	}
	if got.RoomNumber != "abc" {
		t.Errorf("RoomNumber = %q, want %q", got.RoomNumber, "abc")
	}
	if got.Floor != 0 {
		t.Errorf("Floor = %d, want 0 (unparseable)", got.Floor)
	}
	if got.Confidence >= 1.0 {
		t.Errorf("Confidence = %f, want < 1.0", got.Confidence)
	}
	if len(got.Warnings) == 0 {
		t.Error("expected 'floor digit not numeric' warning")
	}
}

func TestParse_MissingBuildingCode(t *testing.T) {
	got := Parse("-302")
	if got.BuildingCode != "" {
		t.Errorf("BuildingCode = %q, want empty", got.BuildingCode)
	}
	if len(got.Warnings) == 0 {
		t.Error("expected 'missing building code' warning")
	}
}

// Parse must never return an error / panic for pathological inputs.
func TestParse_PathologicalInputs_NoPanic(t *testing.T) {
	inputs := []string{
		"", "   ", "-", "--", "-1-2", "1-2-3", "1--2",
		"!!!", "\n", "1-\t302", "1--", " 1-302 ",
	}
	for _, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Parse(%q) panicked: %v", in, r)
				}
			}()
			_ = Parse(in)
		}()
	}
}

func TestParse_TrimsWhitespace(t *testing.T) {
	got := Parse("  1-302  ")
	if got.BuildingCode != "1" || got.Floor != 3 || got.RoomNumber != "02" {
		t.Errorf("Parse trimmed input incorrectly: %+v", got)
	}
	if got.Confidence != 1.0 {
		t.Errorf("Confidence = %f, want 1.0", got.Confidence)
	}
}

// ============ BuildingPartition ============

func TestBuildingPartition(t *testing.T) {
	cases := []struct {
		in            string
		wantBase      string
		wantPartition string
	}{
		{"1", "1", ""},
		{"5A", "5", "A"},
		{"5B", "5", "B"},
		{"6A", "6", "A"},
		{"7B", "7", "B"},
		{"", "", ""},
		{"XA", "X", "A"},  // non-numeric base still supported
		{"10", "10", ""},  // multi-digit code without partition
		{"10A", "10", "A"}, // multi-digit code with partition
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			base, part := BuildingPartition(c.in)
			if base != c.wantBase || part != c.wantPartition {
				t.Errorf("BuildingPartition(%q) = (%q,%q), want (%q,%q)",
					c.in, base, part, c.wantBase, c.wantPartition)
			}
		})
	}
}
