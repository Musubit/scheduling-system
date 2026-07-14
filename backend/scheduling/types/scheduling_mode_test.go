package types

import (
	"reflect"
	"testing"
)

func TestSchedulingMode_Constants(t *testing.T) {
	if ModeFullScheduling != "FULL_SCHEDULING" {
		t.Errorf("ModeFullScheduling = %q, want %q", ModeFullScheduling, "FULL_SCHEDULING")
	}
	if ModeTimeOnlyScheduling != "TIME_ONLY_SCHEDULING" {
		t.Errorf("ModeTimeOnlyScheduling = %q, want %q", ModeTimeOnlyScheduling, "TIME_ONLY_SCHEDULING")
	}
}

func TestSchedulingMode_IsValid(t *testing.T) {
	cases := []struct {
		mode SchedulingMode
		want bool
	}{
		{ModeFullScheduling, true},
		{ModeTimeOnlyScheduling, true},
		{"", false},
		{"UNKNOWN_MODE", false},
		{"full_scheduling", false}, // case-sensitive
	}
	for _, c := range cases {
		if got := c.mode.IsValid(); got != c.want {
			t.Errorf("(%q).IsValid() = %v, want %v", c.mode, got, c.want)
		}
	}
}

func TestSchedulingMode_IsTimeOnly(t *testing.T) {
	if ModeFullScheduling.IsTimeOnly() {
		t.Error("FULL_SCHEDULING should not report IsTimeOnly()")
	}
	if !ModeTimeOnlyScheduling.IsTimeOnly() {
		t.Error("TIME_ONLY_SCHEDULING should report IsTimeOnly()")
	}
}

func TestSchedulingMode_RequiresRoomAssignment(t *testing.T) {
	if !ModeFullScheduling.RequiresRoomAssignment() {
		t.Error("FULL_SCHEDULING should require room assignment")
	}
	if ModeTimeOnlyScheduling.RequiresRoomAssignment() {
		t.Error("TIME_ONLY_SCHEDULING should not require room assignment")
	}
}

func TestSchedulingMode_EnabledScoreDimensions(t *testing.T) {
	full := ModeFullScheduling.EnabledScoreDimensions()
	wantFull := []string{"time", "teacher", "student", "resource"}
	if !reflect.DeepEqual(full, wantFull) {
		t.Errorf("FULL dims = %v, want %v", full, wantFull)
	}

	timeOnly := ModeTimeOnlyScheduling.EnabledScoreDimensions()
	wantTimeOnly := []string{"time", "teacher", "student"}
	if !reflect.DeepEqual(timeOnly, wantTimeOnly) {
		t.Errorf("TIME_ONLY dims = %v, want %v", timeOnly, wantTimeOnly)
	}
}
