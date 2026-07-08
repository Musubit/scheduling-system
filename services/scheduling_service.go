package services

// SchedulingService handles the automated scheduling algorithm.
type SchedulingService struct{}

func NewSchedulingService() *SchedulingService {
	return &SchedulingService{}
}

// SchedulingConfig holds the parameters for a scheduling run.
type SchedulingConfig struct {
	Scope      string `json:"scope"`      // "all" or department name
	Semester   string `json:"semester"`
	Strategy   string `json:"strategy"`   // "teacher_first", "room_utilization", "student_balance"
	Iterations int    `json:"iterations"`
}

// SchedulingResult holds the output of a scheduling run.
type SchedulingResult struct {
	TotalCourses int     `json:"totalCourses"`
	Scheduled    int     `json:"scheduled"`
	Conflicts    int     `json:"conflicts"`
	Utilization  float64 `json:"utilization"`
	Logs         []string `json:"logs"`
}
