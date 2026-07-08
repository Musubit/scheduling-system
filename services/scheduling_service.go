package services

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"scheduling-system/database"
	"scheduling-system/models"
	"sort"
	"time"

	"gorm.io/gorm"
)

type SchedulingService struct{}

func NewSchedulingService() *SchedulingService { return &SchedulingService{} }

type SchedulingConfig struct {
	Scope       string           `json:"scope"`
	Semester    string           `json:"semester"`
	Strategy    string           `json:"strategy"`
	Iterations  int              `json:"iterations"`
	Constraints []string         `json:"constraints"`
	LockedSlots []lockedTimeSlot `json:"lockedSlots,omitempty"`
}

type SchedulingResult struct {
	TotalCourses int             `json:"totalCourses"`
	Scheduled    int             `json:"scheduled"`
	Conflicts    int             `json:"conflicts"`
	Utilization  float64         `json:"utilization"`
	Score        float64         `json:"score"`
	ScoreDetail  *ScoreBreakdown `json:"scoreDetail,omitempty"`
	Logs         []string        `json:"logs"`
	Error        string          `json:"error,omitempty"`
}

// LockedTimeSlot represents a globally locked time period.
type lockedTimeSlot struct {
	DayOfWeek   int `json:"dayOfWeek"`
	StartPeriod int `json:"startPeriod"`
	Span        int `json:"span"`
}

