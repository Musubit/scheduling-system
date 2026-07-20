package services

import (
	"encoding/json"
	"fmt"
	"log"
	"runtime/debug"
	"scheduling-system/backend/database"
	"scheduling-system/backend/models"
	schedtypes "scheduling-system/backend/scheduling/types"
	"sync"
	"sync/atomic"
	"time"
)

type SchedulingService struct {
	db                database.DB
	versions          *VersionService
	orchestrator      *SolverOrchestrator
	runMu             sync.Mutex
	schedulingRunning atomic.Bool

	// Cross-run best result cache: only overwritten when a new run scores higher.
	bestCachedScore   float64
	bestCachedEntries []models.ScheduleEntry
	bestCachedResult  *SchedulingResult
}

func NewSchedulingService(db database.DB, versions *VersionService, orchestrator *SolverOrchestrator) *SchedulingService {
	return &SchedulingService{db: db, versions: versions, orchestrator: orchestrator}
}

type SchedulingConfig struct {
	Scope             string                    `json:"scope"`
	Mode              schedtypes.SchedulingMode `json:"mode,omitempty"`
	Strategy          string                    `json:"strategy"`
	Iterations        int                       `json:"iterations"`
	TimeLimit         int                       `json:"timeLimit"` // max solve time in seconds, default 60
	Constraints       []string                  `json:"constraints"`
	LockedSlotsJSON   string                    `json:"lockedSlotsJson,omitempty"`   // JSON string, avoids Wails enum serialization pitfall
	SemesterID        uint                      `json:"semesterId,omitempty"`        // active semester ID
	ConstraintWeights map[string]int            `json:"constraintWeights,omitempty"` // per-constraint weights (0-100)
	MaxRetries        int                       `json:"maxRetries,omitempty"`        // Orchestrator internal retry limit, 0 = no retry
}

type SchedulingResult struct {
	Mode             string             `json:"mode,omitempty"`
	TotalCourses     int                `json:"totalCourses"`
	Scheduled        int                `json:"scheduled"`
	TasksScheduled   int                `json:"tasksScheduled"`
	Conflicts        int                `json:"conflicts"`
	TeacherConflicts int                `json:"teacherConflicts"`
	RoomConflicts    int                `json:"roomConflicts"`
	ClassConflicts   int                `json:"classConflicts"`
	Utilization      float64            `json:"utilization"`
	Score            float64            `json:"score"`
	ScoreDetail      *ScoreBreakdown    `json:"scoreDetail,omitempty"`
	Logs             []string           `json:"logs"`
	Error            string             `json:"error,omitempty"`
	ProgressHistory  []ScheduleProgress `json:"progressHistory,omitempty"`
	UnplacedTasks    []UnplacedTask     `json:"unplacedTasks,omitempty"`
}

// UnplacedTask describes a teaching task that could not be scheduled.
type UnplacedTask struct {
	TaskID       uint   `json:"taskId"`
	CourseName   string `json:"courseName"`
	TeacherName  string `json:"teacherName"`
	ClassName    string `json:"className"`
	RequiredRoom string `json:"requiredRoom,omitempty"` // COMPUTER / LAB / GYM / ""
	Placed       int    `json:"placed"`
	Expected     int    `json:"expected"`
	RootCause    string `json:"rootCause,omitempty"` // 确诊 or 疑似
}

// ScheduleProgress records a scheduling phase milestone.
type ScheduleProgress struct {
	Progress int    `json:"progress"` // 0-100 percentage
	Stage    string `json:"stage"`    // human-readable phase name
}

// LockedTimeSlot represents a globally locked time period.
type LockedTimeSlot struct {
	DayOfWeek   models.DayOfWeek `json:"dayOfWeek"`
	StartPeriod models.Period    `json:"startPeriod"`
	Span        int              `json:"span"`
}

