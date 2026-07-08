package services

import (
	"fmt"
	"math/rand"
	"scheduling-system/database"
	"scheduling-system/models"
	"time"

	"gorm.io/gorm"
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
	Error        string   `json:"error,omitempty"`
}

func (s *SchedulingService) RunScheduling(config SchedulingConfig) *SchedulingResult {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	result := &SchedulingResult{Logs: []string{}}
	log := func(msg string) { result.Logs = append(result.Logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)) }

	// Load courses
	var courses []models.Course
	scopeCode := reverseDeptMap[config.Scope] // Convert Chinese→code for DB query
	scopeSQL := config.Scope
	if scopeCode != "" {
		scopeSQL = scopeCode
	}
	if config.Scope == "全校所有院系" {
		if err := database.DB.Find(&courses).Error; err != nil {
			result.Error = "加载课程失败: " + err.Error()
			return result
		}
	} else {
		if err := database.DB.Where("dept = ?", scopeSQL).Find(&courses).Error; err != nil {
			result.Error = "加载课程失败: " + err.Error()
			return result
		}
	}
	result.TotalCourses = len(courses)
	if len(courses) == 0 {
		result.Error = "没有找到课程"
		return result
	}
	log(fmt.Sprintf("INFO 排课引擎启动，共 %d 门课程待排", len(courses)))

	// Load resources
	var classrooms []models.Classroom
	if err := database.DB.Where("status = ?", "available").Find(&classrooms).Error; err != nil {
		result.Error = "加载教室失败: " + err.Error()
		return result
	}
	var teachers []models.Teacher
	if err := database.DB.Where("status = ?", "active").Find(&teachers).Error; err != nil {
		result.Error = "加载教师失败: " + err.Error()
		return result
	}
	if len(classrooms) == 0 || len(teachers) == 0 {
		result.Error = "缺少教室或教师资源"
		return result
	}

	// Execute in transaction
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Clear existing schedule for this semester
		if err := tx.Where("semester = ?", config.Semester).Delete(&models.ScheduleEntry{}).Error; err != nil {
			return fmt.Errorf("清空旧课表失败: %w", err)
		}

		validStarts := []int{0, 2, 4, 6, 8}
		roomOccupied := make(map[string]bool)
		teacherOccupied := make(map[string]bool)
		teacherLoad := make(map[uint]int)
		scheduled := 0

		for _, course := range courses {
			placed := false
			days := rng.Perm(7)
			for _, day := range days {
				if placed {
					break
				}
				starts := make([]int, len(validStarts))
				copy(starts, validStarts)
				rng.Shuffle(len(starts), func(i, j int) { starts[i], starts[j] = starts[j], starts[i] })

				for _, start := range starts {
					if placed {
						break
					}
					span := 2
					if start == 8 && rng.Intn(3) == 0 {
						span = 3 // 9-10-11三连上
					}
					if start+span > 11 {
						continue
					}

					teacherCandidates := findTeachers(teachers, course.Dept)
					rng.Shuffle(len(teacherCandidates), func(i, j int) { teacherCandidates[i], teacherCandidates[j] = teacherCandidates[j], teacherCandidates[i] })

					for _, teacher := range teacherCandidates {
						if teacherLoad[teacher.ID] >= 16*2 { // max 16 periods worth of courses
							continue
						}
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
							if err := tx.Create(&entry).Error; err != nil {
								continue
							}

							for p := start; p < start+span; p++ {
								roomOccupied[fmt.Sprintf("%d-%d-%d", day, p, room.ID)] = true
								teacherOccupied[fmt.Sprintf("%d-%d-%d", day, p, teacher.ID)] = true
							}
							teacherLoad[teacher.ID] += span // 按节次累计
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

		result.Scheduled = scheduled
		return nil
	})

	if err != nil {
		result.Error = "排课事务失败: " + err.Error()
		log("ERR " + err.Error())
		return result
	}

	// Conflict detection AFTER transaction (sees committed data)
	conflicts := (&ConflictService{}).DetectConflicts(config.Semester)
	result.Conflicts = len(conflicts)
	if result.TotalCourses > 0 {
		result.Utilization = float64(result.Scheduled) / float64(result.TotalCourses)
	}
	log(fmt.Sprintf("INFO 排课完成！已排 %d/%d 门，利用率 %.1f%%，冲突 %d 个",
		result.Scheduled, result.TotalCourses, result.Utilization*100, len(conflicts)))
	if result.Scheduled < result.TotalCourses {
		log(fmt.Sprintf("WARN 剩余 %d 门课程需手动调整", result.TotalCourses-result.Scheduled))
	}

	return result
}

// deptMap maps course dept codes to teacher dept names (Chinese)
var deptMap = map[string]string{
	"mech": "机械工程学院", "elec": "电气与电子工程学院",
	"mate": "材料与化学工程学院", "bio": "生物工程与食品学院",
	"civil": "土木建筑与环境学院", "cs": "计算机学院",
	"art": "艺术设计学院", "design": "工业设计学院",
	"econ": "经济与管理学院", "eng": "外国语学院",
	"sci": "理学院", "marx": "马克思主义学院",
	"voc": "职业技术师范学院", "intl": "国际学院",
	"pe": "体育学院", "cont": "继续教育学院",
	"innov": "创新创业学院", "engtech": "工程技术学院",
	"detroit": "底特律绿色工业学院",
}

// reverseDeptMap maps Chinese names back to codes (for scope filtering)
var reverseDeptMap = map[string]string{}

func init() {
	for k, v := range deptMap {
		reverseDeptMap[v] = k
	}
}

func findTeachers(teachers []models.Teacher, courseDept string) []models.Teacher {
	targetDept := deptMap[courseDept]
	var same, other []models.Teacher
	for _, t := range teachers {
		if t.Dept == targetDept {
			same = append(same, t)
		} else {
			other = append(other, t)
		}
	}
	return append(same, other...)
}
