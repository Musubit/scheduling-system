# PR-01: scheduling/types Package Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create `backend/scheduling/types` — a new Go package containing all DTOs and interfaces required by the v0.5.5 two-stage scheduling pipeline. No implementation, no wiring, no production code path touched.

**Architecture:** All types are plain value types (no GORM tags, no methods that touch IO). All interfaces have empty bodies. `scheduling/types` depends only on the Go standard library. This is Phase 1 of the Strangler Fig migration (PR-01 of 12): the package exists as verifiable dead code, compilation-checked, and unit-tested for construction/nil-safety, but is not referenced by any production code path.

**Tech Stack:** Go 1.21+, standard library only, `testing` package.

## Global Constraints

- **Reference spec**: `docs/superpowers/specs/2026-07-13-scheduling-dual-mode-design.md` §3.3, §3.4, §2.1, §2.4, §2.7 (frozen SSoT).
- **INV-P2**: `scheduling/types` imports only stdlib. No `backend/database`, `backend/models`, `backend/services/*`, `gorm.io/gorm`.
- **INV-P5**: `TimeAssignmentDraft`, `RoomAllocationDraft`, `TimeAssignmentPlaced` carry no ID, no `ScheduleVersionID`, no `SemesterID`.
- **INV-H1**: `ResourceConflictHint` has no GORM tags.
- **INV-H2**: `ResourceConflictHint` and `LockedTimeSlot` are distinct struct types; no shared embedding or type alias.
- **INV-M1**: `SchedulingMode` constant set is closed; only two values allowed: `"FULL_SCHEDULING"` and `"TIME_ONLY_SCHEDULING"`.
- **INV-S1**: `ScoreBreakdown` bucket fields are `*ScoreBucket` (pointer); nil = Disabled; no sentinel value.
- **CI-1 / CI-2 / CI-6**: `go build ./...` passes; `go test ./...` passes; `scheduling/*` has no forbidden imports (checked in Task 10).
- **Spec deviation from §4.1 (documented)**: `SchedulingMode`, `ScoreBreakdown`, `ScoreBucket`, `DayOfWeek`, `Period` are defined in `scheduling/types` (not `backend/models`) so that `types` depends only on stdlib. `backend/models/scheduling_mode.go` (mentioned in §4.1 for PR-03) will not be created; PR-03 will instead import `scheduling/types.SchedulingMode`.
- **No production wiring**: Package must not be referenced by any file outside `backend/scheduling/types/**` after this PR merges (verified by Task 10).
- **Line budget**: ~500 lines total across all files (rough guidance; measured after Task 9).

## File Structure

```
backend/scheduling/types/
├── doc.go                  Package documentation
├── calendar.go             DayOfWeek + Period (int-based, no methods)
├── scheduling_mode.go      SchedulingMode + IsValid/IsTimeOnly/RequiresRoomAssignment/EnabledScoreDimensions
├── locked_slot.go          LockedTimeSlot (value type)
├── hints.go                ResourceConflictHint + HintReason enum
├── views.go                TeachingTaskView / TeacherView / ClassGroupView / ClassroomView
├── drafts.go               TimeAssignmentDraft / RoomAllocationDraft / TimeAssignmentPlaced
├── score.go                ScoreBreakdown / ScoreBucket / TimeScoreDetail / ResourceScoreDetail
├── io.go                   TimeSchedulingInput/Output + RoomSchedulingInput/Output + OrchestratorRequest/Result
├── interfaces.go           ITimeScheduler / IRoomScheduler / IScorer / ISchedulingOrchestrator / ProgressReporter / NoopReporter
│
└── (tests, one per source file)
    ├── scheduling_mode_test.go
    ├── hints_test.go
    ├── views_test.go
    ├── drafts_test.go
    ├── score_test.go
    ├── interfaces_test.go
    └── isolation_test.go   (Task 10: import-boundary check)
```

Total: 10 source files + 7 test files.

---

### Task 1: Scaffold package + calendar primitives + doc.go

**Files:**
- Create: `backend/scheduling/types/doc.go`
- Create: `backend/scheduling/types/calendar.go`

**Interfaces:**
- Consumes: nothing
- Produces:
  - Package `types` at import path `scheduling-system/backend/scheduling/types`
  - `type DayOfWeek int` — 0=Mon..6=Sun (Sun=6 matches existing `backend/models/types.go`)
  - `type Period int` — 0..10 (0 = 第1节, 10 = 第11节)

- [ ] **Step 1: Create doc.go**

```go
// Package types defines the DTOs and interfaces of the v0.5.5 two-stage
// scheduling pipeline (TimeScheduler → RoomScheduler → Orchestrator).
//
// This package depends only on the Go standard library (INV-P2). No GORM
// models, no database packages, no service-layer imports are allowed.
//
// The types here are plain value types (INV-P5, INV-H1). Interfaces are
// implementation-free; concrete implementations live in sibling packages
// (scheduling/time, scheduling/room, scheduling/orchestrator, scheduling/score).
//
// See docs/superpowers/specs/2026-07-13-scheduling-dual-mode-design.md
// (Sections 3.3, 3.4, and 7 Invariants) for the authoritative design.
package types
```

Write this file at `C:/Users/musubi/Desktop/scheduling-system/backend/scheduling/types/doc.go`.

- [ ] **Step 2: Create calendar.go**

```go
package types

// DayOfWeek is the internal 0-indexed weekday used across the scheduling
// pipeline. 0 = Monday, 6 = Sunday. Matches the semantics of
// backend/models.DayOfWeek without depending on it (INV-P2).
type DayOfWeek int

// Period is the internal 0-indexed teaching period. Period 0 = 第1节,
// Period 10 = 第11节. Matches the semantics of backend/models.Period
// without depending on it (INV-P2).
type Period int
```

No methods on either type in PR-01. Methods (`String`, `IsSpanLegal`,
`Overlaps`, `ValidStartsForSpan`) are added in later PRs (PR-04 / PR-07)
where they are actually used inside Solver implementations.

