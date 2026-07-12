package models

import "testing"

func TestParseEquipment_Valid(t *testing.T) {
	s := ParseEquipment(`["projector","smartboard"]`)
	if !s.Has("projector") {
		t.Error("expected projector")
	}
	if !s.Has("smartboard") {
		t.Error("expected smartboard")
	}
	if s.Has("aircon") {
		t.Error("did not expect aircon")
	}
}

func TestParseEquipment_Empty(t *testing.T) {
	s := ParseEquipment("")
	if !s.IsEmpty() {
		t.Error("expected empty set")
	}
}

func TestParseEquipment_Malformed(t *testing.T) {
	s := ParseEquipment("broken json")
	if !s.IsEmpty() {
		t.Error("expected empty set for malformed JSON")
	}
}

func TestEquipmentSet_ContainsAll_Full(t *testing.T) {
	required := ParseEquipment(`["a","b"]`)
	set := ParseEquipment(`["a","b","c"]`)
	if !set.ContainsAll(required) {
		t.Error("expected ContainsAll=true")
	}
}

func TestEquipmentSet_ContainsAll_Partial(t *testing.T) {
	required := ParseEquipment(`["a","d"]`)
	set := ParseEquipment(`["a","b","c"]`)
	if set.ContainsAll(required) {
		t.Error("expected ContainsAll=false")
	}
}

func TestEquipmentSet_ContainsAll_EmptyRequired(t *testing.T) {
	required := ParseEquipment("")
	set := ParseEquipment(`["a"]`)
	if !set.ContainsAll(required) {
		t.Error("empty required should always match")
	}
}

func TestEquipmentSet_ToJSON_RoundTrip(t *testing.T) {
	original := `["projector","smartboard"]`
	s := ParseEquipment(original)
	roundTrip := s.ToJSON()
	s2 := ParseEquipment(roundTrip)
	if !s2.Has("projector") || !s2.Has("smartboard") {
		t.Error("round-trip lost items")
	}
}

func TestEquipmentSet_ToJSON_Empty(t *testing.T) {
	s := EquipmentSet{}
	if s.ToJSON() != "" {
		t.Error("expected empty string for empty set")
	}
}
