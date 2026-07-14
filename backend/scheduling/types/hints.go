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