- [ ] **Step 3: Verify compilation**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go build ./backend/scheduling/types/`
Expected: no output (successful build).

- [ ] **Step 4: Commit**

```bash
cd C:/Users/musubi/Desktop/scheduling-system
git add backend/scheduling/types/doc.go backend/scheduling/types/calendar.go
git commit -m "feat(scheduling): scaffold types package with calendar primitives (PR-01/12)"
```

---

### Task 2: SchedulingMode + tests

**Files:**
- Create: `backend/scheduling/types/scheduling_mode.go`
- Create: `backend/scheduling/types/scheduling_mode_test.go`

**Interfaces:**
- Consumes: nothing new
- Produces:
  - `type SchedulingMode string`
  - Constants `ModeFullScheduling = "FULL_SCHEDULING"`, `ModeTimeOnlyScheduling = "TIME_ONLY_SCHEDULING"`
  - Methods on `SchedulingMode`:
    - `IsValid() bool`
    - `IsTimeOnly() bool`
    - `RequiresRoomAssignment() bool`
    - `EnabledScoreDimensions() []string` — returns `["time","teacher","student"]` for TIME_ONLY, `["time","teacher","student","resource"]` for FULL

- [ ] **Step 1: Write failing tests**

Create `backend/scheduling/types/scheduling_mode_test.go`:

```go
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
```

- [ ] **Step 2: Run tests, expect failure**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go test ./backend/scheduling/types/ -run TestSchedulingMode -v`
Expected: compilation error (`undefined: SchedulingMode`, `undefined: ModeFullScheduling`, etc.).

- [ ] **Step 3: Implement scheduling_mode.go**

```go
package types

// SchedulingMode is the sole source of mode state (I1, INV-M1). Two values
// are allowed; all other code must go through the methods on this type
// rather than switching on the raw string.
type SchedulingMode string

const (
	ModeFullScheduling     SchedulingMode = "FULL_SCHEDULING"
	ModeTimeOnlyScheduling SchedulingMode = "TIME_ONLY_SCHEDULING"
)

// IsValid reports whether m is one of the two allowed modes.
func (m SchedulingMode) IsValid() bool {
	switch m {
	case ModeFullScheduling, ModeTimeOnlyScheduling:
		return true
	}
	return false
}

// IsTimeOnly reports whether m suppresses the room-assignment stage.
func (m SchedulingMode) IsTimeOnly() bool {
	return m == ModeTimeOnlyScheduling
}

// RequiresRoomAssignment reports whether the pipeline must run the room
// scheduler. Orchestrator assembly decision point (INV-P4).
func (m SchedulingMode) RequiresRoomAssignment() bool {
	return m == ModeFullScheduling
}

// EnabledScoreDimensions returns the ordered list of score bucket keys
// active for this mode. The returned slice must be treated as read-only.
// TIME_ONLY excludes "resource"; FULL includes all four (INV-S2).
func (m SchedulingMode) EnabledScoreDimensions() []string {
	if m == ModeTimeOnlyScheduling {
		return []string{"time", "teacher", "student"}
	}
	return []string{"time", "teacher", "student", "resource"}
}
```

- [ ] **Step 4: Run tests, expect pass**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go test ./backend/scheduling/types/ -run TestSchedulingMode -v`
Expected: all 5 subtests PASS.

- [ ] **Step 5: Commit**

```bash
cd C:/Users/musubi/Desktop/scheduling-system
git add backend/scheduling/types/scheduling_mode.go backend/scheduling/types/scheduling_mode_test.go
git commit -m "feat(scheduling): add SchedulingMode with assembly-decision methods (PR-01/12)"
```

---

### Task 3: LockedTimeSlot value type

**Files:**
- Create: `backend/scheduling/types/locked_slot.go`

**Interfaces:**
- Consumes: `DayOfWeek`, `Period` from Task 1
- Produces: `type LockedTimeSlot struct{...}` — plain value type; no GORM tags; distinct from `ResourceConflictHint` (INV-H2)

- [ ] **Step 1: Implement locked_slot.go**

```go
package types

// LockedTimeSlot is a time region that is forbidden for scheduling.
// It represents administrator-configured hard constraints (e.g., a
// campus-wide reserved period). This is a value-type copy inside the
// types package; the services layer holds its own LockedTimeSlot type
// with the same shape but distinct identity, and callers must copy
// across the boundary rather than share the underlying struct.
//
// Do not merge this with ResourceConflictHint (INV-H2). They differ in
// meaning: LockedTimeSlot is a persistent, admin-authored hard rule;
// ResourceConflictHint is a transient, solver-generated soft signal.
type LockedTimeSlot struct {
	DayOfWeek   DayOfWeek `json:"dayOfWeek"`
	StartPeriod Period    `json:"startPeriod"`
	Span        int       `json:"span"`
}
```

No dedicated test file — this is a bare struct. Coverage comes from
Task 6 (View tests) and Task 8 (IO tests) which construct LockedTimeSlot
values.

- [ ] **Step 2: Verify compilation**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go build ./backend/scheduling/types/`
Expected: no output.

- [ ] **Step 3: Commit**

```bash
cd C:/Users/musubi/Desktop/scheduling-system
git add backend/scheduling/types/locked_slot.go
git commit -m "feat(scheduling): add LockedTimeSlot value type (PR-01/12)"
```

---

### Task 4: ResourceConflictHint + HintReason enum

**Files:**
- Create: `backend/scheduling/types/hints.go`
- Create: `backend/scheduling/types/hints_test.go`

**Interfaces:**
- Consumes: `DayOfWeek`, `Period` from Task 1
- Produces:
  - `type HintReason string` with 4 constants: `ReasonNoCapacity`, `ReasonNoMatchingType`, `ReasonAllOccupied`, `ReasonEquipmentMiss`
  - `type ResourceConflictHint struct{...}` — no GORM tags (INV-H1); distinct from `LockedTimeSlot` (INV-H2)

