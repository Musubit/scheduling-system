# v0.5.5 Dual-Mode Scheduling Architecture — Design Spec

- **Status**: Frozen (v0.5.5 Phase 2 SSoT)
- **Date**: 2026-07-13
- **Scope**: Split the current time+room joint scheduling engine into a two-stage
  layered pipeline supporting `FULL_SCHEDULING` and `TIME_ONLY_SCHEDULING` modes.
- **Non-goals**: UI redesign; new UX beyond mode-aware column/score rendering;
  migration of legacy snapshot content back to the new model.

This document is the **single source of truth** for the v0.5.5 dual-mode
scheduling refactor. Every implementation PR must be traceable to a section
here. Any deviation requires an explicit spec amendment and a new frozen
invariant.

---

## Table of Contents

1. [Overall Architecture](#1-overall-architecture)
2. [Domain Model & Database Schema](#2-domain-model--database-schema)
3. [Pipeline Component Design](#3-pipeline-component-design)
4. [Solver Internal Design](#4-solver-internal-design)
5. [Migration & Implementation Plan](#5-migration--implementation-plan)
6. [Frontend Migration Boundary](#6-frontend-migration-boundary)
7. [Consolidated Invariants](#7-consolidated-invariants)

---

## 1. Overall Architecture

### 1.1 High-Level Structure

```
┌─────────────────────────────────────────────────────────────┐
│                  SchedulingService (Facade)                 │
│                                                             │
│    SchedulingConfig                                         │
│      Mode: FULL_SCHEDULING | TIME_ONLY_SCHEDULING           │
│                                                             │
│    SchedulingOrchestrator  (Two-Stage Pipeline)             │
│                                                             │
│      for attempt in 1..K=3:                                 │
│        times = TimeScheduler.Solve(                 ← Stage 1│
│                 tasks, teachers, classes,                   │
│                 lockedSlots, avoidanceHints,                │
│                 enabledTimeConstraints)                     │
│                                                             │
│        if Mode == TIME_ONLY: return times                   │
│                                                             │
│        rooms, feedback = RoomScheduler.Assign(     ← Stage 2│
│                           times, classrooms,                │
│                           resourceRules)                    │
│                                                             │
│        if feedback.hints empty: return rooms                │
│        avoidanceHints.union(feedback.hints)                 │
│      return partial result + unplaced diagnostics           │
│                                                             │
│    Persistence:                                             │
│      TimeAssignments  ←── ScheduleEntries                   │
│      (Stage 1 output)     (Stage 2 output)                  │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 Three Self-Contained Units

| Unit | Responsibility | Input | Output | Dependencies |
|---|---|---|---|---|
| **TimeScheduler** | Feasibility over teachers/classes/tasks/time + time soft-constraint optimization | `TeachingTask`, `Teacher`, `ClassGroup`, `LockedTimeSlot`, `ResourceConflictHint[]`, `constraints[]` | `[]TimeAssignment` | Does NOT depend on `Classroom` |
| **RoomScheduler** | For each `TimeAssignment`, pick a classroom that satisfies resource constraints | `[]TimeAssignment`, `[]Classroom`, `ResourceRules` | `[]ScheduleEntry` + `[]ResourceConflictHint` | Does NOT depend on `SchedulingMode` |
| **SchedulingOrchestrator** | Compose pipeline per Mode + feedback retry + persistence transaction | `SchedulingConfig` | `SchedulingResult` | Depends on TimeScheduler + RoomScheduler + Scorer |

### 1.3 SchedulingMode as Single Source of Truth

```go
type SchedulingMode string

const (
    ModeFullScheduling     SchedulingMode = "FULL_SCHEDULING"
    ModeTimeOnlyScheduling SchedulingMode = "TIME_ONLY_SCHEDULING"
)

type SchedulingConfig struct {
    // existing fields...
    Mode       SchedulingMode `json:"mode"`
    MaxRetries int            `json:"maxRetries"` // default 3, orchestrator retry cap
}
```

**Rule**: The Orchestrator selects between assembling one stage vs two based on
`mode.RequiresRoomAssignment()`. This is an **assembly decision** (which
components are wired in), not a **behavioral branch** (an `if` inside a coupled
function). Mode is consumed **only** by `SchedulingService` (for defaulting)
and `SchedulingOrchestrator` (for assembly + scoring dimensions). Solver
components never see it.

### 1.4 Core Architectural Invariants (I1–I10)

Every subsequent section respects these:

| # | Invariant | Signal of violation |
|---|---|---|
| I1 | `SchedulingMode` is the sole source of mode state; no secondary field | Any `EnableRoomAssignment` / `has_classroom` / `mode_hint` |
| I2 | TimeScheduler signature and impl reference no `Classroom` / `ClassroomID` / room type / capacity | `[]Classroom` in TimeScheduler parameter list |
| I3 | RoomScheduler signature and impl reference no `SchedulingMode` | `if mode == ...` inside RoomScheduler |
| I4 | Stage 2 communicates with Stage 1 only through return values (`ResourceConflictHints`); no direct calls or references | `*TimeScheduler` field on RoomScheduler struct or vice versa |
| I5 | `ResourceConflictHint` and `LockedTimeSlot` are distinct types; no mutual assignment | Same type alias for both |
| I6 | `TimeAssignment` definition, CRUD, migration are mode-agnostic | `mode` column on time_assignments |
| I7 | `ScheduleEntry` keeps only `semester_id / time_assignment_id / classroom_id + timestamps`; no redundant time fields | day/period/teacher_id columns on schedule_entries |
| I8 | Score buckets (Time/Teacher/Student/Resource) are gated by `EnabledDimensions`; TIME_ONLY yields `Resource = nil` (Disabled), not 0 or 100 | Missing field cannot distinguish disabled from 0 |
| I9 | Orchestrator retry cap `K=3` is configurable, not a magic number; exhausting K yields partial result + unplaced diagnostics, not silent failure | Hardcoded `3` inside orchestrator body |
| I10 | Clean migration: DROP schedule_entries + CREATE new tables; legacy snapshots/versions marked `v0.5.4` are read-only, not transformed | Any migration up/down that rewrites legacy rows |

---

## 2. Domain Model & Database Schema

### 2.1 `models.SchedulingMode`

```go
type SchedulingMode string

const (
    ModeFullScheduling     = "FULL_SCHEDULING"
    ModeTimeOnlyScheduling = "TIME_ONLY_SCHEDULING"
)

// Only these four methods may inspect a SchedulingMode value.
func (m SchedulingMode) IsValid() bool
func (m SchedulingMode) IsTimeOnly() bool
func (m SchedulingMode) RequiresRoomAssignment() bool
func (m SchedulingMode) EnabledScoreDimensions() []string // ["time","teacher","student"] or [...+"resource"]
```

- String enum, not int — Wails-friendly, audit-friendly.
- All code outside these four methods is forbidden from `switch`ing on the raw
  string value.

**INV-M1**: `SchedulingMode` constant set is closed. Any file using a mode
string literal that is not defined here violates this invariant.

### 2.2 `models.TimeAssignment` (new table)

| Field | Type | Constraint | Meaning |
|---|---|---|---|
| `ID` | uint | PK | |
| `SemesterID` | uint | FK, NOT NULL, index | Query scope |
| `ScheduleVersionID` | uint | FK, NOT NULL, index | Version ownership |
| `TeachingTaskID` | uint | FK, NOT NULL, index | JOIN source for teacher / course / class |
| `DayOfWeek` | DayOfWeek | NOT NULL | 0=Mon..6=Sun |
| `StartPeriod` | Period | NOT NULL | 0..10 |
| `Span` | int | NOT NULL, default 2 | 1..3 |
| `CreatedAt` / `UpdatedAt` / `DeletedAt` | | GORM standard | Soft delete supported |

**Explicitly absent**: `TeacherID`, `CourseID`, `ClassroomID`, `Weeks`, `Mode`.

**Indexes**:
- `idx_ta_semester_version` on `(semester_id, schedule_version_id)`
- `idx_ta_version_task` on `(schedule_version_id, teaching_task_id)`
- `idx_ta_version_day_period` on `(schedule_version_id, day_of_week, start_period)`

**Invariants**:
- **INV-T1**: One TA row = one weekly session.
- **INV-T2**: TA structure is identical across both modes.
- **INV-T3**: A TA existing does not mean scheduling is complete; completeness
  is determined by the presence of an associated `ScheduleEntry`.
- **INV-T4**: Every TA row belongs to a `ScheduleVersion`; `ScheduleVersionID`
  is NOT NULL; TAs never span versions.

### 2.3 `models.ScheduleEntry` (refactored, minimal)

> `ScheduleEntry` is the **Resource Allocation Entity** in the v0.5.5 two-stage
> scheduling pipeline. It represents "a room has been allocated to a scheduled
> time slot"; it does not carry time/teacher/course/class semantics. Those
> live in the referenced `TimeAssignment`. Created only by `RoomScheduler` in
> FULL_SCHEDULING mode.

| Field | Type | Constraint | Meaning |
|---|---|---|---|
| `ID` | uint | PK | |
| `SemesterID` | uint | FK, NOT NULL, index | Semester scope |
| `ScheduleVersionID` | uint | FK, NOT NULL, index | Aligned with TA |
| `TimeAssignmentID` | uint | FK, **UNIQUE**, NOT NULL | One-to-one with TA |
| `ClassroomID` | uint | FK, NOT NULL, index | Allocated room |
| `CreatedAt` / `UpdatedAt` / `DeletedAt` | | GORM standard | |

**Why redundant `ScheduleVersionID` on Entry despite being derivable from TA**:

1. Soft-delete alignment (avoids orphan Entry when TA is soft-deleted).
2. Index efficiency for snapshot queries.
3. Actively-asserted integrity: `Entry.version == TA.version` is verifiable
   without a JOIN.

**Invariants**:
- **INV-E1**: In TIME_ONLY mode, `schedule_entries` has zero rows for the
  current version (soft-deleted rows from prior versions may exist).
- **INV-E2**: `Entry.SemesterID == Entry.TimeAssignment.SemesterID`.
- **INV-E3**: Only `(TimeAssignmentID, ClassroomID)` carries semantic value on
  Entry; other fields are audit metadata.
- **INV-E4**: `Entry.ScheduleVersionID == Entry.TimeAssignment.ScheduleVersionID`
  strictly. Asserted at write path + before snapshot serialization.

### 2.4 `services.ResourceConflictHint` (in-memory DTO)

```go
type ResourceConflictHint struct {
    TeachingTaskID uint
    DayOfWeek      DayOfWeek
    StartPeriod    Period
    Span           int
    Reason         HintReason
    Detail         string
}

type HintReason string
const (
    ReasonNoCapacity     HintReason = "no_room_with_capacity"
    ReasonNoMatchingType HintReason = "no_room_of_required_type"
    ReasonAllOccupied    HintReason = "all_matching_rooms_occupied"
    ReasonEquipmentMiss  HintReason = "no_room_with_equipment"
)
```

**Lifecycle**: created by RoomScheduler → consumed by Orchestrator retry loop
→ logged into `SchedulingResult.Logs` → discarded at run end. Never persisted.

**Invariants**:
- **INV-H1**: `ResourceConflictHint` has no GORM tags; persistence is forbidden.
- **INV-H2**: `ResourceConflictHint` and `LockedTimeSlot` share no struct
  embedding or type alias; forced-copy across the boundary.

### 2.5 `dto.ScheduleSnapshotDTO` (new package)

```go
type ScheduleSnapshotDTO struct {
    SchemaVersion     string                    // "v0.5.5"
    SemesterID        uint
    SemesterName      string
    Mode              SchedulingMode
    ScheduleVersionID uint
    CreatedAt         time.Time
    Assignments       []ScheduledAssignmentDTO
    Score             ScoreBreakdown
}

type ScheduledAssignmentDTO struct {
    // Time layer (from TimeAssignment)
    TeachingTaskID  uint
    TeacherID       uint
    TeacherName     string
    CourseID        uint
    CourseName      string
    ClassGroupIDs   []uint
    ClassGroupNames []string
    DayOfWeek       DayOfWeek
    StartPeriod     Period
    Span            int
    WeekRange       string  // "1-16"

    // Resource layer (from ScheduleEntry; absent in TIME_ONLY)
    ClassroomID     *uint
    ClassroomName   *string
    ClassroomFloor  *int
    ClassroomType   *string
}
```

**Rules**:

1. DTO carries no GORM tags; the entire object graph is a snapshot value.
2. `omitempty + pointer` expresses classroom absence in TIME_ONLY.
3. Fields are the mirror at snapshot time; source data changes do not mutate
   a stored DTO.
4. `SchemaVersion` gates upgrade; mismatched values enter a legacy readonly path.
5. There is no `DTO → models` mapping; the DTO is a terminal serialization
   value.

**Invariants**:
- **INV-SN1**: Snapshot / Version JSON content is exactly `ScheduleSnapshotDTO`.
  `SnapshotService` and `VersionService` serialize DTO instances only;
  `Marshal(entries)` or `Marshal(models.X)` is forbidden.
- **INV-SN2**: DTO → models has no reverse-mapping API; historical snapshots
  cannot be reconstituted as editable models.

### 2.6 `SchedulingConfig` Changes

**Added**:
```go
Mode       SchedulingMode `json:"mode"`
MaxRetries int            `json:"maxRetries"`
```

**Removed**: None (`EnableRoomAssignment` was never introduced).

**Defaulting**:
- Empty `Mode` → `ModeFullScheduling`, logged as warning, once at
  `RunScheduling` entrypoint.
- `MaxRetries <= 0` → `3`, at the same entrypoint.

**Invariants**:
- **INV-C1**: `SchedulingConfig` has no mode-related field other than `Mode`.
- **INV-C2**: Mode/MaxRetries defaulting occurs exactly once in
  `RunScheduling`; downstream code assumes them populated.

### 2.7 `ScoreBreakdown` Four-Bucket Refactor

```go
type ScoreBreakdown struct {
    Time     *ScoreBucket  // nil = Disabled
    Teacher  *ScoreBucket
    Student  *ScoreBucket
    Resource *ScoreBucket  // always nil in TIME_ONLY

    EnabledDimensions []string
    PerBucketMax      float64

    PlacedSessions   int
    ExpectedSessions int
    Completeness     float64

    Total      float64  // sum of non-nil buckets
    FinalTotal float64  // Total × completeness factor
}

type ScoreBucket struct {
    Value   float64
    Max     float64
    Details map[string]float64  // sub-constraint scores
}
```

**Bucket ownership** (frozen mapping):

| Bucket | Sub-constraints |
|---|---|
| Time | `course_dispersed`, `avoid_saturday`, `avoid_sunday`, `pe_preferred_periods` |
| Teacher | `teacher_preference`, `teacher_days_limit` |
| Student | `student_fatigue` |
| Resource | `low_floor_preference` |

**Invariants**:
- **INV-S1**: No sentinel value expresses Disabled; only `nil` pointer does.
- **INV-S2**: `EnabledDimensions` and the set of non-nil buckets are mutually
  exclusive and exhaustive.
- **INV-S3**: Bucket-to-sub-constraint mapping is a code constant, not
  configurable.

### 2.8 Migration Strategy

**Meta table**:
```sql
CREATE TABLE schema_migrations (
    version    TEXT PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL
);
```

**Migration procedure** (single transaction):
```
current := max(version) FROM schema_migrations
if current < "v0.5.5":
    BEGIN TX
      DROP TABLE IF EXISTS schedule_entries
      CREATE TABLE time_assignments (...)
      CREATE TABLE schedule_entries  (...v0.5.5 schema...)
      ALTER TABLE schedule_snapshots ADD COLUMN schema_version TEXT DEFAULT 'v0.5.4'
      ALTER TABLE schedule_versions  ADD COLUMN schema_version TEXT DEFAULT 'v0.5.4'
      ALTER TABLE schedule_versions  ADD COLUMN mode TEXT DEFAULT 'FULL_SCHEDULING'
      INSERT INTO schema_migrations VALUES ('v0.5.5', NOW())
    COMMIT
```

**Legacy handling**: Old snapshot / version rows retain their JSON content
untouched, tagged `schema_version = 'v0.5.4'`. UI marks them as legacy and
disables restore. Content is **never** deserialized into runtime models
(INV-MI2).

**Invariants**:
- **INV-MI1**: `schema_migrations` row `version='v0.5.5'` marks migration
  complete. Table existence alone is insufficient.
- **INV-MI2**: Legacy `entries_json` is never deserialized back into a runtime
  model object.
- **INV-MI3**: Migration steps run in a single transaction; partial state is
  impossible.

---

## 3. Pipeline Component Design

### 3.1 Component Responsibility Table

| Component | Input | Output | Read | Write | Catch |
|---|---|---|---|---|---|
| **TimeScheduler** | `TimeSchedulingInput` | `TimeSchedulingOutput` | input params only | none | internal algorithm errors → `error` |
| **RoomScheduler** | `RoomSchedulingInput` | `RoomSchedulingOutput` | input params only | none | internal matching errors → `error` |
| **SchedulingOrchestrator** | `OrchestratorRequest` | `OrchestratorResult` | input params only | none | downstream errors → SchedulingResult |
| **SchedulingService (Facade)** | `SchedulingConfig` + resources | `SchedulingResult` | DB | TA/Entry/Version | all errors + persistence |

### 3.2 Dependency Injection

```
SchedulingService (Wails Facade)
  ├── db, snapshots, versions, orchestrator
  │
  └── SchedulingOrchestrator (no DB, no snapshot, no wails)
        ├── timeScheduler  ITimeScheduler
        ├── roomScheduler  IRoomScheduler
        └── scorer         IScorer
```

Solvers implement interfaces; the Orchestrator depends only on interfaces.
This enables:
- SA and OR-Tools as parallel `ITimeScheduler` implementations, mediated
  internally by a `Driver`.
- Fake solvers for Orchestrator unit tests with no database.

### 3.3 Input/Output DTOs

#### 3.3.1 TimeSchedulingInput

```go
type TimeSchedulingInput struct {
    Tasks             []TeachingTaskView
    Teachers          []TeacherView
    ClassGroups       []ClassGroupView

    LockedSlots       []LockedTimeSlot
    AvoidanceHints    []ResourceConflictHint

    Deadline          time.Time
    Seed              int64
    Constraints       []string
    ConstraintWeights map[string]int
    SportsCourseIDs   map[uint]bool
    SemesterID        uint
}
```

`TeachingTaskView` is a plain value type stripped of GORM concepts. It carries
`RequiredRoomType` and `AllowedRoomIDs` but TimeScheduler ignores those fields
— shared view keeps Service-layer conversion cost low; projection isolation
is enforced by INV-P1/P2 at compile time.

#### 3.3.2 TimeSchedulingOutput

```go
type TimeSchedulingOutput struct {
    Assignments []TimeAssignmentDraft   // no ID, no version_id, no semester_id
    ScoreDetail TimeScoreDetail          // 3 buckets: Time+Teacher+Student
    Diagnostics []string
    Iterations  int
    ElapsedMs   int64
}

type TimeAssignmentDraft struct {
    TeachingTaskID uint
    DayOfWeek      DayOfWeek
    StartPeriod    Period
    Span           int
}
```

#### 3.3.3 RoomSchedulingInput / Output

```go
type RoomSchedulingInput struct {
    Assignments []TimeAssignmentPlaced  // LocalRef for correlation
    Classrooms  []ClassroomView
    Tasks       []TeachingTaskView
    Deadline    time.Time
}

type RoomSchedulingOutput struct {
    Allocations []RoomAllocationDraft
    Hints       []ResourceConflictHint
    ScoreDetail ResourceScoreDetail
    ElapsedMs   int64
}
```

Drafts contain no persisted IDs; correlation is via `LocalRef` (an index
assigned by the Orchestrator).

#### 3.3.4 Scorer

```go
type IScorer interface {
    Score(assignments []TimeAssignmentDraft,
          allocations []RoomAllocationDraft,   // nil in TIME_ONLY
          teachers []TeacherView,
          classrooms []ClassroomView,          // nil in TIME_ONLY
          tasks []TeachingTaskView,
          dims []string) ScoreBreakdown
}
```

The scorer alone maps `dims` to bucket nil-ness (INV-S2).

### 3.4 Minimal Interfaces

```go
type ITimeScheduler interface {
    Solve(ctx context.Context,
          input TimeSchedulingInput,
          progress ProgressReporter) (TimeSchedulingOutput, error)
}

type IRoomScheduler interface {
    Assign(ctx context.Context,
           input RoomSchedulingInput,
           progress ProgressReporter) (RoomSchedulingOutput, error)
}

type IScorer interface {
    Score(...) ScoreBreakdown
}

type ISchedulingOrchestrator interface {
    Run(ctx context.Context,
        req OrchestratorRequest,
        progress ProgressReporter) (OrchestratorResult, error)
}
```

`ProgressReporter` is an **explicit** parameter, never injected via
`context.Value`.

```go
type ProgressReporter interface {
    Stage(name string, percent int)
    Iteration(current, total int, currentScore, bestScore, temperature float64)
    Log(message string)
}
type NoopReporter struct{} // nil-safe default
```

Callers must pass `NoopReporter{}` when they do not care. `nil` is defensively
replaced at every implementation entrypoint.

### 3.5 Orchestrator Main Loop

```
func (o *Orchestrator) Run(ctx, req, progress) (OrchestratorResult, error):
    if progress == nil: progress = NoopReporter{}

    dims      := req.Mode.EnabledScoreDimensions()
    needsRoom := req.Mode.RequiresRoomAssignment()

    var hints []ResourceConflictHint
    var lastTime TimeSchedulingOutput
    var lastRoom RoomSchedulingOutput
    var attempts int

    for attempt := 1; attempt <= req.MaxRetries; attempt++:
        attempts = attempt

        timeIn := buildTimeInput(req, hints)
        timeOut, err := o.timeScheduler.Solve(ctx, timeIn, progress)
        if err != nil: return {}, wrapErr("time stage", err)
        lastTime = timeOut

        if !needsRoom: break

        placed := toPlaced(timeOut.Assignments, req.Tasks)
        roomOut, err := o.roomScheduler.Assign(ctx, RoomSchedulingInput{
            Assignments: placed, Classrooms: req.Classrooms,
            Tasks: req.Tasks, Deadline: req.Deadline,
        }, progress)
        if err != nil: return {}, wrapErr("room stage", err)
        lastRoom = roomOut

        if len(roomOut.Hints) == 0: break

        hints = append(hints, roomOut.Hints...)

    score := o.scorer.Score(
        lastTime.Assignments, lastRoom.Allocations,
        req.Teachers, req.Classrooms, req.Tasks, dims,
    )

    return OrchestratorResult{
        Assignments: lastTime.Assignments,
        Allocations: lastRoom.Allocations,
        Score:       score,
        Attempts:    attempts,
        // ...
    }, nil
```

Mode has exactly two consumers: `EnabledScoreDimensions()` and
`RequiresRoomAssignment()`. Hints accumulate across attempts; no oscillation.

### 3.6 SchedulingService Facade Duties

`SchedulingService.RunScheduling(config)`:

1. Load DB resources (GORM models).
2. Convert to View DTOs (models → types).
3. Build `OrchestratorRequest` (including `config.Mode`).
4. Call `orchestrator.Run(ctx, req, reporter)`.
5. Transaction A begins:
   a. Create `ScheduleVersion` (source=AutoGenerate, mode=config.Mode).
   b. Set prior current version `IsCurrent=false`.
   c. Convert `TimeAssignmentDraft` → `models.TimeAssignment` with version_id
      + semester_id. Batch insert; retrieve real IDs.
   d. FULL mode: convert `RoomAllocationDraft` → `models.ScheduleEntry`
      with version_id + resolved TA_id. Batch insert.
   e. Assert INV-E4 (Entry.version == TA.version).
6. Transaction A commits.
7. Build `dto.ScheduleSnapshotDTO` from Orchestrator result + DB enrichment.
8. `SnapshotService.CreateSnapshotFromDTO(dto)` (independent transaction).
9. `VersionService.SetEntriesJSONFromDTO(versionID, dto)` (independent).
10. Return Wails result.

### 3.7 Transaction Boundaries

| Stage | Transaction | Owner | Notes |
|---|---|---|---|
| Orchestrator.Run | none | Orchestrator | pure computation |
| Create Version, TA, Entry | TX A | SchedulingService | atomic |
| INV-E4 assertion | TX A | SchedulingService | rollback on failure |
| Snapshot write | independent TX | SnapshotService | log on failure, no rollback of A |
| Version.EntriesJSON write | independent TX | VersionService | log on failure, no rollback of A |

Rationale: The authoritative representation is TA + Entry; snapshot and
version JSON are audit copies. Their failure must not erase the just-computed
schedule.

### 3.8 Error Classification

| Layer | Error Type | Propagation | User-visible |
|---|---|---|---|
| Solver internal | algorithm error / ctx cancel | return `error` | yes (SchedulingResult.Error) |
| Solver partial failure | unplaced TA / Hints | return-value field | yes (Diagnostics) |
| Orchestrator | component error, timeout | return `error` | yes |
| Service transaction | DB error, invariant assertion | Go error + rollback | yes |
| Snapshot/Version side-effect | independent TX | log warning + swallow | logs only |

Partial success is **not** an error. A run returning 95/100 tasks with 5
unplaced entries is `error == nil` with populated `Diagnostics`.

### 3.9 Orchestrator DB-Free Unit Testing

| # | Scenario | Assertion |
|---|---|---|
| O1 | TIME_ONLY | RoomScheduler.Assign never called; 3-bucket score only |
| O2 | FULL first-try success | 1 Time + 1 Room; no retries |
| O3 | FULL retry succeeds | 2 Time (with hints) + 2 Room; converges |
| O4 | FULL K rounds exhausted | Attempts == MaxRetries; partial result + Diagnostics |
| O5 | Time solver errors | Orchestrator returns error; Room not called |
| O6 | ctx cancellation | Immediate return of ctx.Err(); no retry |
| O7 | Hints accumulate | Attempt-3 input has hints from attempts 1+2 |
| O8 | FULL with empty Classrooms | Orchestrator accepts; RoomScheduler returns Hints for all |
| O9 | Bucket nil-ness | TIME_ONLY yields Resource=nil, FULL yields non-nil |
| O10 | Idempotence | Same seed + input → identical business-field output |

### 3.10 Integration Tests

| # | Scenario | Assertion |
|---|---|---|
| I1 | TIME_ONLY full flow | time_assignments has rows; schedule_entries zero; Version.Mode == TIME_ONLY |
| I2 | FULL full flow | TA + Entry one-to-one; INV-E4 holds |
| I3 | No Classroom data + TIME_ONLY | Succeeds without "missing classroom" error |
| I4 | No Classroom data + FULL | Fails cleanly with informative message |
| I5 | Consecutive scheduling runs | Prior version IsCurrent=false; new version current |
| I6 | Snapshot content | `snapshot.entries_json` deserializes as `ScheduleSnapshotDTO` |
| I7 | HBUT real data + FULL | Success rate & score within ±5% of v0.5.4 baseline |
| I8 | HBUT real data + TIME_ONLY | Success rate ≥ FULL |
| I9 | Cold-start migration | schema_migrations has v0.5.5 row; legacy snapshots retain v0.5.4 |
| I10 | Fresh DB cold start | Zero-config startup matches migrated behavior |

### 3.11 Performance Acceptance

| # | Metric | Baseline | v0.5.5 target |
|---|---|---|---|
| P1 | FULL 300 tasks elapsed | ~60s | ≤ 60s |
| P2 | TIME_ONLY 300 tasks elapsed | N/A | ≤ 60% of FULL |
| P3 | Peak memory | current | ≤ +15% |
| P4 | SA neighbor ops | current | ≤ +10% |

### 3.12 Pipeline Invariants (P1–P12)

| # | Invariant | Check |
|---|---|---|
| INV-P1 | Scheduler `Solve/Assign` signatures do not contain `SchedulingMode` or `SchedulingConfig` | grep signatures |
| INV-P2 | Scheduler impl does not import `backend/database`, `backend/models`, `backend/services/snapshot_service`, `backend/services/version_service`, or `gorm.io/gorm` | `go list -deps` static check |
| INV-P3 | Orchestrator constructor takes no DB / Snapshot / Version service | grep constructor |
| INV-P4 | Mode is consumed only in `SchedulingService.RunScheduling` and `Orchestrator.Run` | grep `.Mode` |
| INV-P5 | `TimeAssignmentDraft` and `RoomAllocationDraft` carry no ID / version_id / semester_id | struct audit |
| INV-P6 | Snapshot/Version write paths accept only `*ScheduleSnapshotDTO`; legacy signatures removed | compile-time enforcement |
| INV-P7 | Orchestrator.Run is idempotent (same input + seed → same output business fields) | unit test O10 |
| INV-P8 | TimeScheduler output session count per Task ≤ `resolveSessionPlan(...).SessionsPerWeek()` | unit test |
| INV-P9 | Scheduler components have zero DB / snapshot / version / filesystem side-effects; Solve/Assign are pure (same input + seed → same output; communication only via return value and ProgressReporter) | unit test with IO-tracking test double |
| INV-P10 | `TimeAssignmentDraft`, `RoomAllocationDraft`, `TimeAssignmentPlaced` cannot be persisted. Their package has no GORM tags and no `db.Create/Save/Update` reference. Conversion to `models.TimeAssignment`/`models.ScheduleEntry` occurs only in `SchedulingService` transaction | static check + integration test |
| INV-P11 | **Stage Boundary Integrity**: `TimeSchedulingOutput` contains no classroom-referencing field; `RoomSchedulingInput` contains no mode-referencing field. The pipeline's inter-stage DTO surfaces are the sole coupling points; adding a field that reintroduces cross-stage awareness (e.g., a `classroomHint` on `TimeAssignmentDraft`, or a `mode` on `RoomSchedulingInput`) violates this invariant | struct audit + dedicated tests validating each DTO's field set |
| INV-P12 | **Final State Scoring**: The `ScoreBreakdown` returned in `OrchestratorResult` reflects the **final state** at the last completed retry attempt only. Intermediate attempt scores are emitted via `ProgressReporter` but are not returned. This prevents ambiguity about "which attempt's score is authoritative" and aligns with Snapshot semantics (a snapshot captures one state) | unit test: multi-attempt run yields exactly one `ScoreBreakdown`, matching the state persisted in TA + Entry |

---

## 4. Solver Internal Design

### 4.1 Directory Layout

```
backend/
├── scheduling/
│   ├── types/            (DTOs + interfaces; no dependencies)
│   ├── matcher/          (migrated from services)
│   ├── time/
│   │   ├── scheduler.go
│   │   ├── driver.go     (SA/OR-Tools orchestration)
│   │   ├── sa/           (SA subpackage)
│   │   └── ortools/      (OR-Tools client, Time variant)
│   ├── room/
│   ├── score/
│   └── orchestrator/
│
├── services/             (existing, refactored to Facade)
│   ├── scheduling_service.go   [refactored]
│   ├── snapshot_service.go     [modified — DTO entrypoint]
│   ├── version_service.go      [modified — DTO entrypoint]
│   └── ...
│
└── dto/
    └── schedule_snapshot.go    (ScheduleSnapshotDTO)
```

**Import rules**:

```
scheduling/types           → stdlib only
scheduling/matcher         → scheduling/types
scheduling/score           → scheduling/types
scheduling/time            → scheduling/types + scheduling/matcher (+ net/http for OR-Tools)
scheduling/room            → scheduling/types + scheduling/matcher
scheduling/orchestrator    → scheduling/types (via interfaces)
services/scheduling_*      → scheduling/* + models + database
services/snapshot_service  → dto + models + database
services/version_service   → dto + models + database
```

CI enforcement: `scripts/check_scheduling_isolation.sh` (see 4.10.1).

### 4.2 TimeScheduler Internal Structure

```go
type TimeScheduler struct {
    driver *Driver
}

type Driver struct {
    saConfig       SAConfig
    ortoolsClient  ORToolsClient  // nil disables OR-Tools
    ortoolsTimeout time.Duration
}

func (d *Driver) Solve(ctx, input, progress) (Output, error):
    if d.ortoolsClient != nil && d.ortoolsClient.IsAvailable():
        progress.Stage("OR-Tools time solve", 40)
        out, err := d.solveWithORTools(ctx, input, progress)
        if err == nil && isAcceptable(out): return out, nil
        progress.Log(fmt.Sprintf("OR-Tools failed or degraded: %v", err))

    progress.Stage("SA time solve", 50)
    return d.solveWithSA(ctx, input, progress)
```

Driver is a **private** type inside `scheduling/time`. External consumers see
only `ITimeScheduler.Solve`. The Orchestrator is unaware of the two-path
internal.

### 4.3 SA Neighbor Redesign

Removed: `MoveRoom`, `Swap` (with room), any classroom-referencing operator.

New operator set:

| Operator | Semantics | Weight |
|---|---|---|
| MoveTime | Change draft (DayOfWeek, StartPeriod); Span unchanged | 60% |
| ChangeSpan | Change draft Span (legality via `IsSpanLegal`) | 10% |
| SwapTime | Swap two drafts' (day, period, span) | 20% |
| ShuffleTask | Randomly redistribute all drafts of a TaskID | 10% |

Hard constraints checked per Try:
1. `IsSpanLegal(start, span)`
2. LockedSlots non-overlap
3. teacherOcc / classOcc non-conflict
4. teacherUnavailable
5. `avoidanceHints` — **soft penalty only** (not hard prune)

`avoidanceHints` penalty formula (added to TimeScore bucket):
```
for each draft d:
    for each hint h where h.TeachingTaskID == d.TaskID:
        if d.Day == h.Day and periodsOverlap(d.Start, d.Span, h.Start, h.Span):
            penalty += 5.0  // hintWeight, code constant
timeScore -= penalty
```

### 4.4 Score Cache Three-Bucket Split

The SA score cache maintains 3 buckets (Time/Teacher/Student). Resource is
not computed inside SA (INV-Q1). Each bucket has its own incremental update
logic; `ShuffleTask`, being multi-draft, marks the affected bucket dirty and
triggers a full recompute of that bucket only.

### 4.5 OR-Tools solver.py Changes

**Removed from solver.py**:
1. `room` decision variable
2. `room_capacity_constraint`
3. `room_conflict_no_overlap`
4. `room_type_match_constraint`
5. `low_floor_preference_objective`
6. `classrooms` input parsing

**Kept**:
1. `session_start`, `session_day` decision variables
2. teacher_conflict, class_group_conflict, locked_slots_forbidden,
   teacher_unavailable_forbidden
3. `IsSpanLegal` block-alignment rule
4. All time soft-constraint objectives

**Added**:
1. `avoidance_hints_soft_penalty`

**Dual-path strategy** (until PR-10):
```python
def solve(input_json):
    if input_json.get("version") == "v0.5.5":
        return solve_time_only(input_json)
    return solve_full(input_json)  # legacy path
```

Legacy path retained through PR-10; PR-11 removes `solve_full`.

### 4.6 RoomScheduler Algorithm

Single-pass greedy. Ordering: hardest-to-place first
(`|AllowedRoomIDs|` asc, `TotalStudents` desc, `RequiredRoomType` specificity).

```
sorted := sortByDifficulty(input.Assignments)
occ := NewOccupancy()
allocations, hints := [], []

for placed in sorted:
    room, reason := findBestRoom(placed, classrooms, occ, tasks)
    if room:
        allocations.append(...)
        if !room.IsShared: occ.Mark(...)
    else:
        hints.append(ResourceConflictHint{Reason: reason, ...})
```

`findBestRoom` selection order:
1. Hard: matcher.Match OK, capacity, non-conflict
2. Soft scoring:
   - Tightest capacity fit (waste distance)
   - Low-floor preference (per teacher)
   - Exact type match bonus

**No backtracking, no multi-round**. Feedback to Time stage is via
Orchestrator retry (K=3).

### 4.7 ResourceMatcher Migration

From `backend/services/resource_matcher.go` to `backend/scheduling/matcher/`.
Signatures switch from `models.TeachingTask` / `models.Classroom` to
`types.TaskView` / `types.ClassroomView`. Logic unchanged; existing tests
migrated with structure adjustments.

### 4.8 Scorer Implementation

```
type Scorer struct { weights map[string]int }

func (s *Scorer) Score(assignments, allocations, teachers, classrooms, tasks, dims) ScoreBreakdown:
    result := ScoreBreakdown{}
    perBucketMax := 100.0 / float64(len(dims))
    result.EnabledDimensions = dims

    for _, dim := range dims:
        switch dim:
        case "time":     result.Time     = computeTimeBucket(...)
        case "teacher":  result.Teacher  = computeTeacherBucket(...)
        case "student":  result.Student  = computeStudentBucket(...)
        case "resource": result.Resource = computeResourceBucket(...)

    result.Total = sumNonNil(...)
    // completeness scaling → FinalTotal
    return result
```

The existing 7 `scoreXxx` functions in `scoring_service.go` migrate 1-to-1
into the 4 buckets with View-based signatures.

### 4.9 ProgressReporter Service-Layer Impl

```go
type schedulingReporter struct {
    result *SchedulingResult
    mu     sync.Mutex
}

func (r *schedulingReporter) Stage(name string, percent int) {...}
func (r *schedulingReporter) Iteration(cur, total int, cs, bs, temp float64) {
    if cur % 100 != 0 { return }  // sample to avoid log flood
    // ...
}
func (r *schedulingReporter) Log(message string) {...}
```

v0.5.5 keeps the "return logs at end" model — no realtime Wails event emit.
If needed later, decorate the reporter with a `WailsEventReporter`.

### 4.10 Solver Invariants (Q1–Q5)

| # | Invariant | Check |
|---|---|---|
| INV-Q1 | SA internal structures (`SAContext`, `ScoreCache`, `Occupancy`) contain no `ClassroomID` field or room-related index | struct audit |
| INV-Q2 | OR-Tools input JSON has no `classrooms` field; output has no `classroom_id` | contract test |
| INV-Q3 | `RoomScheduler.Assign` is single-pass greedy; no internal backtracking or multi-round; does not modify TimeAssignment | code review + INV-P9 assertion |
| INV-Q4 | ProgressReporter calls in Solver layer perform no IO themselves; only the Service-layer impl performs IO | unit test with IO-tracking test double |
| INV-Q5 | DTO has no reverse-mapping API to models (per INV-SN2) | grep for `DTOToModel*` |

#### 4.10.1 CI Static Check

```bash
#!/bin/bash
# scripts/check_scheduling_isolation.sh
DEPS=$(go list -deps ./backend/scheduling/... | \
       grep -E "(scheduling-system/backend/database|scheduling-system/backend/models|scheduling-system/backend/services/(snapshot|version)|gorm\.io/gorm)")
if [ -n "$DEPS" ]; then
    echo "VIOLATION: scheduling/* depends on forbidden packages:"
    echo "$DEPS"
    exit 1
fi
echo "OK: scheduling isolation verified"
```

Integrated into `Taskfile.yml` test target.

### 4.11 Solver Test Matrix

Unit:
- T1: SA MoveTime yields legal (day, period)
- T2: SA ChangeSpan honors `IsSpanLegal`
- T3: SA hint penalty accumulates correctly
- T4: Score cache 3-bucket increment matches full-recompute
- T5: RoomScheduler failure produces Hint with correct Reason
- T6: RoomScheduler `scoreCandidate` selects tightest fit
- T7: RoomScheduler ordering places hardest first
- T8: Matcher migration behavior identical to v0.5.4
- T9: Scorer bucket nil-ness matches `dims`
- T10: Scorer TIME_ONLY yields `Resource == nil`

Integration:
- C1: OR-Tools available → tried first, SA fallback on failure
- C2: OR-Tools disabled → SA only
- C3: Full pipeline FULL mode via Fake DBs
- C4: Full pipeline TIME_ONLY mode (RoomScheduler unused)

Idempotence (from Section 3):
- O10: Same input + seed → identical business fields

---

## 5. Migration & Implementation Plan

### 5.1 Strategy: Strangler Fig

Rejected alternatives:
- **Big Bang**: 3000–4000 lines in one PR is un-reviewable and un-rollbackable.
- **Bottom-Up**: Model-first changes break `main` until downstream PRs catch up.

**Chosen: Strangler Fig**. New code lives alongside old, unused, until a
switch PR flips the entry point atomically. Old code is only deleted after
stability observation.

Three disciplines:
1. Old code is not modified until the switch.
2. New code is not wired to the production path until the switch.
3. The switch is an atomic, reversible PR.

### 5.2 PR Sequence

**12 PRs across 4 phases.**

#### Phase 1 — Foundation (no runtime change)

| PR | Title | Lines | Impact |
|---|---|---|---|
| PR-01 | `scheduling/types` package (DTOs + interfaces, no impl) | ~500 | dead code |
| PR-02 | `dto.ScheduleSnapshotDTO` + Snapshot/Version DTO entrypoints | ~300 | new entry unused |
| PR-03 | `schema_migrations` meta table + `SchedulingMode` + `MaxRetries` fields (no behavior change) | ~200 | schema prep, Wails regen |

#### Phase 2 — Parallel Component Build (unwired)

| PR | Title | Lines | Impact |
|---|---|---|---|
| PR-04 | Migrate `ResourceMatcher` to `scheduling/matcher`, keep old file as shim | ~400 | dead code |
| PR-05 | New `scheduling/score` (Scorer + 4 buckets); keep old ScoringService | ~800 | dead code, equivalence-tested |
| PR-06 | New `scheduling/room` (RoomScheduler + Greedy) | ~500 | dead code |
| PR-07 | New `scheduling/time` (Driver + SA variant + OR-Tools client) | ~1000 | dead code |
| PR-08 | New `scheduling/orchestrator` | ~400 | dead code, DB-free tested |

#### Phase 3 — Switch (single-shot per side)

| PR | Title | Lines | Impact |
|---|---|---|---|
| PR-09 | Data model switch: create `time_assignments`, restructure `schedule_entries`, Migration logic, adapter shim in Service | ~600 | DB structure change, Wails regen |
| PR-10 | Entry switch: SchedulingService calls Orchestrator; UI adds Mode radio | ~800 | production path switch, Wails regen |

#### Phase 4 — Cleanup

| PR | Title | Lines | Impact |
|---|---|---|---|
| PR-11 | Delete legacy: sa_*.go, ortools_client.go, scoring_service.go, resource_matcher.go, solver_orchestrator*.go, solver.py's `solve_full` | ~-3000 net | cleanup |
| PR-12 | Frontend depth adaptation (schedule column, score-card Disabled, exports, snapshots) | ~600 | UX polish |

### 5.3 PR-09 Adapter Shim (Critical Detail)

Between PR-09 and PR-10, `SchedulingService.RunScheduling` still uses the
legacy SA solver but persists into the v0.5.5 schema via an adapter:

```go
// Legacy SA solver returns LegacyScheduleEntry (v0.5.4 rich shape)
// Adapter converts to (TA + Entry) pairs at persistence time.

func (s *SchedulingService) persistLegacyEntries(
    legacyEntries []LegacyScheduleEntry,
    version *models.ScheduleVersion,
) error {
    return s.db.Transaction(func(tx database.DB) error {
        for _, e := range legacyEntries {
            ta := models.TimeAssignment{
                SemesterID:        version.SemesterID,
                ScheduleVersionID: version.ID,
                TeachingTaskID:    *e.TeachingTaskID,
                DayOfWeek:         e.DayOfWeek,
                StartPeriod:       e.StartPeriod,
                Span:              e.Span,
            }
            if err := tx.Create(&ta).Error(); err != nil { return err }

            entry := models.ScheduleEntry{
                SemesterID:        version.SemesterID,
                ScheduleVersionID: version.ID,
                TimeAssignmentID:  ta.ID,
                ClassroomID:       e.ClassroomID,
            }
            if err := tx.Create(&entry).Error(); err != nil { return err }
        }
        return nil
    })
}
```

This shim disappears in PR-10 when the entry point flips to Orchestrator.

### 5.4 Continuous Compilability Invariants

Every PR merge must satisfy:

| # | Invariant | Check |
|---|---|---|
| CI-1 | `go build ./...` passes | CI |
| CI-2 | `go test ./...` passes | CI |
| CI-3 | Cold-start migration succeeds | integration test |
| CI-4 | Scheduling flow runs end-to-end (at least FULL mode) | integration test |
| CI-5 | Wails app launches; schedule view opens | manual QA |
| CI-6 | `scheduling/*` has no forbidden imports (from PR-04 onward) | static check |

### 5.5 Data Migration Timeline

```
PR-03:  CREATE schema_migrations; INSERT 'v0.5.5-prep'; ADD mode column
PR-09:  DROP old schedule_entries; CREATE time_assignments + new schedule_entries;
        ADD schema_version to snapshots + versions; INSERT 'v0.5.5'
```

No third data migration point. Old snapshot / version JSON is untouched and
marked legacy (INV-MI2).

### 5.6 Frontend Binding Regeneration Points

Only three: **PR-03, PR-09, PR-10**. All other PRs are forbidden from
touching `frontend/bindings/**` (INV-F1).

### 5.7 Rollback Plan

| PR | Risk | Rollback |
|---|---|---|
| PR-01 – PR-08 | 🟢 | `git revert`; zero data risk |
| **PR-09** | 🔴 | `git revert` + DB rollback script + snapshot JSON export as escape index |
| **PR-10** | 🟡 | `git revert`; legacy shim from PR-09 keeps writing to new schema |
| PR-11 – PR-12 | 🟡/🟢 | `git revert`; PR-11 only meaningful if PR-10 also reverted |

**PR-09 pre-deploy checklist**:
```bash
sqlite3 scheduler.db ".backup pre_v0.5.5_backup.db"
sqlite3 scheduler.db "SELECT id, semester_id, entries_json FROM schedule_snapshots" > snapshots_backup.jsonl
echo "PR-09 deploy time: $(date)" >> deploy.log
```

**PR-09 rollback script** (`roll_back_v0.5.5.sql`) is prepared beforehand:

```sql
START TRANSACTION;
    CREATE TABLE _bak_time_assignments AS SELECT * FROM time_assignments;
    CREATE TABLE _bak_schedule_entries AS SELECT * FROM schedule_entries;
    DROP TABLE schedule_entries;
    DROP TABLE time_assignments;
    -- restore v0.5.4 structure from pre_v0.5.5_backup.db
    DELETE FROM schema_migrations WHERE version = 'v0.5.5';
COMMIT;
```

### 5.8 PR-11 Cleanup Gate

Per user directive (Q18), cleanup is deferred to observe stability after
PR-10:

- ≥ 20 real scheduling runs completed
- No P0/P1 bugs open
- Migration / snapshot / restore stable
- Regression suite passes

Only then does PR-11 delete legacy code. Solver.py's `solve_full` is deleted
here (Q17).

### 5.9 Time & Parallelism

Single-developer estimate:

| Phase | PR | Days |
|---|---|---|
| 1 | PR-01 – PR-03 | 3 |
| 2 | PR-04 | 1 |
| 2 | PR-05 | 2 |
| 2 | PR-06 | 2 |
| 2 | PR-07 | 5 |
| 2 | PR-08 | 1 |
| 3 | PR-09 | 3 |
| 3 | PR-10 | 2 |
| 4 | PR-11 | 1 |
| 4 | PR-12 | 2 |
| **Total** | | **22 workdays** |

Critical path (with parallelism among Phase 2 PRs): **~16 workdays / 3–4 weeks**.

### 5.10 Risk Summary

| # | Risk | Prob | Impact | Mitigation |
|---|---|---|---|---|
| R1 | PR-07 SA rewrite introduces regression | high | med | unit tests + HBUT regression + intermediate commits |
| R2 | PR-09 migration transaction fails | low | high | atomic TX + backup + rollback script |
| R3 | PR-10 entry switch causes perf regression | med | med | P1-P4 acceptance + PR-09 shim as quick rollback |
| R4 | Wails binding regen breaks components | med | med | shim in PR-09 keeps enriched entry shape; UI changes deferred to PR-10/12 |
| R5 | solver.py dual-path introduces Python bug | low | med | Python unit tests + integration tests |
| R6 | Score bucket refactor changes scoring | med | low | PR-05 equivalence test |
| R7 | Orchestrator retry loop non-convergent | low | med | O3-O4 tests + K=3 hard cap |

---

## 6. Frontend Migration Boundary

### 6.1 Impacted Surface

Wails bindings (auto-generated), frontend components (hand-maintained), and
data-loading composables must be assessed each PR against a strict change
matrix (see 6.3).

### 6.2 API Data Shape Changes

#### SchedulingConfig (frontend → backend) — PR-03

```typescript
interface SchedulingConfig {
    // ... existing fields
    mode?: "FULL_SCHEDULING" | "TIME_ONLY_SCHEDULING";  // v0.5.5
    maxRetries?: number;                                 // v0.5.5
}
```

Old clients omitting `mode` are backend-defaulted to FULL.

#### ScheduleEntry (backend → frontend) — PR-09

Old (v0.5.4): rich structure with day/period/teacher/course/classroom fields.

New (v0.5.5 database):
```typescript
interface ScheduleEntry {
    id: number;
    semesterId: number;
    scheduleVersionId: number;
    timeAssignmentId: number;
    classroomId: number;
    classroom?: Classroom;
    timeAssignment?: TimeAssignment;
}

interface TimeAssignment {
    id: number;
    semesterId: number;
    scheduleVersionId: number;
    teachingTaskId: number;
    dayOfWeek: number;
    startPeriod: number;
    span: number;
    teachingTask?: TeachingTask;
}
```

UI components continue consuming the enriched shape via
`EnrichedScheduleEntry` (see 6.5.2).

#### ScheduleVersion — PR-03 & PR-09

```typescript
interface ScheduleVersion {
    // existing...
    mode: "FULL_SCHEDULING" | "TIME_ONLY_SCHEDULING";  // PR-03
    schemaVersion: "v0.5.4" | "v0.5.5";                // PR-09
}
```

#### Snapshot — PR-09

`Snapshot.entriesJson` deserializes as `ScheduleSnapshotDTO` (v0.5.5) or is
treated as legacy readonly (v0.5.4).

```typescript
function parseSnapshot(snapshot: Snapshot): {
    entries: EnrichedScheduleEntry[];
    mode: SchedulingMode;
    score?: ScoreBreakdown;
    isLegacy: boolean;
} {
    if (snapshot.schemaVersion === "v0.5.4") {
        return { entries: [], mode: "FULL_SCHEDULING",
                 score: undefined, isLegacy: true };
    }
    const dto = JSON.parse(snapshot.entriesJson) as ScheduleSnapshotDTO;
    return { entries: dto.assignments, mode: dto.mode,
             score: dto.score, isLegacy: false };
}
```

#### ScoreBreakdown — PR-05 / PR-09

Old: flat 7-field structure. New: 4-bucket structure with pointer-based
Disabled semantics (`resource === undefined` in TIME_ONLY).

### 6.3 Page Impact Matrix

Legend: ✅ = changes; ⚠️ = data-loading shim only; ➖ = untouched.

| Page/Component | PR-03 | PR-09 | PR-10 | PR-12 |
|---|---|---|---|---|
| Scheduling parameters panel | ➖ | ➖ | ✅ Mode radio | ⚠️ MaxRetries advanced |
| Schedule table | ➖ | ⚠️ JOIN via shim | ✅ Conditional classroom column | ✅ empty-state, TIME_ONLY label |
| Week view | ➖ | ⚠️ | ✅ card room field | ➖ |
| Score card | ➖ | ⚠️ shape change | ✅ Four-bucket refactor + Disabled | ➖ |
| Excel export | ➖ | ➖ | ✅ column headers per mode | ✅ classroom-column layout |
| PDF / print | ➖ | ➖ | ✅ column visibility | ⚠️ layout tweaks |
| Snapshot list | ➖ | ✅ Legacy badge + disable restore | ➖ | ⚠️ helper copy |
| Snapshot detail | ➖ | ✅ parseSnapshot | ⚠️ mode display | ➖ |
| Version list | ➖ | ✅ schema_version badge | ⚠️ Mode label | ➖ |
| Version compare | ➖ | ✅ parseSnapshot | ✅ dual mode display | ⚠️ columns |
| Teacher workload | ➖ | ⚠️ | ➖ | ➖ |
| Room utilization | ➖ | ⚠️ | ✅ empty-state for TIME_ONLY | ➖ |
| Move-course feature | ➖ | ⚠️ | ➖ | ➖ |

### 6.4 TIME_ONLY UI Behavior

#### 6.4.1 Scheduling Parameters Panel
Mode radio shows current selection. Selecting TIME_ONLY does not hide
classroom-related constraints (`low_floor_preference` etc.), but greys them
with tooltip "TIME_ONLY mode does not evaluate this constraint". Constraint
preferences are preserved across mode switches.

#### 6.4.2 Schedule Table

FULL columns: `| Teacher | Class | Time | Classroom | Weeks |`
TIME_ONLY columns: `| Teacher | Class | Time | Weeks |`

Classroom column is **removed** (not displayed as `—`). Page-top pill
displays "This schedule has no classroom allocation". Edit-classroom actions
are disabled.

#### 6.4.3 Score Card
FULL: 4 buckets shown. TIME_ONLY: 4 buckets shown, but Resource bucket:
- Value area shows `—` (em dash)
- Card background greyed
- Tooltip: "TIME_ONLY mode does not evaluate resources"
- **Never shows `0.0`** (avoids "resource scored zero" confusion).

#### 6.4.4 Excel Export

FULL: `Teacher | Course | Class | Day | Periods | Classroom | Weeks`
TIME_ONLY: `Teacher | Course | Class | Day | Periods | Weeks`

Filename suffix: FULL → `课表_{semester}_{ts}.xlsx`;
TIME_ONLY → `时间表_{semester}_{ts}.xlsx`.

#### 6.4.5 Room Utilization Page (TIME_ONLY)
Top banner: "This version is TIME_ONLY; no classrooms allocated; utilization
cannot be computed." Chart hidden. Provide switch link to a FULL version if
one exists for the semester.

#### 6.4.6 Version Compare
Cross-mode compare is allowed. Difference panel top warning:
"Compared versions use different modes (TIME_ONLY vs FULL); classroom fields
appear only in FULL." Classroom difference rows greyed.

### 6.5 Wails Binding Change Points

#### 6.5.1 PR-03 Regeneration
Triggers: `SchedulingMode`, `SchedulingConfig.Mode/MaxRetries`,
`ScheduleVersion.Mode`. Frontend does not consume new fields yet;
`npm run build` passes; app launches as before.

#### 6.5.2 PR-09 Regeneration (largest surface)

Triggers: `ScheduleEntry` slimming, `TimeAssignment` new, DTO package new,
Snapshot/Version DTO entrypoints.

**Frontend strategy — enriched-entry shim**:

Backend adds `ScheduleQueryService.GetEnrichedScheduleEntries(semesterID, versionID)`
returning:

```typescript
interface EnrichedScheduleEntry {
    id: number;
    dayOfWeek: number;
    startPeriod: number;
    span: number;
    weeks: string;
    teacherId: number;
    teacherName: string;
    courseId: number;
    courseName: string;
    classGroupIds: number[];
    classGroupNames: string[];
    classroomId?: number;      // absent in TIME_ONLY
    classroomName?: string;
    classroomFloor?: number;
    classroomType?: string;
    scheduleVersionId: number;
}
```

The composable:
```typescript
// composables/useEnrichedEntries.ts
export async function loadEnrichedEntries(
    semesterId: number, versionId?: number,
): Promise<EnrichedScheduleEntry[]> {
    return ScheduleQueryService.GetEnrichedScheduleEntries(semesterId, versionId ?? 0);
}
```

UI components are untouched in PR-09 — they consume `EnrichedScheduleEntry`
exactly as they consumed the old `ScheduleEntry` shape.

#### 6.5.3 PR-10 Regeneration
No new binding surface expected. Regeneration is performed to keep binding
files in lockstep with the backend binary; if diff is empty, the regen step
may be skipped.

### 6.6 Frontend Invariants

| # | Invariant | Check |
|---|---|---|
| INV-F1 | Wails bindings regenerate only in PR-03 / PR-09 / PR-10 | Git history audit: `bindings/**` commits only in these three PRs |
| INV-F2 | UI components do not directly consume `models.ScheduleEntry` or `models.TimeAssignment`; only `EnrichedScheduleEntry` | Static lint: grep `.timeAssignment.` / `.classroom.` warnings |
| INV-F3 | TIME_ONLY mode: classroom column absent (not shown as `—`); Resource bucket shown as `—` (not `0`). FULL mode with partial failure: classroom column present, per-row `—` allowed. | UI snapshot test |
| INV-F4 | Frontend `mode` conditions occur only in presentation layer (templates / conditional render). Data-handling layer (composables, stores) is mode-agnostic; shape is normalized by backend enrichment. | Code review: no `if (mode === ...)` in composables/stores that mutates data |
| INV-F5 | **Frontend DTO Ownership**: `dto.ScheduleSnapshotDTO` is owned by the backend; frontend consumes it as a read-only value. Frontend must never construct or serialize DTO instances back to the backend. Any snapshot restore flows use ID references, not DTO round-tripping. | Code review: no `JSON.stringify(dto)` sent to backend endpoints |
| INV-F6 | **Snapshot Display Purity**: Rendering a snapshot from `Snapshot.entriesJson` must not trigger additional DB fetches for teacher/course/classroom names. All display fields are already embedded in `ScheduledAssignmentDTO`. This preserves snapshot semantics: a snapshot rendered later reflects the state at snapshot time, not current data. | Integration test: rendering a snapshot after mutating a teacher's name yields the snapshotted (old) name |

### 6.7 Frontend Per-PR Acceptance

- **PR-03**: `npm run build` passes; app launches; scheduling flow unchanged.
- **PR-09**: `npm run build` passes; **all pages visually identical to PR-03**
  (via enriched-entry shim); legacy snapshot rows show badge + disabled restore.
- **PR-10**: Mode radio functional; column visibility per mode; score card
  Disabled semantics; export column headers correct; utilization page
  empty-state for TIME_ONLY.
- **PR-12**: All 6.4 UI-behavior rules met; empty-state copy, tooltips,
  badges complete.

### 6.8 Component-Immutability Guard for PR-09

To enforce PR-09's "shim only, no UI changes":

- PR description mandates: no `frontend/src/components/**` diff except:
  - Data-loading entrypoint swap (old API → `GetEnrichedScheduleEntries`)
  - Introduction of `parseSnapshot` helper
- PR-09 frontend diff ≤ ±500 lines.
- Screenshot diff (if configured) shows pixel-identical pages under identical
  input.

### 6.9 Frontend Rollback

Aligned with backend per-PR risk (see 5.7). PR-03/12 are 🟢; PR-09 is 🟡
(coupled to backend PR-09); PR-10 is 🟡 (coupled to backend PR-10).

### 6.10 Boundary Test Scenario Clarifications

Two distinct empty-state semantics:

1. **TIME_ONLY**: all TAs have no ScheduleEntry → frontend **hides** classroom
   column entirely.
2. **FULL with partial failure**: some TAs have no ScheduleEntry → frontend
   **keeps** classroom column, individual rows display `—`.

Rule: `mode === TIME_ONLY_SCHEDULING` hides the column; otherwise, the column
is shown with per-row optional value.

---

## 7. Consolidated Invariants

### 7.1 Architectural (Section 1)

| # | Statement |
|---|---|
| I1 | `SchedulingMode` is the sole source of mode state; no secondary field |
| I2 | TimeScheduler references no `Classroom`/`ClassroomID`/room type/capacity |
| I3 | RoomScheduler references no `SchedulingMode` |
| I4 | Stage 2 communicates with Stage 1 only via return values (Hints); no cross-references |
| I5 | `ResourceConflictHint` ≠ `LockedTimeSlot` at the type level |
| I6 | `TimeAssignment` is mode-agnostic |
| I7 | `ScheduleEntry` carries no redundant time fields |
| I8 | Score buckets are gated by `EnabledDimensions`; Disabled is `nil`, not 0 |
| I9 | Retry cap `K` is configurable; exhausting yields partial + diagnostics |
| I10 | Clean migration; legacy is read-only |

### 7.2 Domain Model (Section 2)

| # | Statement |
|---|---|
| INV-M1 | `SchedulingMode` constant set is closed |
| INV-T1 | One TA row = one weekly session |
| INV-T2 | TA structure is mode-agnostic |
| INV-T3 | TA existence ≠ completion |
| INV-T4 | Every TA belongs to a `ScheduleVersion`; NOT NULL |
| INV-E1 | TIME_ONLY yields zero current-version Entry rows |
| INV-E2 | `Entry.SemesterID == Entry.TA.SemesterID` |
| INV-E3 | Only `(TA_ID, Room_ID)` carries semantic value on Entry |
| INV-E4 | `Entry.ScheduleVersionID == Entry.TA.ScheduleVersionID` |
| INV-H1 | Hint has no GORM tags; not persisted |
| INV-H2 | Hint and LockedTimeSlot share no structure |
| INV-C1 | `SchedulingConfig` has no mode-related field other than `Mode` |
| INV-C2 | Mode/MaxRetries default only at `RunScheduling` entry |
| INV-S1 | Disabled expressed by nil pointer only |
| INV-S2 | EnabledDimensions ⇔ non-nil buckets, mutually exclusive & exhaustive |
| INV-S3 | Bucket → sub-constraint mapping is a code constant |
| INV-SN1 | Snapshot/Version JSON ≡ `ScheduleSnapshotDTO` |
| INV-SN2 | DTO → models has no reverse-mapping API |
| INV-MI1 | `schema_migrations` row `v0.5.5` marks migration complete |
| INV-MI2 | Legacy `entries_json` is never deserialized to models |
| INV-MI3 | Migration is single-transaction atomic |

### 7.3 Pipeline (Section 3)

| # | Statement |
|---|---|
| INV-P1 | Scheduler signatures carry no Mode/Config |
| INV-P2 | Scheduler impl does not import DB / models / snapshot / version / GORM |
| INV-P3 | Orchestrator constructor takes no DB/Snapshot/Version service |
| INV-P4 | Mode is consumed only in `SchedulingService.RunScheduling` and `Orchestrator.Run` |
| INV-P5 | Drafts carry no ID / version_id / semester_id |
| INV-P6 | Snapshot/Version write paths accept only `*ScheduleSnapshotDTO` |
| INV-P7 | Orchestrator.Run is idempotent |
| INV-P8 | TimeScheduler output session count per Task ≤ session plan |
| INV-P9 | Scheduler components have zero DB / snapshot / version / filesystem side-effects |
| INV-P10 | Draft objects cannot be persisted; conversion happens only in `SchedulingService` transaction |
| INV-P11 | **Stage Boundary Integrity**: `TimeSchedulingOutput` has no classroom-referencing field; `RoomSchedulingInput` has no mode-referencing field |
| INV-P12 | **Final State Scoring**: `OrchestratorResult.Score` reflects the final retry attempt only; intermediate scores are emitted via ProgressReporter, not returned |

### 7.4 Solver (Section 4)

| # | Statement |
|---|---|
| INV-Q1 | SA internal structures carry no ClassroomID / room index |
| INV-Q2 | OR-Tools I/O JSON has no classroom field |
| INV-Q3 | RoomScheduler is single-pass greedy; no backtracking / multi-round |
| INV-Q4 | ProgressReporter calls in Solver perform no IO |
| INV-Q5 | DTO has no reverse-mapping API to models |

### 7.5 Migration (Section 5)

| # | Statement |
|---|---|
| CI-1 | `go build ./...` passes at every PR merge |
| CI-2 | `go test ./...` passes at every PR merge |
| CI-3 | Cold-start migration succeeds |
| CI-4 | Scheduling flow runs end-to-end (at least FULL) |
| CI-5 | Wails app launches; schedule view opens |
| CI-6 | `scheduling/*` has no forbidden imports (PR-04+) |

### 7.6 Frontend (Section 6)

| # | Statement |
|---|---|
| INV-F1 | Wails bindings regenerate only in PR-03 / PR-09 / PR-10 |
| INV-F2 | UI components consume only `EnrichedScheduleEntry`, not raw models |
| INV-F3 | TIME_ONLY hides classroom column; Resource shown as `—`, not `0` |
| INV-F4 | Mode checks appear only in presentation layer |
| INV-F5 | **Frontend DTO Ownership**: DTO is backend-owned; frontend consumes it read-only; frontend never constructs/serializes DTOs back |
| INV-F6 | **Snapshot Display Purity**: Snapshot rendering uses only embedded DTO fields; no additional DB fetches for display names |

---

## Appendix A — Open Questions Resolved

| Q | Decision |
|---|---|
| Q1 | `ScheduleEntry` retains its name; doc comment declares Resource Allocation Entity |
| Q2 | Snapshot/Version JSON uses `ScheduleSnapshotDTO` |
| Q3 | No Mode validation layer inside Solver; single-point read in Service+Orchestrator |
| Q4 | `ScheduleVersion` adds `Mode` field |
| Q5 | Soft-delete strategy for legacy TA/Entry rows |
| Q6 | `TeachingTaskView` is shared (single view, projection isolation via INV-P1/P2) |
| Q7 | Scorer is interfaced (`IScorer`) |
| Q8 | `MaxRetries` in `SchedulingConfig`, default 3 |
| Q9 | ProgressReporter is an explicit interface parameter (no context.Value) |
| Q10 | SA/OR-Tools selection encapsulated in TimeScheduler's Driver |
| Q11 | RoomScheduler is single-pass greedy |
| Q12 | SA neighbor weights are code constants |
| Q13 | Hint penalty weight is a code constant |
| Q14 | solver.py does not preserve v0.5.4 request format long-term |
| Q15 | Score cache dirty-recompute fallback accepted |
| Q16 | 12 PRs, no consolidation; one PR per domain change |
| Q17 | solver.py retains `solve_full` + `solve_time_only` until PR-10; deleted in PR-11 |
| Q18 | Cleanup after observation gate (≥ 20 runs, no P0/P1, stable) |
| Q19 | Wails bindings regen only in PR-03 / PR-09 / PR-10 |
| Q20 | New `ScheduleQueryService` for enriched-entry JOIN endpoint |
| Q21 | v0.5.4 snapshots are disabled (badge + disabled restore); no preview |
| Q22 | TIME_ONLY → FULL upgrade is out of scope for v0.5.5 |

---

## Appendix B — Terminology

- **TimeAssignment (TA)**: Time-fact entity; a scheduled weekly session.
- **ScheduleEntry**: Resource allocation entity; binds a classroom to a TA.
- **Draft**: In-memory representation of a TA or Entry prior to persistence.
- **View**: A GORM-free value type consumed by Solver components.
- **DTO**: `ScheduleSnapshotDTO`; a persisted snapshot of a run's state.
- **Hint**: `ResourceConflictHint`; feedback from RoomScheduler to next retry.
- **Enriched Entry**: Backend-JOINed rich object consumed by frontend UI.

---

## Appendix C — Change Log

- 2026-07-13: Initial freeze. Sections 1–6 approved. Invariants I1–I10,
  INV-M1, INV-T1..T4, INV-E1..E4, INV-H1..H2, INV-C1..C2, INV-S1..S3,
  INV-SN1..SN2, INV-MI1..MI3, INV-P1..P12, INV-Q1..Q5, CI-1..CI-6,
  INV-F1..F6 frozen.
