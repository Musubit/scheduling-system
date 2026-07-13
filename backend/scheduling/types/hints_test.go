package types

import "testing"

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