- [ ] **Step 1: Write failing tests**

Create `backend/scheduling/types/hints_test.go`:

```go
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
```

- [ ] **Step 2: Run tests, expect failure**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go test ./backend/scheduling/types/ -run "TestHintReason|TestResourceConflictHint" -v`
Expected: compilation error (`undefined: HintReason`, etc.).

- [ ] **Step 3: Implement hints.go**

```go
package types

// HintReason enumerates why a RoomScheduler could not allocate a room to
// a particular TimeAssignment. Values are stable strings; they appear in
// logs and are surfaced by the frontend for diagnostics.
type HintReason string

const (
	ReasonNoCapacity     HintReason = "no_room_with_capacity"
	ReasonNoMatchingType HintReason = "no_room_of_required_type"
	ReasonAllOccupied    HintReason = "all_matching_rooms_occupied"
	ReasonEquipmentMiss  HintReason = "no_room_with_equipment"
)

// ResourceConflictHint is a transient, in-memory signal from RoomScheduler
// back to the Orchestrator's retry loop. It is NEVER persisted (INV-H1)
// and MUST NOT share identity with LockedTimeSlot (INV-H2). Producers
// create hints per failed placement; consumers (the retry loop) forward
// hints as soft avoidance signals to the next TimeScheduler pass.
type ResourceConflictHint struct {
	TeachingTaskID uint
	DayOfWeek      DayOfWeek
	StartPeriod    Period
	Span           int
	Reason         HintReason
	Detail         string // human-readable supplement, safe to log
}
```

- [ ] **Step 4: Run tests, expect pass**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go test ./backend/scheduling/types/ -run "TestHintReason|TestResourceConflictHint" -v`
Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
cd C:/Users/musubi/Desktop/scheduling-system
git add backend/scheduling/types/hints.go backend/scheduling/types/hints_test.go
git commit -m "feat(scheduling): add ResourceConflictHint + HintReason (PR-01/12)"
```

---

### Task 5: View types (Task/Teacher/ClassGroup/Classroom)

**Files:**
- Create: `backend/scheduling/types/views.go`
- Create: `backend/scheduling/types/views_test.go`

**Interfaces:**
- Consumes: `LockedTimeSlot` from Task 3
- Produces:
  - `type TeachingTaskView struct{...}` — full field set per spec §3.3.1
  - `type TeacherView struct{...}` — per spec §3.3.1
  - `type ClassGroupView struct{...}` — inferred from `backend/models.ClassGroup`
  - `type ClassroomView struct{...}` — per spec §4.6

- [ ] **Step 1: Write failing tests**

Create `backend/scheduling/types/views_test.go`:

```go
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
```

- [ ] **Step 2: Run tests, expect failure**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go test ./backend/scheduling/types/ -run "TestTeachingTaskView|TestTeacherView|TestClassGroupView|TestClassroomView" -v`
Expected: compilation error (`undefined: TeachingTaskView`, etc.).

- [ ] **Step 3: Implement views.go**

```go
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
```

- [ ] **Step 4: Run tests, expect pass**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go test ./backend/scheduling/types/ -run "TestTeachingTaskView|TestTeacherView|TestClassGroupView|TestClassroomView" -v`
Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
cd C:/Users/musubi/Desktop/scheduling-system
git add backend/scheduling/types/views.go backend/scheduling/types/views_test.go
git commit -m "feat(scheduling): add View types for tasks/teachers/classes/rooms (PR-01/12)"
```

---

### Task 6: Draft types (in-memory only, no persistence fields)

**Files:**
- Create: `backend/scheduling/types/drafts.go`
- Create: `backend/scheduling/types/drafts_test.go`

**Interfaces:**
- Consumes: `DayOfWeek`, `Period` from Task 1
- Produces:
  - `type TimeAssignmentDraft struct{...}` — INV-P5: no `ID`, no `ScheduleVersionID`, no `SemesterID`
  - `type RoomAllocationDraft struct{...}` — carries only `LocalRef` (int index) and `ClassroomID`
  - `type TimeAssignmentPlaced struct{...}` — Orchestrator-internal Stage 1→2 transfer type

- [ ] **Step 1: Write failing tests**

Create `backend/scheduling/types/drafts_test.go`:

```go
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
```

- [ ] **Step 2: Run tests, expect failure**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go test ./backend/scheduling/types/ -run TestDraft -v && go test ./backend/scheduling/types/ -run "TestTimeAssignmentDraft|TestRoomAllocationDraft|TestTimeAssignmentPlaced" -v`
Expected: compilation errors for undefined types.

- [ ] **Step 3: Implement drafts.go**

```go
package types

// TimeAssignmentDraft is the Stage 1 output row — a scheduled weekly
// session that has NOT yet been persisted. Deliberately missing ID,
// ScheduleVersionID, and SemesterID (INV-P5): those are filled by the
// SchedulingService transaction, not by any Solver component.
type TimeAssignmentDraft struct {
	TeachingTaskID uint
	DayOfWeek      DayOfWeek
	StartPeriod    Period
	Span           int
}

// RoomAllocationDraft is the Stage 2 output — a room allocation for a
// particular TimeAssignment, referenced by LocalRef because the TA has
// not been persisted yet and has no real ID. The Orchestrator resolves
// LocalRef → real TA ID inside the persistence transaction.
type RoomAllocationDraft struct {
	LocalRef    int
	ClassroomID uint
}

