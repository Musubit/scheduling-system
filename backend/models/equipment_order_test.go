package models

import (
	"sort"
	"testing"
)

// TestEquipmentSetItemsDeterministic verifies that Items() returns a consistent order.
// This is a regression test for the map iteration order non-determinism bug.
func TestEquipmentSetItemsDeterministic(t *testing.T) {
	jsonStr := `["projector","smartboard","aircon","computer"]`
	eq := ParseEquipment(jsonStr)

	// Call Items() multiple times and verify consistent order
	var prev []string
	for i := 0; i < 100; i++ {
		items := eq.Items()
		if prev == nil {
			prev = items
			continue
		}
		if len(items) != len(prev) {
			t.Fatalf("Items() returned different lengths: %d vs %d", len(items), len(prev))
		}
		for j := range items {
			if items[j] != prev[j] {
				t.Errorf("Items() order changed: iteration %d, expected %v, got %v", i, prev, items)
				return
			}
		}
	}
}

// TestEquipmentSetToJSONDeterministic verifies that ToJSON() produces consistent output.
func TestEquipmentSetToJSONDeterministic(t *testing.T) {
	jsonStr := `["projector","smartboard","aircon","computer"]`
	eq := ParseEquipment(jsonStr)

	var prev string
	for i := 0; i < 100; i++ {
		result := eq.ToJSON()
		if prev == "" {
			prev = result
			continue
		}
		if result != prev {
			t.Errorf("ToJSON() output changed: iteration %d, expected %q, got %q", i, prev, result)
			return
		}
	}
}

// TestEquipmentSetItemsSorted verifies that Items() returns sorted output.
func TestEquipmentSetItemsSorted(t *testing.T) {
	jsonStr := `["projector","smartboard","aircon","computer"]`
	eq := ParseEquipment(jsonStr)
	items := eq.Items()

	// Check if items are sorted
	if !sort.StringsAreSorted(items) {
		t.Errorf("Items() returned unsorted result: %v", items)
	}
}