func (s *SchedulingService) RunScheduling(config SchedulingConfig) *SchedulingResult {
	if !s.schedulingRunning.CompareAndSwap(false, true) {
		return &SchedulingResult{Error: "排课任务已在运行", Logs: []string{}}
	}
	s.runMu.Lock()
	defer s.runMu.Unlock()
	defer s.schedulingRunning.Store(false)

	result := &SchedulingResult{Logs: []string{}}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[SCHED] PANIC recovered: %v\n%s", r, debug.Stack())
			result.Error = fmt.Sprintf("排课引擎异常: %v", r)
		}
	}()

	mode := config.Mode
	if !mode.IsValid() {
		mode = schedtypes.ModeFullScheduling
	}
	config.Mode = mode
	isTimeOnly := mode.IsTimeOnly()
	effectiveConstraints := constraintsForMode(config.Constraints, mode)

	// Clamp constraint weights to valid range [0, 100]
	for k, v := range config.ConstraintWeights {
		if v < 0 {
			config.ConstraintWeights[k] = 0
		} else if v > 100 {
			config.ConstraintWeights[k] = 100
		}
	}

	log.Printf("[SCHED] RunScheduling CALLED: scope=%q semesterId=%d lockedSlotsJsonLen=%d constraints=%v",
		config.Scope, config.SemesterID, len(config.LockedSlotsJSON), effectiveConstraints)

	result.Mode = string(mode)
	addLog := func(msg string) {
		result.Logs = append(result.Logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
	}
	addProgress := func(progress int, stage string) {
		result.ProgressHistory = append(result.ProgressHistory, ScheduleProgress{
			Progress: progress,
			Stage:    stage,
		})
	}

	if isTimeOnly {
		addLog("排课模式: TIME_ONLY（关闭教室场地分配）")
	} else {
		addLog("排课模式: FULL（时间+教室）")
	}

	// Phase 1: Init
	addProgress(5, "初始化")

	// Load teaching tasks for the active semester (with Preload to avoid N+1 queries)
	var teachingTasks []models.TeachingTask
	taskQuery := s.db.Where("status = ?", "active").
		Preload("Course").Preload("Teacher").Preload("Classes.ClassGroup")
	if config.SemesterID > 0 {
		taskQuery = taskQuery.Where("semester_id = ?", config.SemesterID)
	}
	if err := taskQuery.Find(&teachingTasks).Error(); err != nil {
		log.Printf("[SCHED] EARLY-RETURN-1: load teachingTasks failed: %v", err)
		result.Error = "加载教学任务失败: " + err.Error()
		return result
	}
	log.Printf("[SCHED] teachingTasks loaded: count=%d", len(teachingTasks))

	// Filter by scope if needed (Course already preloaded)
	if config.Scope != "" && config.Scope != "全校所有院系" {
		var filtered []models.TeachingTask
		for _, tt := range teachingTasks {
			if tt.Course.Dept == config.Scope {
				filtered = append(filtered, tt)
			}
		}
		teachingTasks = filtered
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
	if err := s.db.Find(&classGroups).Error(); err != nil {
		log.Printf("[SCHED] EARLY-RETURN-5: load class groups failed: %v", err)
		result.Error = "加载班级分组失败: " + err.Error()
		return result
	}

	if len(classrooms) == 0 || len(teachers) == 0 {
		if isTimeOnly && len(teachers) > 0 {
			log.Printf("[SCHED] TIME_ONLY continue without classrooms: teachers=%d", len(teachers))
		} else {
			log.Printf("[SCHED] EARLY-RETURN-6: classrooms=%d teachers=%d", len(classrooms), len(teachers))
			result.Error = "缺少教室或教师资源"
			return result
		}
	}
	log.Printf("[SCHED] resources loaded: classrooms=%d teachers=%d classGroups=%d", len(classrooms), len(teachers), len(classGroups))

	// v0.6.0: Virtual classroom hack removed — SA solver now handles
	// TIME_ONLY mode without synthetic classrooms. The caller must ensure
	// the solver receives valid classroom data even in TIME_ONLY mode.
	// TODO(v0.6.0): verify SA solver works without virtual classrooms in TIME_ONLY mode.
	solverClassrooms := classrooms

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
	saConfig.TimeOnly = isTimeOnly
	saConfig.ConstraintWeights = config.ConstraintWeights

	addLog(fmt.Sprintf("模拟退火参数: 初始温度=%.1f, 冷却率=%.2f, 最长求解时间=%.0fs",
		saConfig.InitialTemp, saConfig.CoolingRate, saConfig.MaxTimeSeconds))

	// Run OR-Tools and SA in parallel if both available, pick the better result.
	sportsCourseIDs := s.buildSportsCourseIDs(teachingTasks)
	var saResult *SAResult

	if s.orchestrator != nil && s.orchestrator.IsORToolsAvailable() {
		// Both engines available — run in parallel
		addProgress(35, "OR-Tools + SA 并行求解")

		var ortoolsResult *SAResult
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			ortoolsResult = s.tryORTools(teachingTasks, teachers, solverClassrooms, classGroups, lockedSlots, config, sportsCourseIDs, addLog)
		}()

		go func() {
			defer wg.Done()
			solver := NewSASolver()
			result := solver.SolveMultiRun(
				teachingTasks, teachers, solverClassrooms, classGroups,
				lockedSlots, effectiveConstraints, config.SemesterID,
				saConfig, 3, nil, nil,
			)
			result.Entries = solver.PostOptimize(
				result.Entries, teachingTasks, teachers, solverClassrooms,
				lockedSlots, effectiveConstraints,
				max(5, len(result.Entries)/10),
				isTimeOnly,
			)
			saResult = result
		}()

		wg.Wait()

		// Pick the better result using ScoreSchedule as the single judge.
		// Do NOT compare internal scores (OR-Tools objective vs SA score)
		// because they use different scales.
		if ortoolsResult != nil && saResult != nil {
			scorer := NewScoringService()
			scoreClassrooms := classrooms
			if isTimeOnly {
				scoreClassrooms = nil
			}
			// Build a temporary scoring context for the comparison
			tmpExpectedSessions := 0
			for _, tt := range teachingTasks {
				th := tt.TotalHours
				if th <= 0 {
					th = tt.Course.Hours
				}
				plan := resolveSessionPlan(th, tt.StartWeek, tt.EndWeek, tt.MaxHoursPerWeek, tt.PreferredSpan)
				tmpExpectedSessions += plan.SessionsPerWeek()
			}
			tmpCtx := NewScoringContextWithExpected(effectiveConstraints, sportsCourseIDs, teachingTasks, tmpExpectedSessions).WithMode(mode).WithConstraintWeights(config.ConstraintWeights)
			ortoolsBreakdown := scorer.ScoreSchedule(ortoolsResult.Entries, teachers, scoreClassrooms, tmpCtx)
			saBreakdown := scorer.ScoreSchedule(saResult.Entries, teachers, scoreClassrooms, tmpCtx)
			addLog(fmt.Sprintf("并行求解完成: OR-Tools %.1f分 vs SA %.1f分 (ScoreSchedule)",
				ortoolsBreakdown.FinalTotal, saBreakdown.FinalTotal))
			if ortoolsBreakdown.FinalTotal > saBreakdown.FinalTotal {
				saResult = ortoolsResult
				saResult.Score = ortoolsBreakdown.FinalTotal
				addLog("选择 OR-Tools 结果")
			} else {
				saResult.Score = saBreakdown.FinalTotal
				addLog("选择 SA 结果")
			}
		} else if ortoolsResult != nil {
			saResult = ortoolsResult
		}
	} else {
		// Only SA available
		addProgress(45, "模拟退火优化")
		solver := NewSASolver()
		saResult = solver.SolveMultiRun(
			teachingTasks, teachers, solverClassrooms, classGroups,
			lockedSlots, effectiveConstraints, config.SemesterID,
			saConfig, 3, nil, nil,
		)
		saResult.Entries = solver.PostOptimize(
			saResult.Entries, teachingTasks, teachers, solverClassrooms,
			lockedSlots, effectiveConstraints,
			max(5, len(saResult.Entries)/10),
			isTimeOnly,
		)
		addLog(fmt.Sprintf("SA求解完成: %d次迭代, %.1fms",
			saResult.Iterations, float64(saResult.ElapsedMs)))
	}

	// v0.5.2: compute expected total sessions from teaching tasks
	// (single source of truth: resolveSessionPlan, same as SA solver uses).
	expectedTotalSessions := 0
	for _, tt := range teachingTasks {
		th := tt.TotalHours
		if th <= 0 {
			th = tt.Course.Hours
		}
		plan := resolveSessionPlan(th, tt.StartWeek, tt.EndWeek, tt.MaxHoursPerWeek, tt.PreferredSpan)
		expectedTotalSessions += plan.SessionsPerWeek()
	}

	// 构建评分上下文（复用至统一评分 + CreateSnapshot）
	// v0.5.5: 上下文携带 Mode，让快照/版本落库路径能记录当前是 FULL 还是 TIME_ONLY。
	scoringCtx := NewScoringContextWithExpected(effectiveConstraints, sportsCourseIDs, teachingTasks, expectedTotalSessions).WithMode(mode).WithConstraintWeights(config.ConstraintWeights)

	// v0.6.0: applySyntheticClassroomIDsForTimeOnly removed — IsVirtual field
	// no longer exists on ScheduleEntry. TIME_ONLY mode now relies on
	// TimeAssignment-only writes (no ScheduleEntry rows).

	// v0.5.2 Goal 1 — 统一评分语义：OR-Tools 和 SA 路径的最终 Score 都来自
	// ScoreSchedule.FinalTotal。OR-Tools 自算的 objective（output.Score）只作诊断。
	// 这保证三个体系（OR-Tools / SA / ScoreSchedule）对同一 entries 输出完全一致。
	{
		scorer := NewScoringService()
		scoreClassrooms := classrooms
		if isTimeOnly {
			scoreClassrooms = nil
		}
		breakdown := scorer.ScoreSchedule(saResult.Entries, teachers, scoreClassrooms, scoringCtx)
		saResult.Score = breakdown.FinalTotal
		result.Score = breakdown.FinalTotal
		result.ScoreDetail = &breakdown
	}

	// === Cross-run best result cache ===
	// If the new run does not beat the cached score, return the cached result.
	if s.bestCachedResult != nil && !ScoreGreater(result.Score, s.bestCachedScore) {
		cached := *s.bestCachedResult
		cached.Logs = append(cached.Logs, fmt.Sprintf("本次评分 %.1f ≤ 缓存最优 %.1f，返回缓存结果", result.Score, s.bestCachedScore))
		return &cached
	}

	// Post-solve analysis
	addProgress(70, "分析冲突")

	// Save result to database
	addProgress(85, "保存结果")
	err := s.db.Transaction(func(tx database.DB) error {
		// 1. Hard-delete old entries for the semester (TA + SE dual-table).
		// Must use Unscoped (hard delete) because unique indexes do not
		// include deleted_at — soft rows still occupy unique slots.
		// Transaction rollback still works with hard delete in SQLite.
		if err := tx.Unscoped().Where("semester_id = ?", config.SemesterID).Delete(&models.ScheduleEntry{}).Error(); err != nil {
			return fmt.Errorf("清空旧教室分配失败: %w", err)
		}
		if err := tx.Unscoped().Where("semester_id = ?", config.SemesterID).Delete(&models.TimeAssignment{}).Error(); err != nil {
			return fmt.Errorf("清空旧时间分配失败: %w", err)
		}

		// 2. Create ScheduleVersion (unified version record – v0.6.0)
		ver := &models.ScheduleVersion{
			SemesterID: config.SemesterID,
			Name:       fmt.Sprintf("v%s-%s", time.Now().Format("0102-1504"), string(mode)),
			Source:     models.TriggerAuto,
			Solver:     "simulated_annealing",
			Mode:       string(mode),
			EntryCount: len(saResult.Entries),
		}
		if err := tx.Create(ver).Error(); err != nil {
			return fmt.Errorf("创建版本失败: %w", err)
		}

		// 3. Write TimeAssignments + ScheduleEntries from solver result.
		// TODO(v0.6.0): The SA solver (sa_solver.go) produces []models.ScheduleEntry
		// but the new minimal model no longer carries time fields (DayOfWeek,
		// StartPeriod, Span, TeachingTaskID). After Task 7 migrates the SA solver
		// to output TimeAssignments separately, this loop will populate both tables.
		//
		// Final structure (compile-ready once SA solver outputs time data):
		//   for _, timeEntry := range saResult.TimeOutput {
		//       ta := models.TimeAssignment{
		//           SemesterID:        config.SemesterID,
		//           ScheduleVersionID: ver.ID,
		//           TeachingTaskID:    timeEntry.TeachingTaskID,
		//           DayOfWeek:         timeEntry.DayOfWeek,
		//           StartPeriod:       timeEntry.StartPeriod,
		//           Span:              timeEntry.Span,
		//       }
		//       tx.Create(&ta)
		//       if !isTimeOnly {
		//           se := models.ScheduleEntry{
		//               SemesterID:        config.SemesterID,
		//               ScheduleVersionID: ver.ID,
		//               TimeAssignmentID:  ta.ID,
		//               ClassroomID:       timeEntry.ClassroomID,
		//           }
		//           tx.Create(&se)
		//       }
		//   }

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
	if isTimeOnly {
		roomC = 0
	}
	result.TeacherConflicts = teacherC
	result.RoomConflicts = roomC
	result.ClassConflicts = classC
	result.Conflicts = teacherC + roomC + classC

	// Count unique teaching tasks that got at least one entry.
	// TODO(v0.6.0): ScheduleEntry no longer carries TeachingTaskID after Task 4.
	// Task placement tracking must use TimeAssignment records instead.
	// For now, count all entries (each entry = one session placement).
	placedCount := make(map[uint]int)
	taskSet := make(map[uint]bool)
	// TODO(v0.6.0): After SA solver migration, iterate TimeAssignments to
	// collect placed TeachingTaskIDs. ScheduleEntry no longer has TeachingTaskID.
	for _, e := range saResult.Entries {
		_ = e // TODO(v0.6.0): use e.TimeAssignment.TeachingTaskID when TA preload is wired
	}
	result.TasksScheduled = len(taskSet)
	if result.TotalCourses > 0 {
		result.Utilization = float64(result.TasksScheduled) / float64(result.TotalCourses)
	}

	// Build unplaced task list with root cause analysis
	if result.TasksScheduled < result.TotalCourses {
		// Build classroom lookup for root cause
		roomTypeCount := make(map[string]int)
		roomTypeCapacity := make(map[string]int) // max capacity per room type
		for _, c := range classrooms {
			roomTypeCount[c.RoomType]++
			if c.Capacity > roomTypeCapacity[c.RoomType] {
				roomTypeCapacity[c.RoomType] = c.Capacity
			}
		}

		for _, tt := range teachingTasks {
			if taskSet[tt.ID] {
				continue // placed, skip
			}
			// Compute expected sessions
			totalHours := tt.TotalHours
			if totalHours <= 0 {
				totalHours = tt.Course.Hours
			}
			plan := resolveSessionPlan(totalHours, tt.StartWeek, tt.EndWeek, tt.MaxHoursPerWeek, tt.PreferredSpan)
			expected := plan.SessionsPerWeek()

			// Determine required room type
			requiredRoom := ""
			if tt.RequiredRoomType != "" {
				requiredRoom = tt.RequiredRoomType
			} else if tt.Course.Category != "" {
				if rt, ok := models.CategoryRoomTypeMap[tt.Course.Category]; ok {
					requiredRoom = rt
				}
			}

			// Class name
			className := ""
			for i, cls := range tt.Classes {
				if i > 0 {
					className += ", "
				}
				if cls.ClassGroup.Name != "" {
					className += cls.ClassGroup.Name
				}
			}

			// Root cause analysis — evidence strength descending
			cause := ""
			if requiredRoom != "" {
				if roomTypeCount[requiredRoom] == 0 {
					cause = fmt.Sprintf("缺少%s类型教室", requiredRoom)
				} else {
					// Check capacity
					totalStudents := 0
					for _, cls := range tt.Classes {
						totalStudents += cls.ClassGroup.Students
					}
					if totalStudents > roomTypeCapacity[requiredRoom] {
						cause = fmt.Sprintf("%s类型教室容量不足（需%d人，最大%d人）", requiredRoom, totalStudents, roomTypeCapacity[requiredRoom])
					}
				}
			}
			if cause == "" {
				cause = "求解器未能找到可行时段（可能是教师时间冲突或锁定时段过多）"
			}

			courseName := tt.Course.Name
			if courseName == "" {
				courseName = fmt.Sprintf("课程#%d", tt.CourseID)
			}
			teacherName := tt.Teacher.Name
			if teacherName == "" {
				teacherName = fmt.Sprintf("教师#%d", tt.TeacherID)
			}

			result.UnplacedTasks = append(result.UnplacedTasks, UnplacedTask{
				TaskID:       tt.ID,
				CourseName:   courseName,
				TeacherName:  teacherName,
				ClassName:    className,
				RequiredRoom: requiredRoom,
				Placed:       placedCount[tt.ID],
				Expected:     expected,
				RootCause:    cause,
			})
		}
	}

	addLog(fmt.Sprintf("排课完成！任务 %d/%d（%d条），评分 %.1f/100，冲突 教师%d 教室%d 班级%d",
		result.TasksScheduled, result.TotalCourses, len(saResult.Entries), saResult.Score,
		result.TeacherConflicts, result.RoomConflicts, result.ClassConflicts))
	if result.TasksScheduled < result.TotalCourses {
		addLog(fmt.Sprintf("WARN 剩余 %d 个教学任务未能排入", result.TotalCourses-result.TasksScheduled))
	}

	// Auto-version after scheduling — unified version with full scoring + entries
	if s.versions != nil && len(saResult.Entries) > 0 {
		_, verErr := s.versions.CreateVersionFromSchedule(
			config.SemesterID, config.Scope, models.TriggerAuto, "simulated_annealing",
			saResult.Entries, teachers, classrooms,
			scoringCtx,
			saResult.ElapsedMs, result.Conflicts,
		)
		if verErr != nil {
			addLog("WARN 版本保存失败: " + verErr.Error())
		} else {
			addLog("版本已自动保存")
		}
	}

	log.Printf("[SCHED] RunScheduling DONE: totalCourses=%d scheduled=%d conflicts=%d logs=%d",
		result.TotalCourses, result.Scheduled, result.Conflicts, len(result.Logs))

	addProgress(100, "排课完成")

	// Cache final result for cross-run comparison (deep copy ScoreDetail pointer).
	{
		cached := *result
		if result.ScoreDetail != nil {
			sd := *result.ScoreDetail
			cached.ScoreDetail = &sd
		}
		s.bestCachedScore = result.Score
		s.bestCachedEntries = append([]models.ScheduleEntry(nil), saResult.Entries...)
		s.bestCachedResult = &cached
	}

	return result
}

func constraintsForMode(constraints []string, mode schedtypes.SchedulingMode) []string {
	set := constraints
	if len(set) == 0 {
		set = FullDefaultConstraints()
	}
	if !mode.IsTimeOnly() {
		out := make([]string, len(set))
		copy(out, set)
		return out
	}
	out := make([]string, 0, len(set))
	for _, c := range set {
		if c == "low_floor_preference" {
			continue
		}
		out = append(out, c)
	}
	return out
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
//
// TODO(v0.6.0): ScheduleEntry no longer carries time fields (DayOfWeek, StartPeriod,
// Span) or TeacherID / TeachingTaskID after Task 4. Conflict counting must be
// reworked to use TimeAssignment records instead. Until the SA solver migration
// (Task 7) provides TimeAssignments separately, this function returns zeros.
//
// Intended rewrite: accept []models.TimeAssignment + []models.ScheduleEntry
// and detect conflicts via (DayOfWeek, StartPeriod, Span) from TA, with
// ClassroomID from SE for room conflicts.
func (s *SchedulingService) countConflictsQuick(entries []models.ScheduleEntry) (teacher, room, class int) {
	// TODO(v0.6.0): Re-implement after SA solver migration provides TimeAssignments.
	//   - Teacher conflicts: teacher TA overlap on same (day, period)
	//   - Room conflicts: SE overlap on same (classroom, day, period) via TA link
	//   - Class conflicts: TeachingTask overlap on same (day, period) via TA link
	return 0, 0, 0
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

	// Map classrooms (include type + equipment for resource matching)
	for _, c := range classrooms {
		input.Classrooms = append(input.Classrooms, ORToolsRoom{
			ID: c.ID, Floor: c.Floor, Capacity: c.Capacity, Type: c.RoomType, Equipment: c.Equipment,
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
	isTimeOnly := config.Mode.IsTimeOnly()
	taskMap := make(map[uint]models.TeachingTask)
	for _, tt := range teachingTasks {
		classIDs := make([]uint, len(tt.Classes))
		for j, c := range tt.Classes {
			classIDs[j] = c.ClassGroupID
		}
		// v0.5.3: unified resource matching — Go computes allowed rooms, Python just reads the list
		// TIME_ONLY mode: skip room type filtering — all virtual classrooms are allowed.
		// Virtual classrooms are all NORMAL type, so filtering would exclude tasks
		// needing COMPUTER/LAB/GYM, causing them to be unplaced.
		var requiredRoomType string
		var allowedIDs []uint
		if isTimeOnly {
			// All virtual classrooms are allowed — no room type constraint
			allowedIDs = make([]uint, len(classrooms))
			for i, r := range classrooms {
				allowedIDs[i] = r.ID
			}
		} else {
			requiredRoomType = InferRoomType(tt, tt.Course)
			allowedRooms := AllowedRooms(tt, tt.Course, classrooms)
			allowedIDs = make([]uint, len(allowedRooms))
			for i, r := range allowedRooms {
				allowedIDs[i] = r.ID
			}
		}

		// TotalHours fallback: same as SA solver (sa_solver.go line 136-138)
		totalHours := tt.TotalHours
		if totalHours <= 0 {
			totalHours = tt.Course.Hours
		}

		// v0.5.1: Go-side authoritative session plan.
		// Python mirrors the same rules for legality but must accept these spans as-is.
		plan := resolveSessionPlan(totalHours, tt.StartWeek, tt.EndWeek, tt.MaxHoursPerWeek, tt.PreferredSpan)

		input.TeachingTasks = append(input.TeachingTasks, ORToolsTask{
			ID:               tt.ID,
			TeacherID:        tt.TeacherID,
			CourseID:         tt.CourseID,
			ClassIDs:         classIDs,
			SessionsPerWeek:  plan.SessionsPerWeek(),
			SessionSpans:     append([]int{}, plan.Spans...),
			TotalHours:       totalHours,
			MaxHoursPerWeek:  tt.MaxHoursPerWeek,
			PreferredSpan:    tt.PreferredSpan,
			RequiredRoomType: requiredRoomType,
			AllowedRoomIDs:   allowedIDs,
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
	// Override with frontend-provided weights (if any), already clamped
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
		_, ok := taskMap[e.TaskID]
		if !ok {
			continue
		}

		// Validate entry fields from OR-Tools — skip invalid entries
		if e.DayOfWeek < 0 || e.DayOfWeek > 6 ||
			e.StartPeriod < 0 || e.StartPeriod > 10 ||
			e.Span < 1 || e.Span > 3 ||
			e.StartPeriod+e.Span > 11 {
			log(fmt.Sprintf("OR-Tools 返回非法 entry，已跳过: task=%d day=%d start=%d span=%d",
				e.TaskID, e.DayOfWeek, e.StartPeriod, e.Span))
			continue
		}
		// TODO(v0.6.0): OR-Tools output carries time fields (DayOfWeek, StartPeriod,
		// Span, TeacherID) that no longer fit in the minimal ScheduleEntry model.
		// After Tasks 6-7, the OR-Tools path will produce TimeAssignments separately.
		// For now, create ScheduleEntry with only the fields that exist on the new model.
		entry := models.ScheduleEntry{
			SemesterID:  config.SemesterID,
			ClassroomID: e.ClassroomID,
			// ScheduleVersionID and TimeAssignmentID are 0 — they will be
			// assigned during persistence when TA+SE dual-table writes are
			// wired up (see TODO in persistence section).
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
			// Use resolveSessionPlan (Go-side authoritative source) for expected sessions
			totalHours := tt.TotalHours
			if totalHours <= 0 {
				totalHours = tt.Course.Hours
			}
			plan := resolveSessionPlan(totalHours, tt.StartWeek, tt.EndWeek, tt.MaxHoursPerWeek, tt.PreferredSpan)
			expectedSessions := plan.SessionsPerWeek()
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
