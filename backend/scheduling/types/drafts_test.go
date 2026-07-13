package types

import (
	"reflect"
	"strings"
	"testing"
)

func TestTimeAssignmentDraft_HasNoPersistenceFields(t *testing.T) {
	// INV-P5: Drafts must not carry ID, ScheduleVersionID, or SemesterID.
	// This test uses reflection to catch accidental additions.
	forbidden := []string{"ID", "ScheduleVersionID", "SemesterID"}
	typ := reflect.TypeOf(TimeAssignmentDraft{})
	for _, name := range forbidden {
		if _, found := typ.FieldByName(name); found {
			t.Errorf("TimeAssignmentDraft must not contain field %q (INV-P5)", name)
		}
	}
}

func TestTimeAssignmentDraft_FieldsAccessible(t *testing.T) {
	d := TimeAssignmentDraft{
		TeachingTaskID: 42, DayOfWeek: 2, StartPeriod: 4, Span: 2,
	}
	if d.TeachingTaskID != 42 || d.Span != 2 {
		t.Fatalf("field roundtrip broken: %+v", d)
	}
}

func TestRoomAllocationDraft_HasNoPersistenceFields(t *testing.T) {
	forbidden := []string{"ID", "ScheduleVersionID", "SemesterID", "TimeAssignmentID"}
	typ := reflect.TypeOf(RoomAllocationDraft{})
	for _, name := range forbidden {
		if _, found := typ.FieldByName(name); found {
			t.Errorf("RoomAllocationDraft must not contain field %q (INV-P5)", name)
		}
	}
}

func TestRoomAllocationDraft_FieldsAccessible(t *testing.T) {
	a := RoomAllocationDraft{LocalRef: 7, ClassroomID: 500}
	if a.LocalRef != 7 || a.ClassroomID != 500 {
		t.Fatalf("field roundtrip broken: %+v", a)
	}
}

func TestTimeAssignmentPlaced_FieldsAccessible(t *testing.T) {
	p := TimeAssignmentPlaced{
		LocalRef: 3, TeachingTaskID: 42,
		DayOfWeek: 2, StartPeriod: 4, Span: 2,
		TotalStudents: 90, RequiredRoomType: "computer_lab",
		AllowedRoomIDs: []uint{500, 501},
	}
	if p.LocalRef != 3 || p.RequiredRoomType != "computer_lab" {
		t.Fatalf("field roundtrip broken: %+v", p)
	}
}

func TestDrafts_NoGormTags(t *testing.T) {
	// INV-P10: Draft types must not carry gorm tags (would enable accidental persistence).
	for _, typ := range []reflect.Type{
		reflect.TypeOf(TimeAssignmentDraft{}),
		reflect.TypeOf(RoomAllocationDraft{}),
		reflect.TypeOf(TimeAssignmentPlaced{}),
	} {
		for i := 0; i < typ.NumField(); i++ {
			if tag := typ.Field(i).Tag.Get("gorm"); tag != "" {
				t.Errorf("%s.%s has gorm tag %q — Drafts must be persistence-free (INV-P10)",
					typ.Name(), typ.Field(i).Name, tag)
			}
			// Also reject GORM's soft-delete embedded types by name.
			if strings.Contains(typ.Field(i).Type.String(), "gorm.Model") {
				t.Errorf("%s.%s embeds gorm.Model — forbidden (INV-P10)",
					typ.Name(), typ.Field(i).Name)
			}
		}
	}
}