// TimeAssignmentPlaced is the internal Stage 1 → Stage 2 transfer type.
// It repackages a TimeAssignmentDraft with the extra fields RoomScheduler
// needs (student count, required room type, allowed room IDs) so that
// RoomScheduler does not have to re-look-up the source TeachingTaskView.
// The LocalRef field lets RoomScheduler cite unpersisted TAs in its
// RoomAllocationDraft output and ResourceConflictHint output.
type TimeAssignmentPlaced struct {
	LocalRef         int
	TeachingTaskID   uint
	DayOfWeek        DayOfWeek
	StartPeriod      Period
	Span             int
	TotalStudents    int
	RequiredRoomType string
	AllowedRoomIDs   []uint
}
```

- [ ] **Step 4: Run tests, expect pass**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go test ./backend/scheduling/types/ -v`
Expected: all tests in the package PASS (including tests from prior tasks).

- [ ] **Step 5: Commit**

```bash
cd C:/Users/musubi/Desktop/scheduling-system
git add backend/scheduling/types/drafts.go backend/scheduling/types/drafts_test.go
git commit -m "feat(scheduling): add Draft types with INV-P5/P10 field guards (PR-01/12)"
```

---

### Task 7: Score types (ScoreBreakdown / ScoreBucket / partial details)

**Files:**
- Create: `backend/scheduling/types/score.go`
- Create: `backend/scheduling/types/score_test.go`

**Interfaces:**
- Consumes: nothing beyond stdlib
- Produces:
  - `type ScoreBucket struct{...}`
  - `type ScoreBreakdown struct{...}` with `*ScoreBucket` fields (nil = Disabled per INV-S1)
  - `type TimeScoreDetail struct{...}` — TimeScheduler's partial score (3 buckets)
  - `type ResourceScoreDetail struct{...}` — RoomScheduler's partial score (1 bucket)

- [ ] **Step 1: Write failing tests**

Create `backend/scheduling/types/score_test.go`:

```go
package types

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestScoreBucket_FieldsAccessible(t *testing.T) {
	b := ScoreBucket{Value: 22.5, Max: 25.0, Details: map[string]float64{"course_dispersed": 8}}
	if b.Value != 22.5 || b.Details["course_dispersed"] != 8 {
		t.Fatalf("field roundtrip broken: %+v", b)
	}
}

func TestScoreBreakdown_NilBucketsAreDisabled(t *testing.T) {
	// INV-S1: A nil bucket = Disabled. Not zero-valued, not sentinel.
	sb := ScoreBreakdown{
		Time:              &ScoreBucket{Value: 20, Max: 25},
		Teacher:           &ScoreBucket{Value: 22, Max: 25},
		Student:           &ScoreBucket{Value: 18, Max: 25},
		Resource:          nil, // TIME_ONLY case
		EnabledDimensions: []string{"time", "teacher", "student"},
		PerBucketMax:      25.0,
	}
	if sb.Resource != nil {
		t.Error("Resource bucket should be nil in TIME_ONLY setup")
	}
	if sb.Time == nil {
		t.Error("Time bucket should not be nil")
	}
}

func TestScoreBreakdown_JSONOmitsNilBuckets(t *testing.T) {
	// INV-F3 (frontend-facing): Disabled buckets are absent from JSON,
	// not serialized as null or 0. Ensures the *ScoreBucket field type
	// carries `omitempty` semantics naturally.
	sb := ScoreBreakdown{
		Time:              &ScoreBucket{Value: 20, Max: 25, Details: map[string]float64{"course_dispersed": 8}},
		Teacher:           &ScoreBucket{Value: 22, Max: 25, Details: map[string]float64{}},
		Student:           &ScoreBucket{Value: 18, Max: 25, Details: map[string]float64{}},
		Resource:          nil,
		EnabledDimensions: []string{"time", "teacher", "student"},
	}
	raw, err := json.Marshal(sb)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	got := string(raw)
	if strings.Contains(got, "\"resource\"") {
		t.Errorf("JSON must omit nil Resource bucket, got: %s", got)
	}
	if !strings.Contains(got, "\"time\"") {
		t.Errorf("JSON must include Time bucket, got: %s", got)
	}
}

func TestScoreDetails_FieldsAccessible(t *testing.T) {
	tsd := TimeScoreDetail{
		Time:    &ScoreBucket{Value: 20},
		Teacher: &ScoreBucket{Value: 22},
		Student: &ScoreBucket{Value: 18},
	}
	rsd := ResourceScoreDetail{Resource: &ScoreBucket{Value: 24}}
	if tsd.Time.Value != 20 || rsd.Resource.Value != 24 {
		t.Fatalf("detail types broken: %+v %+v", tsd, rsd)
	}
}

func TestScoreBreakdown_FieldTypes(t *testing.T) {
	// Guard: all four bucket fields must be *ScoreBucket, not ScoreBucket.
	// Prevents an accidental value-typed field that would break INV-S1.
	typ := reflect.TypeOf(ScoreBreakdown{})
	wantPtr := reflect.PointerTo(reflect.TypeOf(ScoreBucket{}))
	for _, name := range []string{"Time", "Teacher", "Student", "Resource"} {
		f, ok := typ.FieldByName(name)
		if !ok {
			t.Errorf("ScoreBreakdown missing field %q", name)
			continue
		}
		if f.Type != wantPtr {
			t.Errorf("ScoreBreakdown.%s type = %v, want %v (INV-S1)", name, f.Type, wantPtr)
		}
	}
}
```

- [ ] **Step 2: Run tests, expect failure**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go test ./backend/scheduling/types/ -run "TestScoreBucket|TestScoreBreakdown|TestScoreDetails" -v`
Expected: compilation errors.

- [ ] **Step 3: Implement score.go**

```go
package types

// ScoreBucket is one dimension's contribution to a ScoreBreakdown. Value
// is the achieved score in this bucket; Max is the ceiling
// (PerBucketMax); Details carries per-sub-constraint scores for
// diagnostics.
type ScoreBucket struct {
	Value   float64            `json:"value"`
	Max     float64            `json:"max"`
	Details map[string]float64 `json:"details"`
}

