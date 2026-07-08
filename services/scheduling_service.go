package services

import (
	"fmt"
	"math/rand"
	"scheduling-system/database"
	"scheduling-system/models"
	"time"
)

type SchedulingService struct{}

func NewSchedulingService() *SchedulingService { return &SchedulingService{} }

type SchedulingConfig struct {
	Scope       string   `json:"scope"`
	Semester    string   `json:"semester"`
	Strategy    string   `json:"strategy"`
	Iterations  int      `json:"iterations"`
	Constraints []string `json:"constraints"`
}

type SchedulingResult struct {
	TotalCourses int      `json:"totalCourses"`
	Scheduled    int      `json:"scheduled"`
	Conflicts    int      `json:"conflicts"`
	Utilization  float64  `json:"utilization"`
	Logs         []string `json:"logs"`
}

func (s *SchedulingService) RunScheduling(config SchedulingConfig) *SchedulingResult {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	result := &SchedulingResult{Logs: []string{}}

	// Load data
	var courses []models.Course
	if config.Scope == "全校所有院系" {
		database.DB.Find(&courses)
	} else {
		database.DB.Where("dept = ?", config.Scope).Find(&courses)
	}
	result.TotalCourses = len(courses)
	log := func(msg string) { result.Logs = append(result.Logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)) }
	log(fmt.Sprintf("INFO 排课引擎启动，共 %d 门课程待排", len(courses)))

	var classrooms []models.Classroom
	database.DB.Where("status = ?", "available").Find(&classrooms)
	var teachers []models.Teacher
	database.DB.Where("status = ?", "active").Find(&teachers)
	if len(classrooms) == 0 || len(teachers) == 0 {
		log("ERR 缺少教室或教师资源")
		return result
	}

	// Clear existing
	database.DB.Where("semester = ?", config.Semester).Delete(&models.ScheduleEntry{})

	// Valid start periods for 连上 blocks
	validStarts := []int{0, 2, 4, 6, 8}

	// Occupancy tracking: key = "day-period-roomID" or "day-period-teacherID"
	roomOccupied := make(map[string]bool)
	teacherOccupied := make(map[string]bool)
	teacherLoad := make(map[uint]int)

	scheduled := 0
	for _, course := range courses {
		placed := false
		// Shuffle days for variety
		days := rng.Perm(7)
		for _, day := range days {
			if placed {
				break
			}
			// Shuffle valid starts
			starts := make([]int, len(validStarts))
			copy(starts, validStarts)
			rng.Shuffle(len(starts), func(i, j int) { starts[i], starts[j] = starts[j], starts[i] })

			for _, start := range starts {
				if placed {
					break
				}
				span := 2
				if start == 8 {
					span = 2 + rng.Intn(2) // 9-10 or 9-10-11
				}
				if start+span > 11 {
					continue
				}

				// Find matching teacher
				teacherCandidates := findTeachers(teachers, course.Dept)
				rng.Shuffle(len(teacherCandidates), func(i, j int) { teacherCandidates[i], teacherCandidates[j] = teacherCandidates[j], teacherCandidates[i] })

				for _, teacher := range teacherCandidates {
					if teacherLoad[teacher.ID] >= 16 {
						continue
					}
					// Check teacher availability for all periods in span
					teacherBusy := false
					for p := start; p < start+span; p++ {
						if teacherOccupied[fmt.Sprintf("%d-%d-%d", day, p, teacher.ID)] {
							teacherBusy = true
							break
						}
					}
					if teacherBusy {
						continue
					}

					// Find matching classroom
					for _, room := range classrooms {
						roomBusy := false
						for p := start; p < start+span; p++ {
							if roomOccupied[fmt.Sprintf("%d-%d-%d", day, p, room.ID)] {
								roomBusy = true
								break
							}
						}
						if roomBusy {
							continue
						}

						// Place the course!
						entry := models.ScheduleEntry{
							CourseID:    course.ID,
							TeacherID:   teacher.ID,
							ClassroomID: room.ID,
							Semester:    config.Semester,
							DayOfWeek:   day,
							StartPeriod: start,
							Span:        span,
							Weeks:       "1-16",
						}
						if err := database.DB.Create(&entry).Error; err != nil {
							continue
						}

						// Mark occupied
						for p := start; p < start+span; p++ {
							roomOccupied[fmt.Sprintf("%d-%d-%d", day, p, room.ID)] = true
							teacherOccupied[fmt.Sprintf("%d-%d-%d", day, p, teacher.ID)] = true
						}
						teacherLoad[teacher.ID]++
						scheduled++
						placed = true
						break
					}
					if placed {
						break
					}
				}
			}
		}
		if scheduled%20 == 0 && scheduled > 0 {
			log(fmt.Sprintf("INFO 已排 %d 门课程", scheduled))
		}
	}

	// Detect conflicts
	conflicts := (&ConflictService{}).DetectConflicts(config.Semester)

	result.Scheduled = scheduled
	result.Conflicts = len(conflicts)
	if result.TotalCourses > 0 {
		result.Utilization = float64(scheduled) / float64(result.TotalCourses)
	}

	log(fmt.Sprintf("INFO 排课完成！已排 %d/%d 门，利用率 %.1f%%，冲突 %d 个",
		scheduled, result.TotalCourses, result.Utilization*100, len(conflicts)))
	if scheduled < result.TotalCourses {
		log(fmt.Sprintf("WARN 剩余 %d 门课程需手动调整", result.TotalCourses-scheduled))
	}

	return result
}

func findTeachers(teachers []models.Teacher, dept string) []models.Teacher {
	// Prefer same department
	var same, other []models.Teacher
	for _, t := range teachers {
		if t.Dept == dept || (dept == "cs" && t.Dept == "计算机科学学院") ||
			(dept == "math" && t.Dept == "数学与统计学院") ||
			(dept == "phys" && t.Dept == "物理学院") ||
			(dept == "eng" && t.Dept == "外国语学院") ||
			(dept == "eco" && t.Dept == "经济管理学院") ||
			(dept == "law" && t.Dept == "法学院") ||
			(dept == "art" && t.Dept == "艺术学院") ||
			(dept == "edu" && t.Dept == "教育学院") {
			same = append(same, t)
		} else {
			other = append(other, t)
		}
	}
	return append(same, other...)
}
