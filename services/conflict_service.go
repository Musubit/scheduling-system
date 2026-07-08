package services

// ConflictService handles conflict detection and resolution.
type ConflictService struct{}

func NewConflictService() *ConflictService {
	return &ConflictService{}
}

// Conflict represents a scheduling conflict.
type Conflict struct {
	ID          uint   `json:"id"`
	Type        string `json:"type"`        // "teacher", "room_capacity", "room_double_booked"
	Description string `json:"description"`
	Severity    string `json:"severity"`    // "error", "warning"
	Details     map[string]interface{} `json:"details"`
}

// Resolution represents a proposed solution for a conflict.
type Resolution struct {
	ID          uint   `json:"id"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
}
