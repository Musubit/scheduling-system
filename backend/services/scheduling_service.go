package services

import (
	"encoding/json"
	"fmt"
	"log"
	"scheduling-system/backend/database"
	"scheduling-system/backend/models"
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
		Scope            string            `json:"scope"`
		Semester         string            `json:"semester"`
		Strategy         string            `json:"strategy"`
		Iterations       int               `json:"iterations"`
		TimeLimit        int               `json:"timeLimit"` // max solve time in seconds, default 60
		Constraints      []string          `json:"constraints"`
		LockedSlotsJSON  string            `json:"lockedSlotsJson,omitempty"` // JSON string, avoids Wails enum serialization pitfall
		SemesterID       uint              `json:"semesterId,omitempty"`     // active semester ID
		ConstraintWeights map[string]int   `json:"constraintWeights,omitempty"` // per-constraint weights (0-100)
	}

type SchedulingResult struct {
	TotalCourses     int             `json:"totalCourses"`
	Scheduled        int             `json:"scheduled"`
	TasksScheduled   int             `json:"tasksScheduled"`
	Conflicts        int             `json:"conflicts"`
	TeacherConflicts int             `json:"teacherConflicts"`
	RoomConflicts    int             `json:"roomConflicts"`
	ClassConflicts   int             `json:"classConflicts"`
	Utilization      float64         `json:"utilization"`
	Score            float64         `json:"score"`
	ScoreDetail      *ScoreBreakdown `json:"scoreDetail,omitempty"`
	Logs             []string        `json:"logs"`
	Error            string          `json:"error,omitempty"`
}

// LockedTimeSlot represents a globally locked time period.
type LockedTimeSlot struct {
	DayOfWeek   models.DayOfWeek `json:"dayOfWeek"`
	StartPeriod models.Period    `json:"startPeriod"`
	Span        int              `json:"span"`
}