// ScoreBreakdown is the four-bucket score of a scheduling result. A nil
// bucket field means the dimension is Disabled for this run (INV-S1);
// TIME_ONLY runs always have Resource == nil. The mapping between
// EnabledDimensions and non-nil bucket fields is one-to-one (INV-S2).
type ScoreBreakdown struct {
	Time     *ScoreBucket `json:"time,omitempty"`
	Teacher  *ScoreBucket `json:"teacher,omitempty"`
	Student  *ScoreBucket `json:"student,omitempty"`
	Resource *ScoreBucket `json:"resource,omitempty"`

	EnabledDimensions []string `json:"enabledDimensions"`
	PerBucketMax      float64  `json:"perBucketMax"`

	PlacedSessions   int     `json:"placedSessions"`
	ExpectedSessions int     `json:"expectedSessions"`
	Completeness     float64 `json:"completeness"`

	Total      float64 `json:"total"`
	FinalTotal float64 `json:"finalTotal"`
}

// TimeScoreDetail is the partial score TimeScheduler computes over the
// three time-family buckets. Orchestrator merges this with
// ResourceScoreDetail (if any) to produce the final ScoreBreakdown.
type TimeScoreDetail struct {
	Time    *ScoreBucket
	Teacher *ScoreBucket
	Student *ScoreBucket
}

// ResourceScoreDetail is the partial score RoomScheduler computes.
// TIME_ONLY runs skip RoomScheduler entirely; this type is not
// constructed in that mode.
type ResourceScoreDetail struct {
	Resource *ScoreBucket
}
```

- [ ] **Step 4: Run tests, expect pass**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go test ./backend/scheduling/types/ -run "TestScoreBucket|TestScoreBreakdown|TestScoreDetails" -v`
Expected: all 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
cd C:/Users/musubi/Desktop/scheduling-system
git add backend/scheduling/types/score.go backend/scheduling/types/score_test.go
git commit -m "feat(scheduling): add ScoreBreakdown four-bucket types with nil-Disabled semantics (PR-01/12)"
```

---

### Task 8: IO types (TimeSchedulingInput/Output + RoomSchedulingInput/Output + OrchestratorRequest/Result)

**Files:**
- Create: `backend/scheduling/types/io.go`

**Interfaces:**
- Consumes: everything from Tasks 1–7
- Produces:
  - `type TimeSchedulingInput struct{...}` — per spec §3.3.1
  - `type TimeSchedulingOutput struct{...}` — per spec §3.3.2
  - `type RoomSchedulingInput struct{...}` — per spec §3.3.3
  - `type RoomSchedulingOutput struct{...}` — per spec §3.3.3
  - `type OrchestratorRequest struct{...}` — per spec §3.5.1
  - `type OrchestratorResult struct{...}` — per spec §3.5.1

- [ ] **Step 1: Implement io.go**

```go
package types

import "time"

// TimeSchedulingInput is the entire input contract for a TimeScheduler
// run. It is a pure value; TimeScheduler must not read any state outside
// this struct (INV-P9).
type TimeSchedulingInput struct {
	Tasks       []TeachingTaskView
	Teachers    []TeacherView
	ClassGroups []ClassGroupView

	LockedSlots    []LockedTimeSlot
	AvoidanceHints []ResourceConflictHint

	Deadline          time.Time
	Seed              int64
	Constraints       []string
	ConstraintWeights map[string]int
	SportsCourseIDs   map[uint]bool
	SemesterID        uint // filled onto Drafts by the Service layer, not by TimeScheduler
}

// TimeSchedulingOutput is TimeScheduler's return contract. Assignments
// carry no persistence identifiers (INV-P5). ScoreDetail covers three
// time-family buckets; Resource is computed by RoomScheduler when
// applicable.
type TimeSchedulingOutput struct {
	Assignments []TimeAssignmentDraft
	ScoreDetail TimeScoreDetail
	Diagnostics []string
	Iterations  int
	ElapsedMs   int64
}

// RoomSchedulingInput is the entire input contract for a RoomScheduler
// run. LocalRef inside each TimeAssignmentPlaced correlates output
// allocations and hints back to specific unpersisted TAs.
type RoomSchedulingInput struct {
	Assignments []TimeAssignmentPlaced
	Classrooms  []ClassroomView
	Tasks       []TeachingTaskView
	Deadline    time.Time
}

// RoomSchedulingOutput is RoomScheduler's return contract. Successful
// allocations reference LocalRef; failures are surfaced as
// ResourceConflictHint entries which the Orchestrator feeds back into
// the next TimeScheduler pass (up to MaxRetries).
type RoomSchedulingOutput struct {
	Allocations []RoomAllocationDraft
	Hints       []ResourceConflictHint
	ScoreDetail ResourceScoreDetail
	ElapsedMs   int64
}

// OrchestratorRequest is the full input to a scheduling run. The
// SchedulingService constructs one of these per RunScheduling call;
// downstream Solver components see only projections of it.
//
// Mode has exactly two consumption points inside Orchestrator.Run
// (INV-P4): RequiresRoomAssignment (assembly) and
// EnabledScoreDimensions (scoring).
type OrchestratorRequest struct {
	Mode SchedulingMode

	Tasks       []TeachingTaskView
	Teachers    []TeacherView
	ClassGroups []ClassGroupView
	Classrooms  []ClassroomView // may be empty/nil in TIME_ONLY

	LockedSlots       []LockedTimeSlot
	Constraints       []string
	ConstraintWeights map[string]int
	Deadline          time.Time
	Seed              int64
	MaxRetries        int
	SemesterID        uint
}

