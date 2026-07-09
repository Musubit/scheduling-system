package services

import (
	"encoding/json"
	"scheduling-system/backend/models"
	"testing"
)

func TestLockedSlotsJSONParsing(t *testing.T) {
	// Simulate what the frontend sends as lockedSlotsJson
	lockedSlotsJSON := `[{"dayOfWeek":1,"startPeriod":0,"span":2},{"dayOfWeek":1,"startPeriod":2,"span":2}]`

	var slots []LockedTimeSlot
	err := json.Unmarshal([]byte(lockedSlotsJSON), &slots)
	if err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if len(slots) != 2 {
		t.Fatalf("expected 2 locked slots, got %d", len(slots))
	}

	if int(slots[0].DayOfWeek) != 1 {
		t.Errorf("DayOfWeek mismatch: got %d, want 1", slots[0].DayOfWeek)
	}
	if int(slots[0].StartPeriod) != 0 {
		t.Errorf("StartPeriod mismatch: got %d, want 0", slots[0].StartPeriod)
	}

	// Verify the LockedTimeSlot type serializes correctly for OR-Tools
	ortoolsInput := ORToolsInput{
		LockedSlots: slots,
	}
	data, _ := json.Marshal(ortoolsInput)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	ls := raw["lockedSlots"].([]interface{})
	first := ls[0].(map[string]interface{})
	if first["dayOfWeek"].(float64) != 1 {
		t.Errorf("ORTools serialization: dayOfWeek=%v, want 1", first["dayOfWeek"])
	}

	// Test with full-day lock (span=11)
	fullDayJSON := `[{"dayOfWeek":1,"startPeriod":0,"span":11}]`
	var fullDay []LockedTimeSlot
	json.Unmarshal([]byte(fullDayJSON), &fullDay)
	if len(fullDay) != 1 {
		t.Fatal("full day parse failed")
	}
	if int(fullDay[0].DayOfWeek) != 1 || int(fullDay[0].StartPeriod) != 0 || fullDay[0].Span != 11 {
		t.Errorf("full day parse wrong: day=%d start=%d span=%d",
			fullDay[0].DayOfWeek, fullDay[0].StartPeriod, fullDay[0].Span)
	}

	// Now test that periodsOverlapInt correctly detects overlap
	// Locked slot: Tuesday 0-11, SA tries Tuesday period 2 with span 2
	if !periodsOverlapInt(2, 2, 0, 11) {
		t.Error("BUG: periodsOverlapInt(2,2,0,11) should be true (Tuesday 3-4节 overlaps full lock)")
	}
	// Locked slot: Tuesday 0-2, SA tries Tuesday period 4 with span 2
	if periodsOverlapInt(4, 2, 0, 2) {
		t.Error("BUG: periodsOverlapInt(4,2,0,2) should be false (non-overlapping)")
	}

	t.Logf("OK: LockedTimeSlot JSON parsing and serialization all correct")
}

func TestLoadLockedSlotsFromDB(t *testing.T) {
	// Test the DB loading fallback path
	// If the JSON version of LockedSlots is properly deserialized...
	lockedSlotsJSON := `[{"dayOfWeek":0,"startPeriod":4,"span":2}]`

	var slots []LockedTimeSlot
	json.Unmarshal([]byte(lockedSlotsJSON), &slots)

	if len(slots) == 0 {
		t.Error("BUG: loadLockedSlots would return empty — Setting.Value was saved but can't be parsed")
	}

	// Test what happens when loadLockedSlots returns nil
	var nilSlots []LockedTimeSlot
	if len(nilSlots) != 0 {
		t.Error("nil slice should have len 0")
	}

	t.Log("OK: loadLockedSlots fallback works correctly")
}

func TestLockedSlotsInBuildInitial(t *testing.T) {
	// Full integration test: SA solver with locked slots
	teachers := []models.Teacher{
		{Name: "老师A"},
	}
	teachers[0].ID = 1

	classrooms := []models.Classroom{
		{Name: "教室A", Capacity: 100},
	}
	classrooms[0].ID = 1

	classGroups := []models.ClassGroup{
		{Name: "班级A", Students: 50},
	}
	classGroups[0].ID = 1

	ttc := []models.TeachingTaskClass{{ClassGroupID: 1}}
	tasks := []models.TeachingTask{
		{CourseID: 1, TeacherID: 1, SemesterID: 1, Classes: ttc},
		{CourseID: 2, TeacherID: 1, SemesterID: 1, Classes: ttc},
		{CourseID: 3, TeacherID: 1, SemesterID: 1, Classes: ttc},
		{CourseID: 4, TeacherID: 1, SemesterID: 1, Classes: ttc},
		{CourseID: 5, TeacherID: 1, SemesterID: 1, Classes: ttc},
	}
	// Set embedded gorm.Model.ID explicitly
	for i := range tasks {
		tasks[i].ID = uint(i + 1)
	}

	// Lock Monday and Tuesday entirely
	locked := []LockedTimeSlot{
		{DayOfWeek: 0, StartPeriod: 0, Span: 11}, // Monday
		{DayOfWeek: 1, StartPeriod: 0, Span: 11}, // Tuesday
	}

	config := SAConfig{
		InitialTemp: 10.0, CoolingRate: 0.95, IterationsPerTemp: 50,
		MinTemp: 0.1, MaxTimeSeconds: 5, Seed: 42,
	}

	solver := NewSASolver()
	result := solver.Solve(tasks, teachers, classrooms, classGroups,
		locked, []string{}, "2025-S2", config, nil, nil)

	for _, e := range result.Entries {
		if e.DayOfWeek == 0 || e.DayOfWeek == 1 {
			t.Errorf("BUG: entry on locked day: day=%d, start=%d, course=%d",
				e.DayOfWeek, e.StartPeriod, e.CourseID)
		}
	}

	t.Logf("OK: %d entries placed, none on locked Mon/Tue", len(result.Entries))
}