func (s *SchedulingService) RunScheduling(config SchedulingConfig) *SchedulingResult {
	log.Printf("[SCHED] RunScheduling CALLED: scope=%q semester=%q semesterId=%d lockedSlotsJsonLen=%d constraints=%v",
		config.Scope, config.Semester, config.SemesterID, len(config.LockedSlotsJSON), config.Constraints)

	result := &SchedulingResult{Logs: []string{}}
	addLog := func(msg string) {
		result.Logs = append(result.Logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
	}

	// Load teaching tasks for the active semester
	var teachingTasks []models.TeachingTask
	taskQuery := s.db.Where("status = ?", "active")
	if config.SemesterID > 0 {
		taskQuery = taskQuery.Where("semester_id = ?", config.SemesterID)
	}
	if err := taskQuery.Find(&teachingTasks).Error(); err != nil {
		log.Printf("[SCHED] EARLY-RETURN-1: load teachingTasks failed: %v", err)
		result.Error = "加载教学任务失败: " + err.Error()
		return result
	}
	log.Printf("[SCHED] teachingTasks loaded: count=%d", len(teachingTasks))

	// Filter by scope if needed (Course.Dept is now Chinese name, same as config.Scope)
	if config.Scope != "" && config.Scope != "全校所有院系" {
		var filtered []models.TeachingTask
		for _, tt := range teachingTasks {
			var course models.Course
			if err := s.db.First(&course, tt.CourseID).Error(); err != nil {
				log.Printf("[SCHED] WARN: scope filter - course ID=%d not found for task ID=%d, skipping", tt.CourseID, tt.ID)
				addLog(fmt.Sprintf("WARN 教学任务(ID=%d)关联的课程(ID=%d)不存在，已跳过", tt.ID, tt.CourseID))
				continue
			}
			if course.Dept == config.Scope {
				filtered = append(filtered, tt)
			}
		}
		teachingTasks = filtered
	}

	// Load classes for each teaching task
	for i := range teachingTasks {
		var classes []models.TeachingTaskClass
		if err := s.db.Where("teaching_task_id = ?", teachingTasks[i].ID).
			Preload("ClassGroup").Find(&classes).Error(); err != nil {
			log.Printf("[SCHED] WARN: failed to load classes for task ID=%d: %v", teachingTasks[i].ID, err)
			addLog(fmt.Sprintf("WARN 加载教学任务(ID=%d)的班级关联失败", teachingTasks[i].ID))
		} else {
			teachingTasks[i].Classes = classes
		}
		// Preload course and teacher
		if err := s.db.First(&teachingTasks[i].Course, teachingTasks[i].CourseID).Error(); err != nil {
			log.Printf("[SCHED] WARN: course ID=%d not found for task ID=%d: %v", teachingTasks[i].CourseID, teachingTasks[i].ID, err)
		}
		if err := s.db.First(&teachingTasks[i].Teacher, teachingTasks[i].TeacherID).Error(); err != nil {
			log.Printf("[SCHED] WARN: teacher ID=%d not found for task ID=%d: %v", teachingTasks[i].TeacherID, teachingTasks[i].ID, err)
		}
	}

	result.TotalCourses = len(teachingTasks)
	if len(teachingTasks) == 0 {
		log.Printf("[SCHED] EARLY-RETURN-2: zero teaching tasks")
		result.Error = "没有找到教学任务，请先在资源管理中创建教学任务"
		return result
	}
	addLog(fmt.Sprintf("排课引擎启动（模拟退火），共 %d 个教学任务待排", len(teachingTasks)))

	// Load resources
	var classrooms []models.Classroom
	if err := s.db.Where("status = ?", "available").Find(&classrooms).Error(); err != nil {
		log.Printf("[SCHED] EARLY-RETURN-3: load classrooms failed: %v", err)
		result.Error = "加载教室失败: " + err.Error()
		return result
	}
	var teachers []models.Teacher
	if err := s.db.Where("status = ?", "active").Find(&teachers).Error(); err != nil {
		log.Printf("[SCHED] EARLY-RETURN-4: load teachers failed: %v", err)
		result.Error = "加载教师失败: " + err.Error()
		return result
	}
	var classGroups []models.ClassGroup
	s.db.Find(&classGroups)

	if len(classrooms) == 0 || len(teachers) == 0 {
		log.Printf("[SCHED] EARLY-RETURN-5: classrooms=%d teachers=%d", len(classrooms), len(teachers))
		result.Error = "缺少教室或教师资源"
		return result
	}
	log.Printf("[SCHED] resources loaded: classrooms=%d teachers=%d classGroups=%d", len(classrooms), len(teachers), len(classGroups))

	// Load locked time slots — merge from both frontend JSON and SQLite
		var lockedSlots []LockedTimeSlot
		seen := make(map[string]bool)
		addSlots := func(slots []LockedTimeSlot) {
			for _, s := range slots {
				key := fmt.Sprintf("%d-%d-%d", s.DayOfWeek, s.StartPeriod, s.Span)
				if !seen[key] {
					seen[key] = true
					lockedSlots = append(lockedSlots, s)
				}
			}
		}

		// 1. Parse frontend JSON
		if config.LockedSlotsJSON != "" {
			var parsed []LockedTimeSlot
			if err := json.Unmarshal([]byte(config.LockedSlotsJSON), &parsed); err == nil {
				addSlots(parsed)
				addLog(fmt.Sprintf("[DEBUG] 前端传入 %d 个锁定时段", len(parsed)))
			} else {
				addLog(fmt.Sprintf("[DEBUG] 前端JSON解析失败: %v", err))
			}
		}

		// 2. Load from database (always — as merge, not just fallback)
		dbSlots := s.loadLockedSlots()
		addSlots(dbSlots)
		if len(dbSlots) > 0 {
			addLog(fmt.Sprintf("[DEBUG] 数据库加载 %d 个锁定时段", len(dbSlots)))
		}

		if len(lockedSlots) > 0 {
			addLog(fmt.Sprintf("加载了 %d 个全局锁定时间段 (前端+数据库合并)", len(lockedSlots)))
		} else {
			addLog("未加载任何全局锁定时间段 — 锁定时段功能未启用")
		}

	log.Printf("[SCHED] lockedSlots: frontendJsonLen=%d dbSlots=%d merged=%d slots=%+v",
		len(config.LockedSlotsJSON), len(dbSlots), len(lockedSlots), lockedSlots)

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

	addLog(fmt.Sprintf("模拟退火参数: 初始温度=%.1f, 冷却率=%.2f, 最长求解时间=%.0fs",
		saConfig.InitialTemp, saConfig.CoolingRate, saConfig.MaxTimeSeconds))

	// Try OR-Tools first if available
	var saResult *SAResult
	sportsCourseIDs := s.buildSportsCourseIDs(teachingTasks)
	if s.orchestrator != nil {
		if ortoolsResult := s.tryORTools(teachingTasks, teachers, classrooms, classGroups, lockedSlots, config, sportsCourseIDs, addLog); ortoolsResult != nil {
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
			postBreakdown := (&ScoringService{}).ScoreSchedule(saResult.Entries, teachers, classrooms, config.Constraints, sportsCourseIDs, teachingTasks)
			saResult.Score = postBreakdown.Total

		addLog(fmt.Sprintf("SA求解完成: %d次迭代, %.1fms, 最优分=%.1f",
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
		addLog("ERR " + err.Error())
		return result
	}

	// Quick conflict count on best result — split by type
	teacherC, roomC, classC := s.countConflictsQuick(saResult.Entries)
	result.TeacherConflicts = teacherC
	result.RoomConflicts = roomC
	result.ClassConflicts = classC
	result.Conflicts = teacherC + roomC + classC

	// Count unique teaching tasks that got at least one entry
	taskSet := make(map[uint]bool)
	for _, e := range saResult.Entries {
		if e.TeachingTaskID != nil {
			taskSet[*e.TeachingTaskID] = true
		}
	}
	result.TasksScheduled = len(taskSet)
	if result.TotalCourses > 0 {
		result.Utilization = float64(result.TasksScheduled) / float64(result.TotalCourses)
	}
	result.Score = saResult.Score

	// Re-score on final data for detailed breakdown
		scorer := NewScoringService()
		finalBreakdown := scorer.ScoreSchedule(saResult.Entries, teachers, classrooms, config.Constraints, sportsCourseIDs, teachingTasks)
		result.ScoreDetail = &finalBreakdown

	addLog(fmt.Sprintf("排课完成！任务 %d/%d（%d条），评分 %.1f/100，冲突 教师%d 教室%d 班级%d",
		result.TasksScheduled, result.TotalCourses, len(saResult.Entries), saResult.Score,
		result.TeacherConflicts, result.RoomConflicts, result.ClassConflicts))
	if result.TasksScheduled < result.TotalCourses {
		addLog(fmt.Sprintf("WARN 剩余 %d 个教学任务未能排入", result.TotalCourses-result.TasksScheduled))
	}

	// Auto-snapshot after scheduling
	if s.snapshots != nil && len(saResult.Entries) > 0 {
		_, snapErr := s.snapshots.CreateSnapshot(
			config.Semester, config.Scope, "auto", "simulated_annealing",
			saResult.Entries, teachers, classrooms, config.Constraints,
			saResult.ElapsedMs, result.Conflicts,
		)
		if snapErr != nil {
			addLog("WARN 快照保存失败: " + snapErr.Error())
		} else {
			addLog("快照已自动保存")
		}
	}

	log.Printf("[SCHED] RunScheduling DONE: totalCourses=%d scheduled=%d conflicts=%d logs=%d",
		result.TotalCourses, result.Scheduled, result.Conflicts, len(result.Logs))

	return result
}

// buildSportsCourseIDs returns a set of course IDs that are "体育" courses.
func (s *SchedulingService) buildSportsCourseIDs(teachingTasks []models.TeachingTask) map[uint]bool {
	ids := make(map[uint]bool)
	for _, tt := range teachingTasks {
		if models.IsSportsCourse(tt.Course.Name) {
			ids[tt.CourseID] = true
		}
	}
	return ids
}

// containsKeyword checks if s contains the given keyword (case-insensitive).
func containsKeyword(s, keyword string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(keyword))
}

// loadLockedSlots reads locked time slots from the settings table.
// Package-level function so MoveService can also use it without coupling to SchedulingService.
func loadLockedSlotsDB(db database.DB) []LockedTimeSlot {
	var setting models.Setting
	if err := db.Where("key = ?", "locked_time_slots").First(&setting).Error(); err != nil {
		return nil
	}
	var slots []LockedTimeSlot
	if err := json.Unmarshal([]byte(setting.Value), &slots); err != nil {
		return nil
	}
	return slots
}

func (s *SchedulingService) loadLockedSlots() []LockedTimeSlot {
	return loadLockedSlotsDB(s.db)
}

// countConflictsQuick does a fast in-memory conflict count without DB queries.
// Returns teacher, room, and class conflict counts separately.
func (s *SchedulingService) countConflictsQuick(entries []models.ScheduleEntry) (teacher, room, class int) {
	// Teacher conflicts
	teacherSlots := make(map[string]bool)
	for _, e := range entries {
		for p := e.StartPeriod; p < e.StartPeriod+models.Period(e.Span); p++ {
			key := fmt.Sprintf("t-%d-%d-%d", e.TeacherID, e.DayOfWeek, p)
			if teacherSlots[key] {
				teacher++
			}
			teacherSlots[key] = true
		}
	}

	// Room conflicts
	roomSlots := make(map[string]bool)
	for _, e := range entries {
		for p := e.StartPeriod; p < e.StartPeriod+models.Period(e.Span); p++ {
			key := fmt.Sprintf("r-%d-%d-%d", e.ClassroomID, e.DayOfWeek, p)
			if roomSlots[key] {
				room++
			}
			roomSlots[key] = true
		}
	}

	// Class group conflicts (using TeachingTask data)
	classSlots := make(map[string]bool)
	for _, e := range entries {
		for p := e.StartPeriod; p < e.StartPeriod+models.Period(e.Span); p++ {
			key := fmt.Sprintf("c-%d-%d-%d", e.TeachingTaskID, e.DayOfWeek, p)
			if classSlots[key] {
				class++
			}
			classSlots[key] = true
		}
	}

	return
}

// tryORTools attempts to solve the scheduling problem using the OR-Tools microservice.
// Returns nil if OR-Tools is unavailable or fails, in which case the caller should use SA.
func (s *SchedulingService) tryORTools(
	teachingTasks []models.TeachingTask,
	teachers []models.Teacher,
	classrooms []models.Classroom,
	classGroups []models.ClassGroup,
	lockedSlots []LockedTimeSlot,
	config SchedulingConfig,
	sportsCourseIDs map[uint]bool,
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
		Constraints:       config.Constraints,
		LockedSlots:       lockedSlots,
		ConstraintWeights: make(map[string]int),
		TimeLimitSeconds:  config.TimeLimit,
	}
	if input.TimeLimitSeconds <= 0 {
		input.TimeLimitSeconds = 60
	}

	// Map classrooms (include type for room-type matching)
	for _, c := range classrooms {
		input.Classrooms = append(input.Classrooms, ORToolsRoom{
			ID: c.ID, Floor: c.Floor, Capacity: c.Capacity, Type: c.Type,
		})
	}

		// Map teachers
		for _, t := range teachers {
			input.Teachers = append(input.Teachers, ORToolsTeacher{
				ID: t.ID, Name: t.Name,
				PreferNoEarly: t.PreferNoEarly, PreferNoLate: t.PreferNoLate,
				MaxDaysPerWeek: t.MaxDaysPerWeek, PreferLowFloor: t.PreferLowFloor,
				UnavailableSlots: t.UnavailableSlots,
			})
		}

	// Map class groups (needed for capacity constraint)
	for _, cg := range classGroups {
		input.ClassGroups = append(input.ClassGroups, ORToolsClassGroup{
			ID: cg.ID, Students: cg.Students,
		})
	}

	// Map teaching tasks (with multi-session + room type support)
	taskMap := make(map[uint]models.TeachingTask)
	for _, tt := range teachingTasks {
		classIDs := make([]uint, len(tt.Classes))
		for j, c := range tt.Classes {
			classIDs[j] = c.ClassGroupID
		}
		// Determine required room type from course name
			requiredRoomType := ""
			courseName := tt.Course.Name
			if models.IsSportsCourse(courseName) {
				requiredRoomType = "体育馆"
			} else if containsKeyword(courseName, "实验") {
				requiredRoomType = "实验室"
			} else if containsKeyword(courseName, "上机") {
				requiredRoomType = "机房"
			}

			// TotalHours fallback: same as SA solver (sa_solver.go line 136-138)
			totalHours := tt.TotalHours
			if totalHours <= 0 {
				totalHours = tt.Course.Hours
			}

		input.TeachingTasks = append(input.TeachingTasks, ORToolsTask{
			ID:               tt.ID,
			TeacherID:        tt.TeacherID,
			CourseID:         tt.CourseID,
			ClassIDs:         classIDs,
			SessionsPerWeek:  0, // let solver.py compute from TotalHours
			TotalHours:       totalHours,
			MaxHoursPerWeek:  tt.MaxHoursPerWeek,
			RequiredRoomType: requiredRoomType,
			StartWeek:        tt.StartWeek,
			EndWeek:          tt.EndWeek,
		})
		taskMap[tt.ID] = tt
	}

	// Sports course IDs
	for cid := range sportsCourseIDs {
		input.SportsCourseIDs = append(input.SportsCourseIDs, cid)
	}

	// Constraint weights: use frontend-provided values, fallback to defaults
	weightDefaults := map[string]int{
		"teacher_preference": 50, "course_dispersed": 50,
		"teacher_days_limit": 50, "low_floor_preference": 50,
		"avoid_saturday": 30, "avoid_sunday": 30,
		"pe_preferred_periods": 50, "student_fatigue": 50,
	}
	for _, c := range config.Constraints {
		if w, ok := weightDefaults[c]; ok {
			input.ConstraintWeights[c] = w
		}
	}
	// Override with frontend-provided weights (if any)
	if config.ConstraintWeights != nil {
		for k, v := range config.ConstraintWeights {
			input.ConstraintWeights[k] = v
		}
	}

	// Call OR-Tools
	output, err := client.Solve(input)
	if err != nil {
		log(fmt.Sprintf("OR-Tools求解失败: %v，降级到SA", err))
		return nil
	}

	if output.Status == "error" || len(output.Entries) == 0 {
		log(fmt.Sprintf("OR-Tools返回空解(status=%s)，降级到SA", output.Status))
		if len(output.Conflicts) > 0 {
			for _, c := range output.Conflicts {
				log(fmt.Sprintf("  冲突诊断: %s", c))
			}
		}
		return nil
	}
	if output.Status == "infeasible" {
		log(fmt.Sprintf("OR-Tools不可满足(INFEASIBLE)，降级到SA"))
		if len(output.Conflicts) > 0 {
			for _, c := range output.Conflicts {
				log(fmt.Sprintf("  冲突诊断: %s", c))
			}
		}
		return nil
	}

	// Convert OR-Tools output to ScheduleEntry
	// OR-Tools handles multi-session internally (each task can produce multiple entries)
	var entries []models.ScheduleEntry
	for _, e := range output.Entries {
		tt, ok := taskMap[e.TaskID]
		if !ok {
			continue
		}
		weeks := fmt.Sprintf("%d-%d", tt.StartWeek, tt.EndWeek)
		if tt.StartWeek <= 0 {
			weeks = "1-16"
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
			Weeks:          weeks,
		}
		entries = append(entries, entry)
	}

	log(fmt.Sprintf("OR-Tools求解完成(status=%s): %d/%d会话, %.1fms, 分=%.1f",
		output.Status, output.SessionsPlaced, output.SessionsExpected, float64(output.ElapsedMs), output.Score))

	// Enrich unplaced diagnostics with Go-side course/class names
	if output.SessionsPlaced < output.SessionsExpected && len(output.UnplacedDiagnostics) > 0 {
		for _, d := range output.UnplacedDiagnostics {
			log(fmt.Sprintf("  未排入诊断(Python): %s", d))
		}
		// Supplement with course + class names from Go taskMap
		// Count placed sessions per task to detect partial placement
		placedCount := make(map[uint]int)
		for _, e := range output.Entries {
			placedCount[e.TaskID]++
		}
		for _, tt := range teachingTasks {
			// Compute expected sessions (same formula as Python solver.py)
			weeks := tt.EndWeek - tt.StartWeek + 1
			if weeks < 1 {
				weeks = 1
			}
			totalHours := tt.TotalHours
			if totalHours <= 0 {
				totalHours = tt.Course.Hours
			}
			weeklyHours := float64(totalHours) / float64(weeks)
			expectedSessions := int(weeklyHours/2.0 + 0.999)
			if expectedSessions < 1 {
				expectedSessions = 1
			}
			if expectedSessions > 4 {
				expectedSessions = 4
			}
			if tt.MaxHoursPerWeek > 0 {
				maxSess := tt.MaxHoursPerWeek / 2
				if maxSess < 1 {
					maxSess = 1
				}
				if expectedSessions > maxSess {
					expectedSessions = maxSess
				}
			}
			actualPlaced := placedCount[tt.ID]
			if actualPlaced >= expectedSessions {
				continue
			}
			classNames := make([]string, 0)
			for _, c := range tt.Classes {
				if c.ClassGroup.Name != "" {
					classNames = append(classNames, c.ClassGroup.Name)
				}
			}
			log(fmt.Sprintf("  未排入补充(Go): 课程=%s 教师=%s 班级=%v 已排%d/%d个session",
				tt.Course.Name, tt.Teacher.Name, classNames, actualPlaced, expectedSessions))
		}
	}

	return &SAResult{
		Entries:   entries,
		Score:     output.Score,
		Scheduled: len(entries),
		ElapsedMs: int64(output.ElapsedMs),
	}
}

