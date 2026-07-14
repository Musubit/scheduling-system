package types

import "testing"

func TestTeachingTaskView_FieldsAccessible(t *testing.T) {
	v := TeachingTaskView{
		ID:               1,
		CourseID:         10,
		CourseName:       "计算机组成原理",
		CourseHours:      48,
		TeacherID:        100,
		ClassGroupIDs:    []uint{1000, 1001},
		TotalStudents:    90,
		StartWeek:        1,
		EndWeek:          16,
		MaxHoursPerWeek:  4,
		PreferredSpan:    2,
		RequiredRoomType: "computer_lab",
		AllowedRoomIDs:   []uint{500, 501},
		IsSports:         false,
	}
	if v.ID != 1 || v.CourseName != "计算机组成原理" || len(v.ClassGroupIDs) != 2 {
		t.Fatalf("field roundtrip broken: %+v", v)
	}
}

func TestTeacherView_FieldsAccessible(t *testing.T) {
	v := TeacherView{
		ID:               100,
		Name:             "张老师",
		PreferNoEarly:    true,
		PreferNoLate:     false,
		PreferLowFloor:   true,
		MaxDaysPerWeek:   3,
		UnavailableSlots: []LockedTimeSlot{{DayOfWeek: 5, StartPeriod: 0, Span: 2}},
	}
	if !v.PreferNoEarly || v.PreferNoLate || len(v.UnavailableSlots) != 1 {
		t.Fatalf("field roundtrip broken: %+v", v)
	}
}

func TestClassGroupView_FieldsAccessible(t *testing.T) {
	v := ClassGroupView{ID: 1000, Name: "计科2201", Students: 45}
	if v.Students != 45 {
		t.Fatalf("field roundtrip broken: %+v", v)
	}
}

func TestClassroomView_FieldsAccessible(t *testing.T) {
	v := ClassroomView{
		ID: 500, Capacity: 60, Type: "computer_lab",
		Floor: 3, Equipment: "projector,whiteboard", IsShared: false,
	}
	if v.Capacity != 60 || v.Type != "computer_lab" {
		t.Fatalf("field roundtrip broken: %+v", v)
	}
}
