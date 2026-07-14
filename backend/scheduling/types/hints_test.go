package types

import (
	"reflect"
	"strings"
	"testing"
)

func TestHintReason_Constants(t *testing.T) {
	cases := map[HintReason]string{
		ReasonNoCapacity:     "no_room_with_capacity",
		ReasonNoMatchingType: "no_room_of_required_type",
		ReasonAllOccupied:    "all_matching_rooms_occupied",
		ReasonEquipmentMiss:  "no_room_with_equipment",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Errorf("%v = %q, want %q", got, string(got), want)
		}
	}
}

func TestResourceConflictHint_ZeroValueConstructible(t *testing.T) {
	// Ensures the struct can be constructed by zero-value + field assignment.
	h := ResourceConflictHint{}
	h.TeachingTaskID = 42
	h.DayOfWeek = 1
	h.StartPeriod = 4
	h.Span = 2
	h.Reason = ReasonNoCapacity
	h.Detail = "no room fits 120 students"
	if h.TeachingTaskID != 42 || h.Reason != ReasonNoCapacity {
		t.Fatalf("field roundtrip broken: %+v", h)
	}
}

// TestResourceConflictHint_NoGormTags enforces INV-H1: hints are transient
// signals, never persisted. An accidental gorm tag on any field would silently
// admit the type to AutoMigrate and violate the invariant at the schema level.
func TestResourceConflictHint_NoGormTags(t *testing.T) {
	typ := reflect.TypeOf(ResourceConflictHint{})
	for i := 0; i < typ.NumField(); i++ {
		if tag := typ.Field(i).Tag.Get("gorm"); tag != "" {
			t.Errorf("%s.%s has gorm tag %q — hints must be persistence-free (INV-H1)",
				typ.Name(), typ.Field(i).Name, tag)
		}
		if strings.Contains(typ.Field(i).Type.String(), "gorm.Model") {
			t.Errorf("%s.%s embeds gorm.Model — forbidden (INV-H1)",
				typ.Name(), typ.Field(i).Name)
		}
	}
}

// TestHint_DistinctFromLockedTimeSlot enforces INV-H2: even though both
// carry a (task, day, start, span) tuple, they are distinct types with
// different semantics — hint is a signal to the retry loop, LockedTimeSlot
// is a hard constraint. Aliasing them (e.g. `type ResourceConflictHint =
// LockedTimeSlot`) would let a producer feed one into the other's consumer.
func TestHint_DistinctFromLockedTimeSlot(t *testing.T) {
	hintType := reflect.TypeOf(ResourceConflictHint{})
	lockType := reflect.TypeOf(LockedTimeSlot{})
	if hintType == lockType {
		t.Fatalf("INV-H2 violated: ResourceConflictHint and LockedTimeSlot resolve to the same type")
	}
}
