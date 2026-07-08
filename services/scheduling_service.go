package services

import (
	"encoding/json"
	"fmt"
	"scheduling-system/database"
	"scheduling-system/models"
	"strings"
	"time"
)

type SchedulingService struct {
	db           database.DB
	snapshots    *SnapshotService
	orchestrator *SolverOrchestrator
}

func NewSchedulingService(db database.DB, snapshots *SnapshotService, orchestrator *SolverOrchestrator) *SchedulingService {
	return &SchedulingService{db: db, snapshots: snapshots, orchestrator: orchestrator}
}

type SchedulingConfig struct {
	Scope       string           `json:"scope"`
	Semester    string           `json:"semester"`
	Strategy    string           `json:"strategy"`
	Iterations  int              `json:"iterations"`
	TimeLimit   int              `json:"timeLimit"` // max solve time in seconds, default 60
	Constraints []string         `json:"constraints"`
	LockedSlots []lockedTimeSlot `json:"lockedSlots,omitempty"`
	SemesterID  uint             `json:"semesterId,omitempty"` // active semester ID
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
	DayOfWeek   models.DayOfWeek `json:"dayOfWeek"`
	StartPeriod models.Period    `json:"startPeriod"`
	Span        int              `json:"span"`
}

func (s *SchedulingService) RunScheduling(config SchedulingConfig) *SchedulingResult {
	result := &SchedulingResult{Logs: []string{}}
	log := func(msg string) {
		result.Logs = append(result.Logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
	}

	// Load teaching tasks for the active semester
	var teachingTasks []models.TeachingTask
	taskQuery := s.db.Where("status = ?", "active")
	if config.SemesterID > 0 {
		taskQuery = taskQuery.Where("semester_id = ?", config.SemesterID)
	}
	if err := taskQuery.Find(&teachingTasks).Error(); err != nil {
		result.Error = "加载教学任务失败: " + err.Error()
		return result
	}

	// Filter by scope if needed
	if config.Scope != "" && config.Scope != "全校所有院系" {
		scopeCode := reverseDeptMap[config.Scope]
		if scopeCode == "" {
			scopeCode = config.Scope
		}
		// Filter teaching tasks by course department
		var filtered []models.TeachingTask
		for _, tt := range teachingTasks {
			var course models.Course
			if err := s.db.First(&course, tt.CourseID).Error(); err == nil {
				if course.Dept == scopeCode {
					filtered = append(filtered, tt)
				}
			}
		}
		teachingTasks = filtered
	}

	// Load classes for each teaching task
	for i := range teachingTasks {
		var classes []models.TeachingTaskClass
		if err := s.db.Where("teaching_task_id = ?", teachingTasks[i].ID).
			Preload("ClassGroup").Find(&classes).Error(); err == nil {
			teachingTasks[i].Classes = classes
		}
		// Preload course and teacher
		s.db.First(&teachingTasks[i].Course, teachingTasks[i].CourseID)
		s.db.First(&teachingTasks[i].Teacher, teachingTasks[i].TeacherID)
	}

	result.TotalCourses = len(teachingTasks)
	if len(teachingTasks) == 0 {
		result.Error = "没有找到教学任务，请先在资源管理中创建教学任务"
		return result
	}
	log(fmt.Sprintf("排课引擎启动（模拟退火），共 %d 个教学任务待排", len(teachingTasks)))

	// Load resources
	var classrooms []models.Classroom
	if err := s.db.Where("status = ?", "available").Find(&classrooms).Error(); err != nil {
		result.Error = "加载教室失败: " + err.Error()
		return result
	}
	var teachers []models.Teacher
	if err := s.db.Where("status = ?", "active").Find(&teachers).Error(); err != nil {
		result.Error = "加载教师失败: " + err.Error()
		return result
	}
	var classGroups []models.ClassGroup
	s.db.Find(&classGroups)

	if len(classrooms) == 0 || len(teachers) == 0 {
		result.Error = "缺少教室或教师资源"
		return result
	}

	// Load locked time slots
	lockedSlots := config.LockedSlots
	if len(lockedSlots) == 0 {
		lockedSlots = s.loadLockedSlots()
	}
	if len(lockedSlots) > 0 {
		log(fmt.Sprintf("加载了 %d 个全局锁定时间段", len(lockedSlots)))
	}

	// Configure SA solver
	saConfig := defaultSAConfig()
	if config.Iterations > 0 {
		saConfig.IterationsPerTemp = config.Iterations
	}
	if config.TimeLimit > 0 {
		saConfig.MaxTimeSeconds = float64(config.TimeLimit)
	} else {
		saConfig.MaxTimeSeconds = 60
	}

	log(fmt.Sprintf("模拟退火参数: 初始温度=%.1f, 冷却率=%.2f, 最长求解时间=%.0fs",
		saConfig.InitialTemp, saConfig.CoolingRate, saConfig.MaxTimeSeconds))

	// Try OR-Tools first if available
	var saResult *SAResult
	sportsCourseIDs := s.buildSportsCourseIDs(teachingTasks)
	if s.orchestrator != nil {
		if ortoolsResult := s.tryORTools(teachingTasks, teachers, classrooms, lockedSlots, config, log); ortoolsResult != nil {
			saResult = ortoolsResult
		}
	}

	// Fall back to SA solver if OR-Tools didn't produce a result
	if saResult == nil {
		solver := NewSASolver()
		saResult = solver.SolveMultiRun(
			teachingTasks, teachers, classrooms, classGroups,
			lockedSlots, config.Constraints, config.Semester,
			saConfig,
			3, // 3 runs with different seeds
			nil,
			nil,
		)

		// Greedy post-optimization on worst entries
		saResult.Entries = solver.PostOptimize(
			saResult.Entries, teachingTasks, teachers, classrooms,
			lockedSlots, config.Constraints,
			max(5, len(saResult.Entries)/10),
		)

		// Re-score after post-optimization
		postBreakdown := (&ScoringService{}).ScoreSchedule(saResult.Entries, teachers, classrooms, config.Constraints, sportsCourseIDs)
		saResult.Score = postBreakdown.Total

		log(fmt.Sprintf("SA求解完成: %d次迭代, %.1fms, 最优分=%.1f",
			saResult.Iterations, float64(saResult.ElapsedMs), saResult.Score))
		}

		// Save result to database
	err := s.db.Transaction(func(tx database.DB) error {
		// Hard-delete old entries for the semester
		if err := tx.Unscoped().Where("semester = ?", config.Semester).Delete(&models.ScheduleEntry{}).Error(); err != nil {
			return fmt.Errorf("清空旧课表失败: %w", err)
		}
		if len(saResult.Entries) > 0 {
			if err := tx.Create(&saResult.Entries).Error(); err != nil {
				return fmt.Errorf("保存课表失败: %w", err)
			}
		}
		result.Scheduled = saResult.Scheduled
		return nil
	})

	if err != nil {
		result.Error = "排课事务失败: " + err.Error()
		log("ERR " + err.Error())
		return result
	}

	// Quick conflict count on best result
	result.Conflicts = s.countConflictsQuick(saResult.Entries)
	if result.TotalCourses > 0 {
		result.Utilization = float64(len(saResult.Entries)) / float64(result.TotalCourses)
	}
	result.Score = saResult.Score

	// Re-score on final data for detailed breakdown
	scorer := NewScoringService()
	finalBreakdown := scorer.ScoreSchedule(saResult.Entries, teachers, classrooms, config.Constraints, sportsCourseIDs)
	result.ScoreDetail = &finalBreakdown

	log(fmt.Sprintf("排课完成！已排 %d/%d 个教学任务，利用率 %.1f%%，评分 %.1f/100",
		len(saResult.Entries), result.TotalCourses, result.Utilization*100, saResult.Score))
	if len(saResult.Entries) < result.TotalCourses {
		log(fmt.Sprintf("WARN 剩余 %d 个教学任务需手动调整", result.TotalCourses-len(saResult.Entries)))
	}

	// Auto-snapshot after scheduling
	if s.snapshots != nil && len(saResult.Entries) > 0 {
		_, snapErr := s.snapshots.CreateSnapshot(
			config.Semester, config.Scope, "auto", "simulated_annealing",
			saResult.Entries, teachers, classrooms, config.Constraints,
			saResult.ElapsedMs, result.Conflicts,
		)
		if snapErr != nil {
			log("WARN 快照保存失败: " + snapErr.Error())
		} else {
			log("快照已自动保存")
		}
	}

	return result
}

// buildSportsCourseIDs returns a set of course IDs that are "体育" courses.
func (s *SchedulingService) buildSportsCourseIDs(teachingTasks []models.TeachingTask) map[uint]bool {
	ids := make(map[uint]bool)
	for _, tt := range teachingTasks {
		if strings.Contains(tt.Course.Name, "体育") {
			ids[tt.CourseID] = true
		}
	}
	return ids
}

// loadLockedSlots reads locked time slots from the settings table.
func (s *SchedulingService) loadLockedSlots() []lockedTimeSlot {
	var setting models.Setting
	if err := s.db.Where("key = ?", "locked_time_slots").First(&setting).Error(); err != nil {
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
		for p := e.StartPeriod; p < e.StartPeriod+models.Period(e.Span); p++ {
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
		for p := e.StartPeriod; p < e.StartPeriod+models.Period(e.Span); p++ {
			key := fmt.Sprintf("t-%d-%d-%d", e.TeacherID, e.DayOfWeek, p)
			if teacherSlots[key] {
				count++
			}
			teacherSlots[key] = true
		}
	}

	// Class group conflicts (using TeachingTask data)
	classSlots := make(map[string]bool)
	for _, e := range entries {
		for p := e.StartPeriod; p < e.StartPeriod+models.Period(e.Span); p++ {
			key := fmt.Sprintf("c-%d-%d-%d", e.TeachingTaskID, e.DayOfWeek, p)
			if classSlots[key] {
				count++
			}
			classSlots[key] = true
		}
	}

	return count
}

// tryORTools attempts to solve the scheduling problem using the OR-Tools microservice.
// Returns nil if OR-Tools is unavailable or fails, in which case the caller should use SA.
func (s *SchedulingService) tryORTools(
	teachingTasks []models.TeachingTask,
	teachers []models.Teacher,
	classrooms []models.Classroom,
	lockedSlots []lockedTimeSlot,
	config SchedulingConfig,
	log func(string),
) *SAResult {
	if s.orchestrator == nil || !s.orchestrator.IsORToolsAvailable() {
		log("OR-Tools不可用，使用SA求解")
		return nil
	}

	client := s.orchestrator.GetORToolsClient()
	if client == nil {
		return nil
	}

	log("尝试使用OR-Tools CP-SAT精确求解...")

	// Build OR-Tools input
	input := ORToolsInput{
		Constraints:      config.Constraints,
		LockedSlots:      lockedSlots,
		TimeLimitSeconds: config.TimeLimit,
	}
	if input.TimeLimitSeconds <= 0 {
		input.TimeLimitSeconds = 60
	}

	// Map classrooms
	for _, c := range classrooms {
		input.Classrooms = append(input.Classrooms, ORToolsRoom{
			ID: c.ID, Capacity: c.Capacity,
		})
	}

	// Map teachers
	for _, t := range teachers {
		input.Teachers = append(input.Teachers, ORToolsTeacher{
			ID: t.ID, PreferNoEarly: t.PreferNoEarly, PreferNoLate: t.PreferNoLate,
		})
	}

	// Map teaching tasks
	taskMap := make(map[uint]models.TeachingTask)
	for _, tt := range teachingTasks {
		classIDs := make([]uint, len(tt.Classes))
		for j, c := range tt.Classes {
			classIDs[j] = c.ClassGroupID
		}
		input.TeachingTasks = append(input.TeachingTasks, ORToolsTask{
			ID: tt.ID, TeacherID: tt.TeacherID, ClassIDs: classIDs,
		})
		taskMap[tt.ID] = tt
	}

	// Call OR-Tools
	output, err := client.Solve(input)
	if err != nil {
		log(fmt.Sprintf("OR-Tools求解失败: %v，降级到SA", err))
		return nil
	}

	if output.Status == "error" || len(output.Entries) == 0 {
		log(fmt.Sprintf("OR-Tools返回空解(status=%s)，降级到SA", output.Status))
		return nil
	}

	// Convert OR-Tools output to ScheduleEntry
	var entries []models.ScheduleEntry
	for _, e := range output.Entries {
		tt, ok := taskMap[e.TaskID]
		if !ok {
			continue
		}
		entry := models.ScheduleEntry{
			CourseID:       tt.CourseID,
			TeacherID:      e.TeacherID,
			ClassroomID:    e.ClassroomID,
			TeachingTaskID: &e.TaskID,
			Semester:       config.Semester,
			DayOfWeek:      models.DayOfWeek(e.DayOfWeek),
			StartPeriod:    models.Period(e.StartPeriod),
			Span:           e.Span,
			Weeks:          "1-16",
		}
		entries = append(entries, entry)
	}

	log(fmt.Sprintf("OR-Tools求解完成(status=%s): %d个条目, %.1fms, 分=%.1f",
		output.Status, len(entries), float64(output.ElapsedMs), output.Score))

	return &SAResult{
		Entries:   entries,
		Score:     output.Score,
		Scheduled: len(entries),
		ElapsedMs: int64(output.ElapsedMs),
	}
}

// deptMap maps course dept codes to teacher dept names (Chinese)
// Kept for scope filtering and backward compatibility.
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
