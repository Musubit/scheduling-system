package types

// TeachingTaskView is a value-type projection of models.TeachingTask
// stripped of GORM concepts. The Service layer builds this from the DB
// row before invoking any Solver component.
//
// The View is shared across Time and Room stages (spec §3.3.1). Fields
// RequiredRoomType and AllowedRoomIDs are populated by the Service layer
// but ignored by TimeScheduler (compile-time projection isolation is
// enforced by INV-P1/P2 rather than by splitting the type).
type TeachingTaskView struct {
	ID               uint
	CourseID         uint
	CourseName       string
	CourseHours      int
	TeacherID        uint
	ClassGroupIDs    []uint
	TotalStudents    int
	StartWeek        int
	EndWeek          int
	MaxHoursPerWeek  int
	PreferredSpan    int
	RequiredRoomType string
	AllowedRoomIDs   []uint
	IsSports         bool
}

// TeacherView is a value-type projection of models.Teacher.
type TeacherView struct {
	ID               uint
	Name             string
	PreferNoEarly    bool
	PreferNoLate     bool
	PreferLowFloor   bool
	MaxDaysPerWeek   int
	UnavailableSlots []LockedTimeSlot
}

// ClassGroupView is a value-type projection of models.ClassGroup.
type ClassGroupView struct {
	ID       uint
	Name     string
	Students int
}

// ClassroomView is a value-type projection of models.Classroom. Populated
// only in FULL_SCHEDULING mode; RoomScheduler is the sole consumer.
type ClassroomView struct {
	ID        uint
	Capacity  int
	Type      string
	Floor     int
	Equipment string
	IsShared  bool // e.g., 体育馆 — a shared venue that never conflicts on time
}
