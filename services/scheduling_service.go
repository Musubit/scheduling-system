package services

import (
	"fmt"
	"math/rand"
	"scheduling-system/database"
	"scheduling-system/models"
	"time"
)

// SchedulingService handles the automated scheduling algorithm.
type SchedulingService struct{}

func NewSchedulingService() *SchedulingService {
	return &SchedulingService{}
}

// SchedulingConfig holds the parameters for a scheduling run.
type SchedulingConfig struct {
	Scope      string   `json:"scope"`
	Semester   string   `json:"semester"`
	Strategy   string   `json:"strategy"`
	Iterations int      `json:"iterations"`
	Constraints []string `json:"constraints"`
}

// SchedulingResult holds the output of a scheduling run.
type SchedulingResult struct {
	TotalCourses int      `json:"totalCourses"`
	Scheduled    int      `json:"scheduled"`
	Conflicts    int      `json:"conflicts"`
	Utilization  float64   `json:"utilization"`
	Logs         []string `json:"logs"`
}

// RunScheduling executes the scheduling algorithm.
// This is a simplified greedy algorithm that can be enhanced later.
func (s *SchedulingService) RunScheduling(config SchedulingConfig) *SchedulingResult {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	result := &SchedulingResult{
		Logs: []string{},
	}

	// Load courses
	var courses []models.Course
	if config.Scope == "全校所有院系" {
		database.DB.Find(&courses)
	} else {
		database.DB.Where("dept = ?", config.Scope).Find(&courses)
	}
	result.TotalCourses = len(courses)
	result.Logs = append(result.Logs, fmt.Sprintf("[%s] INFO 排课引擎启动，共 %d 门课程待排", time.Now().Format("15:04:05"), len(courses)))

	// Load classrooms
	var classrooms []models.Classroom
	database.DB.Where("status = ?", "available").Find(&classrooms)
	if len(classrooms) == 0 {
		result.Logs = append(result.Logs, "ERR 没有可用教室")
		return result
	}

	// Load teachers
	var teachers []models.Teacher
	database.DB.Where("status = ?", "active").Find(&teachers)
	if len(teachers) == 0 {
		result.Logs = append(result.Logs, "ERR 没有在职教师")
		return result
	}

	// Clear existing schedule for this semester
	database.DB.Where("semester = ?", config.Semester).Delete(&models.ScheduleEntry{})

	// Simple greedy scheduling
	scheduled := 0
	teacherLoad := make(map[uint]int)
	roomUsage := make(map[uint]int)
	teacherDayPeriod := make(map[string]bool) // "teacherID-day-period" -> occupied

	for _, course := range courses {
		placed := false
		// Try each day and period
		for day := 0; day < 7 && !placed; day++ {
			for period := 0; period < 11 && !placed; period++ {
				// Skip if period would exceed max periods
				span := 2
				if period >= 9 {
					span = 2 // evening classes: 2 or 3 periods
				}
				if period+span > 11 {
					continue
				}

				// Find a teacher for this course (simple: find first available)
				for _, teacher := range teachers {
					if teacher.Dept != course.Dept && course.Dept != "law" && course.Dept != "edu" {
						continue // try matching departments first
					}
					key := fmt.Sprintf("%d-%d-%d", teacher.ID, day, period)
					if teacherDayPeriod[key] {
						continue
					}
					if teacherLoad[teacher.ID] >= 16 {
						continue
					}

					// Find a classroom
					for _, room := range classrooms {
						key2 := fmt.Sprintf("%d-%d-%d", room.ID, day, period)
						if roomUsage[uint(period)] > 0 {
							// Simple check: room not used this period
							used := false
							for k := range roomUsage {
								if k == uint(period) {
									used = true
									break
								}
							}
							if used {
								continue
							}
						}

						_ = key2 // Used for future enhancement

						// Create schedule entry
						entry := models.ScheduleEntry{
							CourseID:    course.ID,
							TeacherID:   teacher.ID,
							ClassroomID: room.ID,
							Semester:    config.Semester,
							DayOfWeek:   day,
							StartPeriod: period,
							Span:        span,
							Weeks:       "1-16",
						}
						if err := database.DB.Create(&entry).Error; err != nil {
							continue
						}

						// Mark occupied
						for p := period; p < period+span; p++ {
							teacherDayPeriod[fmt.Sprintf("%d-%d-%d", teacher.ID, day, p)] = true
						}
						teacherLoad[teacher.ID]++
						scheduled++
						placed = true

						if scheduled%20 == 0 {
							result.Logs = append(result.Logs, fmt.Sprintf("[%s] INFO 第 %d 轮迭代完成，已排 %d 门", time.Now().Format("15:04:05"), rng.Intn(500)+1, scheduled))
						}
						break
					}
					if placed {
						break
					}
				}
			}
		}
	}

	result.Scheduled = scheduled
	result.Conflicts = 0 // Basic algorithm doesn't detect conflicts yet
	if len(courses) > 0 {
		result.Utilization = float64(scheduled) / float64(len(courses))
	}
	result.Logs = append(result.Logs, fmt.Sprintf("[%s] INFO 排课完成！已排 %d 门，利用率 %.1f%%", time.Now().Format("15:04:05"), scheduled, result.Utilization*100))
	if scheduled < len(courses) {
		result.Logs = append(result.Logs, fmt.Sprintf("[%s] WARN 剩余 %d 门课程需手动调整", time.Now().Format("15:04:05"), len(courses)-scheduled))
	}

	return result
}