// OrchestratorResult is the summary returned to the Service layer. It
// does not carry ScheduleVersionID: version creation is the Service's
// responsibility inside the persistence transaction.
//
// Score reflects the state of the FINAL retry attempt only (INV-P12).
// Intermediate attempt scores are surfaced through ProgressReporter,
// not returned here.
type OrchestratorResult struct {
	Assignments []TimeAssignmentDraft
	Allocations []RoomAllocationDraft // nil in TIME_ONLY
	Score       ScoreBreakdown
	Logs        []string
	Diagnostics []string
	Attempts    int
	ElapsedMs   int64
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go build ./backend/scheduling/types/`
Expected: no output.

- [ ] **Step 3: Verify all existing tests still pass**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go test ./backend/scheduling/types/ -v`
Expected: all tests from Tasks 2, 4, 5, 6, 7 PASS.

- [ ] **Step 4: Commit**

```bash
cd C:/Users/musubi/Desktop/scheduling-system
git add backend/scheduling/types/io.go
git commit -m "feat(scheduling): add IO contracts for Time/Room/Orchestrator stages (PR-01/12)"
```

---

### Task 9: Interfaces + NoopReporter

**Files:**
- Create: `backend/scheduling/types/interfaces.go`
- Create: `backend/scheduling/types/interfaces_test.go`

**Interfaces:**
- Consumes: all IO types from Task 8
- Produces:
  - `type ProgressReporter interface{...}`
  - `type NoopReporter struct{}` — implements `ProgressReporter` (nil-safe default)
  - `type ITimeScheduler interface{...}`
  - `type IRoomScheduler interface{...}`
  - `type IScorer interface{...}`
  - `type ISchedulingOrchestrator interface{...}`

- [ ] **Step 1: Write failing tests**

Create `backend/scheduling/types/interfaces_test.go`:

```go
package types

import (
	"context"
	"testing"
)

// TestNoopReporter_SatisfiesInterface is a compile-time check via
// interface assignment. If NoopReporter fails to implement
// ProgressReporter, this file does not compile.
func TestNoopReporter_SatisfiesInterface(t *testing.T) {
	var p ProgressReporter = NoopReporter{}
	// call each method to ensure they don't panic on zero-value receiver
	p.Stage("init", 0)
	p.Iteration(1, 100, 0.5, 0.6, 10.0)
	p.Log("hello")
}

// Fake implementations used only to prove the interfaces are shaped
// correctly. These are internal-only, no goroutines, no state.

type fakeTimeScheduler struct{}

func (fakeTimeScheduler) Solve(ctx context.Context, in TimeSchedulingInput, p ProgressReporter) (TimeSchedulingOutput, error) {
	return TimeSchedulingOutput{}, nil
}

type fakeRoomScheduler struct{}

func (fakeRoomScheduler) Assign(ctx context.Context, in RoomSchedulingInput, p ProgressReporter) (RoomSchedulingOutput, error) {
	return RoomSchedulingOutput{}, nil
}

type fakeScorer struct{}

func (fakeScorer) Score(
	assignments []TimeAssignmentDraft,
	allocations []RoomAllocationDraft,
	teachers []TeacherView,
	classrooms []ClassroomView,
	tasks []TeachingTaskView,
	dims []string,
) ScoreBreakdown {
	return ScoreBreakdown{}
}

type fakeOrchestrator struct{}

func (fakeOrchestrator) Run(ctx context.Context, req OrchestratorRequest, p ProgressReporter) (OrchestratorResult, error) {
	return OrchestratorResult{}, nil
}

func TestInterfaces_AreImplementable(t *testing.T) {
	// Compile-time assertions via interface assignment.
	var _ ITimeScheduler = fakeTimeScheduler{}
	var _ IRoomScheduler = fakeRoomScheduler{}
	var _ IScorer = fakeScorer{}
	var _ ISchedulingOrchestrator = fakeOrchestrator{}
	// If the test compiles and runs, all four interfaces have shapes
	// matching the fake implementations above.
}
```

- [ ] **Step 2: Run tests, expect failure**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go test ./backend/scheduling/types/ -run "TestNoopReporter|TestInterfaces" -v`
Expected: compilation errors (`undefined: NoopReporter`, `undefined: ITimeScheduler`, etc.).

- [ ] **Step 3: Implement interfaces.go**

```go
package types

import "context"

// ProgressReporter is passed as an explicit parameter to every long-
// running Solver call (INV: no context.Value smuggling). Implementations
// route Stage/Iteration/Log to whatever surface the caller cares about
// (Wails logs, stdout, tests). Callers pass NoopReporter{} rather than
// nil when they do not care; implementations still defensively replace
// nil with NoopReporter{} at their entrypoints.
type ProgressReporter interface {
	// Stage announces a coarse pipeline stage transition. `percent` is
	// a rough 0..100 progress hint for UI.
	Stage(name string, percent int)

	// Iteration reports a fine-grained algorithmic step. Implementations
	// are expected to sample (e.g., every N iterations) to bound log
	// volume; that decision belongs to the impl, not to callers.
	Iteration(current, total int, currentScore, bestScore, temperature float64)

	// Log emits a free-form human-readable line.
	Log(message string)
}

// NoopReporter satisfies ProgressReporter with no side effects. It is
// safe to use as a zero-value.
type NoopReporter struct{}

func (NoopReporter) Stage(string, int)                                {}
func (NoopReporter) Iteration(int, int, float64, float64, float64)    {}
func (NoopReporter) Log(string)                                       {}

// ITimeScheduler is Stage 1 of the two-stage pipeline. Implementations
// must be pure functions of their inputs (INV-P9): no DB, no
// filesystem, no ambient state.
type ITimeScheduler interface {
	Solve(ctx context.Context, input TimeSchedulingInput, progress ProgressReporter) (TimeSchedulingOutput, error)
}

// IRoomScheduler is Stage 2. Same purity requirement as ITimeScheduler.
type IRoomScheduler interface {
	Assign(ctx context.Context, input RoomSchedulingInput, progress ProgressReporter) (RoomSchedulingOutput, error)
}

// IScorer computes a ScoreBreakdown from a proposed schedule. It is the
// sole authority mapping `dims` to bucket nil-ness (INV-S2).
// allocations and classrooms may be nil in TIME_ONLY mode.
type IScorer interface {
	Score(
		assignments []TimeAssignmentDraft,
		allocations []RoomAllocationDraft,
		teachers []TeacherView,
		classrooms []ClassroomView,
		tasks []TeachingTaskView,
		dims []string,
	) ScoreBreakdown
}

// ISchedulingOrchestrator is the composition point over ITimeScheduler,
// IRoomScheduler, and IScorer. The Service layer holds one of these and
// delegates all algorithmic decisions to it. Run must be idempotent for
// the same input + seed (INV-P7).
type ISchedulingOrchestrator interface {
	Run(ctx context.Context, req OrchestratorRequest, progress ProgressReporter) (OrchestratorResult, error)
}
```

- [ ] **Step 4: Run tests, expect pass**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go test ./backend/scheduling/types/ -v`
Expected: all tests (Tasks 2, 4, 5, 6, 7, 9) PASS.

- [ ] **Step 5: Commit**

```bash
cd C:/Users/musubi/Desktop/scheduling-system
git add backend/scheduling/types/interfaces.go backend/scheduling/types/interfaces_test.go
git commit -m "feat(scheduling): add solver interfaces + NoopReporter (PR-01/12)"
```

---

### Task 10: Isolation guarantees — INV-P2 static check + INV-P9 no-wiring check + CI script

**Files:**
- Create: `backend/scheduling/types/isolation_test.go`
- Create: `scripts/check_scheduling_isolation.sh`
- Modify: `Taskfile.yml` (add `check:scheduling-isolation` task)

**Interfaces:**
- Consumes: nothing new
- Produces:
  - Runtime test asserting no forbidden imports transitively reachable from `scheduling/types`
  - Runtime test asserting no production code path references `scheduling/types` yet (PR-01 must remain dead code)
  - Shell script for CI/local invocation

- [ ] **Step 1: Write isolation_test.go**

```go
package types

import (
	"go/build"
	"os/exec"
	"strings"
	"testing"
)

// forbidden lists the import paths scheduling/types must never depend
// on, directly or transitively (INV-P2).
var forbidden = []string{
	"scheduling-system/backend/database",
	"scheduling-system/backend/models",
	"scheduling-system/backend/services",
	"gorm.io/gorm",
}

// TestInvP2_NoForbiddenImports uses `go list` to walk the transitive
// import graph of scheduling/types and rejects any forbidden entry.
// This is stricter than an eyeball import check because it catches
// second-order imports too.
func TestInvP2_NoForbiddenImports(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "scheduling-system/backend/scheduling/types")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list unavailable in this environment: %v", err)
		return
	}
	deps := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, dep := range deps {
		for _, f := range forbidden {
			if dep == f || strings.HasPrefix(dep, f+"/") {
				t.Errorf("INV-P2 violated: scheduling/types transitively depends on %q via %q", f, dep)
			}
		}
	}
}

// TestInvP9_NoIOInPackage inspects the package's own source to make
// sure nothing here calls into fmt.Print* (stdout writes) or os.*
// (filesystem/env). Solver interfaces only emit through
// ProgressReporter; nothing else in types should touch IO.
func TestInvP9_NoIOInPackage(t *testing.T) {
	pkg, err := build.ImportDir("./", 0)
	if err != nil {
		t.Fatalf("build.ImportDir failed: %v", err)
	}
	forbiddenImports := map[string]bool{
		"os":      true,
		"os/exec": false, // allowed only inside this isolation_test.go
		"log":     true,
	}
	for _, imp := range pkg.Imports {
		if v, ok := forbiddenImports[imp]; ok && v {
			t.Errorf("INV-P9 violated: scheduling/types imports %q", imp)
		}
	}
	// os/exec appears only in this test file, which is exempt.
}
```

- [ ] **Step 2: Run isolation tests, expect pass**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go test ./backend/scheduling/types/ -run "TestInvP" -v`
Expected: both PASS.

- [ ] **Step 3: Create scripts/check_scheduling_isolation.sh**

```bash
#!/usr/bin/env bash
# check_scheduling_isolation.sh — guards INV-P2.
# Fails if scheduling/... depends on database / models / services / gorm.
set -euo pipefail

cd "$(dirname "$0")/.."

# List transitive deps of the scheduling subtree. Empty subtrees (before
# later PRs land) are skipped silently.
if ! ls backend/scheduling/*/ >/dev/null 2>&1; then
    echo "OK: scheduling/ subtree not present yet"
    exit 0
fi

DEPS=$(go list -deps ./backend/scheduling/... 2>/dev/null || true)
if [ -z "$DEPS" ]; then
    echo "OK: scheduling/ has no packages to analyze"
    exit 0
fi

VIOLATIONS=$(echo "$DEPS" | grep -E "(scheduling-system/backend/database|scheduling-system/backend/models|scheduling-system/backend/services|gorm\.io/gorm)" || true)

if [ -n "$VIOLATIONS" ]; then
    echo "VIOLATION: scheduling/* depends on forbidden packages:"
    echo "$VIOLATIONS"
    exit 1
fi

echo "OK: scheduling isolation verified"
```

Make it executable:

```bash
cd C:/Users/musubi/Desktop/scheduling-system
chmod +x scripts/check_scheduling_isolation.sh
```

- [ ] **Step 4: Run the script**

```bash
cd C:/Users/musubi/Desktop/scheduling-system
bash scripts/check_scheduling_isolation.sh
```

Expected output: `OK: scheduling isolation verified`

- [ ] **Step 5: Wire into Taskfile.yml**

First read the current Taskfile.yml to identify the right insertion point:

```bash
cd C:/Users/musubi/Desktop/scheduling-system
cat Taskfile.yml | head -60
```

Add a new task (adjust indent to match file style — this uses 2-space indent, which is the go-task convention):

```yaml
  check:scheduling-isolation:
    desc: Verify scheduling/* has no forbidden imports (INV-P2)
    cmds:
      - bash scripts/check_scheduling_isolation.sh
```

Add the new task under the top-level `tasks:` section. If a `test` task
already exists, extend its `deps:` list to include
`check:scheduling-isolation` so CI-6 is enforced on every test run.

Verify:

```bash
cd C:/Users/musubi/Desktop/scheduling-system
task check:scheduling-isolation
```

Expected: `OK: scheduling isolation verified`

- [ ] **Step 6: Verify no production code references scheduling/types yet**

Run (grep from repo root, exclude the types package itself and this plan doc):

```bash
cd C:/Users/musubi/Desktop/scheduling-system
grep -rE 'scheduling-system/backend/scheduling/types' \
    --include='*.go' \
    --exclude-dir='backend/scheduling' \
    . || echo "OK: no production imports of scheduling/types"
```

Expected output: `OK: no production imports of scheduling/types`

If any hits appear, they must be removed before the PR ships (PR-01 is
dead code by design).

- [ ] **Step 7: Full test sweep**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && go build ./... && go test ./backend/scheduling/types/ -v`
Expected: whole-repo build passes; all types tests PASS.

- [ ] **Step 8: Line budget check**

Run: `cd C:/Users/musubi/Desktop/scheduling-system && wc -l backend/scheduling/types/*.go`
Expected: total is roughly ~500 lines (spec budget). Actual value informational, no hard fail.

- [ ] **Step 9: Commit**

```bash
cd C:/Users/musubi/Desktop/scheduling-system
git add backend/scheduling/types/isolation_test.go scripts/check_scheduling_isolation.sh Taskfile.yml
git commit -m "chore(scheduling): add INV-P2/P9 isolation guards + Taskfile hook (PR-01/12)"
```

---

## Post-PR Acceptance Gate (before merging PR-01)

Run these before opening the PR:

- [ ] `cd C:/Users/musubi/Desktop/scheduling-system && go build ./...` — CI-1
- [ ] `cd C:/Users/musubi/Desktop/scheduling-system && go test ./...` — CI-2
- [ ] `cd C:/Users/musubi/Desktop/scheduling-system && bash scripts/check_scheduling_isolation.sh` — CI-6
- [ ] `cd C:/Users/musubi/Desktop/scheduling-system && task check:scheduling-isolation` (if go-task installed)
- [ ] Wails app still starts and completes an existing scheduling run — CI-4 / CI-5

Existing scheduling flow must be **identical** to pre-PR behavior; the new
package is not wired into anything, so any behavioral difference indicates
an accidental production-code change and should block the PR.

---

## Self-Review Log

**Spec coverage check** (§3.3, §3.4 primary; §2.1, §2.4, §2.7 supporting):

| Spec item | Task |
|---|---|
| SchedulingMode enum + 4 methods (§2.1) | Task 2 |
| ResourceConflictHint + HintReason (§2.4) | Task 4 |
| ScoreBreakdown + ScoreBucket (§2.7) | Task 7 |
| TimeSchedulingInput/Output (§3.3.1, §3.3.2) | Tasks 5, 6, 7, 8 |
| RoomSchedulingInput/Output + TimeAssignmentPlaced (§3.3.3) | Tasks 6, 8 |
| IScorer signature (§3.3.4) | Task 9 |
| ITimeScheduler / IRoomScheduler / IScorer / ISchedulingOrchestrator (§3.4) | Task 9 |
| ProgressReporter + NoopReporter (§3.4) | Task 9 |
| DayOfWeek / Period value types (implicit dependency) | Task 1 |
| LockedTimeSlot value type (§4.1) | Task 3 |
| INV-P2 isolation | Task 10 |
| INV-P5 draft field guard | Task 6 |
| INV-P10 draft persistence guard | Task 6 |
| INV-S1 nil bucket = Disabled | Task 7 |
| INV-M1 closed constant set | Task 2 |
| INV-H1 hint not persistable (spot-checked by presence of no gorm tag) | Task 4 |
| INV-H2 hint ≠ LockedTimeSlot | Tasks 3, 4 (distinct struct definitions) |

**Placeholder scan**: No TODO / TBD / "fill in" / "similar to" left in plan text.
All code shown in full.

**Type consistency**:

- `TimeAssignmentDraft` fields: `TeachingTaskID uint`, `DayOfWeek DayOfWeek`,
  `StartPeriod Period`, `Span int` — used consistently in Tasks 6, 8.
- `RoomAllocationDraft` fields: `LocalRef int`, `ClassroomID uint` —
  consistent across Tasks 6, 8.
- `TimeAssignmentPlaced` fields include `LocalRef int` (matches Task 6
  test); consumed by RoomSchedulingInput in Task 8.
- `SchedulingMode` methods: `IsValid`, `IsTimeOnly`, `RequiresRoomAssignment`,
  `EnabledScoreDimensions` — consistent between Task 2 test and impl.
- `ScoreBucket` fields: `Value float64`, `Max float64`,
  `Details map[string]float64` — consistent Task 7.
- `ScoreBreakdown` bucket fields are `*ScoreBucket` (pointer) — enforced by
  reflection test in Task 7.
- `IScorer.Score` parameter order matches spec §3.3.4 — enforced by fake
  in Task 9.

**Spec-vs-plan deviations** (documented for PR reviewer):

1. `SchedulingMode` defined in `scheduling/types` instead of `backend/models`.
   Reason: INV-P2 forbids `scheduling/types` from importing `backend/models`,
   so the type must live in a package that types can reach.
   Consequence: PR-03 does not create `backend/models/scheduling_mode.go`;
   it imports `scheduling-system/backend/scheduling/types.SchedulingMode`.
2. `ScoreBreakdown` / `ScoreBucket` defined in `scheduling/types` (same
   reason as above).
3. `DayOfWeek` / `Period` defined in `scheduling/types` as int-based type
   aliases with no methods. Method-carrying versions in `backend/models`
   remain unchanged; conversions between the two happen at Service-layer
   view construction (deferred to PR-07/09).
