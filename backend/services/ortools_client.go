package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ORToolsClient communicates with a local OR-Tools Python microservice.
type ORToolsClient struct {
	baseURL string
	client  *http.Client
}

// NewORToolsClient creates a client for the OR-Tools microservice.
func NewORToolsClient(port int) *ORToolsClient {
	return &ORToolsClient{
		baseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		client: &http.Client{
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: true, // prevent "unsolicited response" on idle channels
			},
		},
	}
}

// ORToolsInput is the JSON payload sent to the Python solver.
type ORToolsInput struct {
	TeachingTasks     []ORToolsTask      `json:"teachingTasks"`
	Teachers          []ORToolsTeacher   `json:"teachers"`
	Classrooms        []ORToolsRoom      `json:"classrooms"`
	ClassGroups       []ORToolsClassGroup `json:"classGroups"`
	LockedSlots       []LockedTimeSlot   `json:"lockedSlots"`
	Constraints       []string           `json:"constraints"`
	ConstraintWeights map[string]int     `json:"constraintWeights"`
	SportsCourseIDs   []uint             `json:"sportsCourseIDs"`
	TimeLimitSeconds  int                `json:"timeLimitSeconds"`
}

type ORToolsTask struct {
	ID                uint   `json:"id"`
	TeacherID         uint   `json:"teacherId"`
	CourseID          uint   `json:"courseId"`
	ClassIDs          []uint `json:"classIds"`
	SessionsPerWeek   int    `json:"sessionsPerWeek,omitempty"`
	// v0.5.1: per-session span shape derived by Go from course hours + preferred span.
	// When non-empty, Python must respect these values instead of computing its own.
	// Length == SessionsPerWeek (or, when SessionsPerWeek is 0, len(SessionSpans) drives it).
	SessionSpans      []int  `json:"sessionSpans,omitempty"`
	TotalHours        int    `json:"totalHours,omitempty"`
	MaxHoursPerWeek   int    `json:"maxHoursPerWeek,omitempty"`
	PreferredSpan     int    `json:"preferredSpan,omitempty"`
	RequiredRoomType  string `json:"requiredRoomType,omitempty"`
	StartWeek         int    `json:"startWeek"`
	EndWeek           int    `json:"endWeek"`
}

type ORToolsTeacher struct {
	ID               uint   `json:"id"`
	Name             string `json:"name"`
	PreferNoEarly    bool   `json:"preferNoEarly"`
	PreferNoLate     bool   `json:"preferNoLate"`
	MaxDaysPerWeek   int    `json:"maxDaysPerWeek"`
	PreferLowFloor   bool   `json:"preferLowFloor"`
	UnavailableSlots string `json:"unavailableSlots"`
}

type ORToolsRoom struct {
	ID       uint   `json:"id"`
	Floor    int    `json:"floor"`
	Capacity int    `json:"capacity"`
	Type     string `json:"type,omitempty"`
}

type ORToolsClassGroup struct {
	ID       uint `json:"id"`
	Students int  `json:"students"`
}

// ORToolsOutput is the response from the Python solver.
type ORToolsOutput struct {
	Entries             []ORToolsEntry `json:"entries"`
	Score               float64        `json:"score"`
	Status              string         `json:"status"`
	ElapsedMs           int            `json:"elapsedMs"`
	Error               string         `json:"error,omitempty"`
	SessionsPlaced      int            `json:"sessionsPlaced,omitempty"`
	SessionsExpected    int            `json:"sessionsExpected,omitempty"`
	Conflicts           []string       `json:"conflicts,omitempty"`
	UnplacedDiagnostics []string       `json:"unplaced,omitempty"`
}

type ORToolsEntry struct {
	TaskID       uint `json:"taskId"`
	TeacherID    uint `json:"teacherId"`
	ClassroomID  uint `json:"classroomId"`
	DayOfWeek    int  `json:"dayOfWeek"`
	StartPeriod  int  `json:"startPeriod"`
	Span         int  `json:"span"`
	SessionIndex int  `json:"sessionIndex,omitempty"`
}

// HealthCheck returns true if the OR-Tools service is reachable.
func (c *ORToolsClient) HealthCheck() bool {
	resp, err := c.client.Get(c.baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// Solve sends a scheduling problem to the OR-Tools service and returns the result.
func (c *ORToolsClient) Solve(input ORToolsInput) (*ORToolsOutput, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}

	resp, err := c.client.Post(c.baseURL+"/solve", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var output ORToolsOutput
	if err := json.Unmarshal(respBody, &output); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if output.Status == "error" {
		return &output, fmt.Errorf("solver error: %s", output.Error)
	}

	return &output, nil
}