func (s *SchedulingService) RunScheduling(config SchedulingConfig) *SchedulingResult {
	result := &SchedulingResult{Logs: []string{}}
	log := func(msg string) {
		result.Logs = append(result.Logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
	}

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
	var classGroups []models.ClassGroup
	database.DB.Find(&classGroups)

	if len(classrooms) == 0 || len(teachers) == 0 {
		result.Error = "缺少教室或教师资源"
		return result
	}

	// Load locked time slots from settings (DB fallback)
	lockedSlots := config.LockedSlots
	if len(lockedSlots) == 0 {
		lockedSlots = s.loadLockedSlots()
	}
	if len(lockedSlots) > 0 {
		log(fmt.Sprintf("INFO 加载了 %d 个全局锁定时间段", len(lockedSlots)))
	}

	// Matching: find class groups for each course by department
	courseClassGroups := make(map[uint][]models.ClassGroup)
	for _, course := range courses {
		targetDept := deptMap[course.Dept]
		var matched []models.ClassGroup
		for _, cg := range classGroups {
			if cg.Dept == targetDept {
				matched = append(matched, cg)
			}
		}
		courseClassGroups[course.ID] = matched
	}

	// Run multiple iterations, keep the best result
	numIterations := config.Iterations
	if numIterations <= 0 {
		numIterations = 10
	}
	if numIterations > 100 {
		numIterations = 100
	}
	log(fmt.Sprintf("INFO 将进行 %d 轮迭代，取最优方案", numIterations))

	validStarts := []int{0, 2, 4, 6, 8}
	scorer := NewScoringService()

	var bestEntries []models.ScheduleEntry
	var bestScheduled int
	var bestScore = -1.0

	for iter := 0; iter < numIterations; iter++ {
		iterRng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(iter*1000)))
		var iterEntries []models.ScheduleEntry
		roomOccupied := make(map[string]bool)
		teacherOccupied := make(map[string]bool)
		classOccupied := make(map[string]bool)
		teacherLoad := make(map[uint]int)
		scheduled := 0

		for _, course := range courses {
			placed := false
			days := iterRng.Perm(7)
			for _, day := range days {
				if placed {
					break
				}
				// Check if this day+any period is locked
				dayLocked := false
				for _, ls := range lockedSlots {
					if ls.DayOfWeek == day {
						dayLocked = true
						break
					}
				}

				starts := make([]int, len(validStarts))
				copy(starts, validStarts)
				iterRng.Shuffle(len(starts), func(i, j int) { starts[i], starts[j] = starts[j], starts[i] })

				for _, start := range starts {
					if placed {
						break
					}
					span := 2
					if start == 8 && iterRng.Intn(3) == 0 {
						span = 3
					}
					if start+span > 11 {
						continue
					}

					// Check locked time slot overlap
					lockedConflict := false
					if dayLocked {
						for _, ls := range lockedSlots {
							if ls.DayOfWeek == day && periodsOverlap(start, span, ls.StartPeriod, ls.Span) {
								lockedConflict = true
								break
							}
						}
					}
					if lockedConflict {
						continue
					}

					// Find matching class groups for this course
					cgs := courseClassGroups[course.ID]

					teacherCandidates := findTeachers(teachers, course.Dept)
					iterRng.Shuffle(len(teacherCandidates), func(i, j int) {
						teacherCandidates[i], teacherCandidates[j] = teacherCandidates[j], teacherCandidates[i]
					})

					for _, teacher := range teacherCandidates {
						if teacherLoad[teacher.ID] >= 32 { // max 32 periods
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

						// Sort rooms by floor for teachers who prefer low floors
						rooms := classrooms
						if teacher.PreferLowFloor {
							rooms = make([]models.Classroom, len(classrooms))
							copy(rooms, classrooms)
							sort.Slice(rooms, func(i, j int) bool {
								return rooms[i].Floor < rooms[j].Floor
							})
						}

						for _, room := range rooms {
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

							// Pick a class group for this entry
							var classGroupID *uint
							if len(cgs) > 0 {
								// Check which class group is free at this time
								for _, cg := range cgs {
									cgBusy := false
									for p := start; p < start+span; p++ {
										if classOccupied[fmt.Sprintf("%d-%d-%d", day, p, cg.ID)] {
											cgBusy = true
											break
										}
									}
									if !cgBusy {
										cgid := cg.ID
										classGroupID = &cgid
										break
									}
								}
								// If all class groups busy, still place but without class group
							}

							entry := models.ScheduleEntry{
								CourseID:     course.ID,
								TeacherID:    teacher.ID,
								ClassroomID:  room.ID,
								ClassGroupID: classGroupID,
								Semester:     config.Semester,
								DayOfWeek:    day,
								StartPeriod:  start,
								Span:         span,
								Weeks:        "1-16",
							}
							iterEntries = append(iterEntries, entry)

							for p := start; p < start+span; p++ {
								roomOccupied[fmt.Sprintf("%d-%d-%d", day, p, room.ID)] = true
								teacherOccupied[fmt.Sprintf("%d-%d-%d", day, p, teacher.ID)] = true
								if classGroupID != nil {
									classOccupied[fmt.Sprintf("%d-%d-%d", day, p, *classGroupID)] = true
								}
							}
							teacherLoad[teacher.ID] += span
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
		}

		// Score this iteration
		scoreBreakdown := scorer.ScoreSchedule(iterEntries, teachers, classrooms, config.Constraints)
		iterScore := scoreBreakdown.Total

		if bestScore < 0 || iterScore > bestScore || (iterScore == bestScore && scheduled > bestScheduled) {
			bestScore = iterScore
			bestScheduled = scheduled
			bestEntries = iterEntries
		}

		if (iter+1)%10 == 0 || iter == numIterations-1 {
			log(fmt.Sprintf("INFO 迭代 %d/%d, 本轮评分 %.1f, 当前最优 %.1f",
				iter+1, numIterations, iterScore, bestScore))
		}
	}

	// Save best result to database
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("semester = ?", config.Semester).Delete(&models.ScheduleEntry{}).Error; err != nil {
			return fmt.Errorf("清空旧课表失败: %w", err)
		}
		if len(bestEntries) > 0 {
			if err := tx.Create(&bestEntries).Error; err != nil {
				return fmt.Errorf("保存课表失败: %w", err)
			}
		}
		result.Scheduled = bestScheduled
		return nil
	})

	if err != nil {
		result.Error = "排课事务失败: " + err.Error()
		log("ERR " + err.Error())
		return result
	}

	// Final conflict detection on committed data
	conflicts := (&ConflictService{}).DetectConflicts(config.Semester)
	result.Conflicts = len(conflicts)
	if result.TotalCourses > 0 {
		result.Utilization = float64(result.Scheduled) / float64(result.TotalCourses)
	}
	result.Score = bestScore

	// Re-score on final committed data for detailed breakdown
	var finalEntries []models.ScheduleEntry
	database.DB.Where("semester = ?", config.Semester).Find(&finalEntries)
	finalBreakdown := scorer.ScoreSchedule(finalEntries, teachers, classrooms, config.Constraints)
	result.ScoreDetail = &finalBreakdown

	log(fmt.Sprintf("INFO 排课完成！已排 %d/%d 门，利用率 %.1f%%，评分 %.1f/100，冲突 %d 个",
		result.Scheduled, result.TotalCourses, result.Utilization*100, bestScore, len(conflicts)))
	if result.Scheduled < result.TotalCourses {
		log(fmt.Sprintf("WARN 剩余 %d 门课程需手动调整", result.TotalCourses-result.Scheduled))
	}

	return result
}

// loadLockedSlots reads locked time slots from the settings table.
func (s *SchedulingService) loadLockedSlots() []lockedTimeSlot {
	var setting models.Setting
	if err := database.DB.Where("key = ?", "locked_time_slots").First(&setting).Error; err != nil {
		return nil
	}
	var slots []lockedTimeSlot
	if err := json.Unmarshal([]byte(setting.Value), &slots); err != nil {
		return nil
	}
	return slots
}

// countConflictsQuick does a fast in-memory conflict count without DB queries.
func (s *SchedulingService) countConflictsQuick(entries []models.ScheduleEntry) int {
	count := 0

	// Room conflicts
	roomSlots := make(map[string]bool)
	for _, e := range entries {
		for p := e.StartPeriod; p < e.StartPeriod+e.Span; p++ {
			key := fmt.Sprintf("r-%d-%d-%d", e.ClassroomID, e.DayOfWeek, p)
			if roomSlots[key] {
				count++
			}
			roomSlots[key] = true
		}
	}

	// Teacher conflicts
	teacherSlots := make(map[string]bool)
	for _, e := range entries {
		for p := e.StartPeriod; p < e.StartPeriod+e.Span; p++ {
			key := fmt.Sprintf("t-%d-%d-%d", e.TeacherID, e.DayOfWeek, p)
			if teacherSlots[key] {
				count++
			}
			teacherSlots[key] = true
		}
	}

	// Class group conflicts
	classSlots := make(map[string]bool)
	for _, e := range entries {
		if e.ClassGroupID == nil {
			continue
		}
		for p := e.StartPeriod; p < e.StartPeriod+e.Span; p++ {
			key := fmt.Sprintf("c-%d-%d-%d", *e.ClassGroupID, e.DayOfWeek, p)
			if classSlots[key] {
				count++
			}
			classSlots[key] = true
		}
	}

	return count
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
