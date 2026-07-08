package services

import (
	"encoding/json"
	"fmt"
	"scheduling-system/database"
	"scheduling-system/models"
	"time"
)

type SchedulingService struct {
	db        database.DB
	snapshots *SnapshotService
}

func NewSchedulingService(db database.DB, snapshots *SnapshotService) *SchedulingService {
	return &SchedulingService{db: db, snapshots: snapshots}
}

type SchedulingConfig struct {
	Scope       string           `json:"scope"`
	Semester    string           `json:"semester"`
	Strategy    string           `json:"strategy"`
	Iterations  int              `json:"iterations"`
	TimeLimit   int              `json:"timeLimit"` // max solve time in seconds, default 60
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
	DayOfWeek   models.DayOfWeek `json:"dayOfWeek"`
	StartPeriod models.Period    `json:"startPeriod"`
	Span        int              `json:"span"`
}

func (s *SchedulingService) RunScheduling(config SchedulingConfig) *SchedulingResult {
	result := &SchedulingResult{Logs: []string{}}
	log := func(msg string) {
		result.Logs = append(result.Logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
	}

	// Load courses
	var courses []models.Course
	scopeCode := reverseDeptMap[config.Scope]
	scopeSQL := config.Scope
	if scopeCode != "" {
		scopeSQL = scopeCode
	}
	if config.Scope == "全校所有院系" {
		if err := s.db.Find(&courses).Error(); err != nil {
			result.Error = "加载课程失败: " + err.Error()
			return result
		}
	} else {
		if err := s.db.Where("dept = ?", scopeSQL).Find(&courses).Error(); err != nil {
			result.Error = "加载课程失败: " + err.Error()
			return result
		}
	}
	result.TotalCourses = len(courses)
	if len(courses) == 0 {
		result.Error = "没有找到课程"
		return result
	}
	log(fmt.Sprintf("排课引擎启动（模拟退火），共 %d 门课程待排", len(courses)))

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

	// Run SA solver
	solver := NewSASolver()
	saResult := solver.Solve(
		courses, teachers, classrooms, classGroups,
		lockedSlots, config.Constraints, config.Semester,
		saConfig,
		nil, // cancelCh, nil = no interrupt for now
		func(iter, total int, currentScore, bestScore, temp float64) {
			if iter%1000 == 0 || iter == total {
				log(fmt.Sprintf("SA进度: %d次迭代, 温度=%.2f, 最优分=%.1f", iter, temp, bestScore))
			}
		},
	)

	log(fmt.Sprintf("SA求解完成: %d次迭代, %.1fms, 最优分=%.1f",
		saResult.Iterations, float64(saResult.ElapsedMs), saResult.Score))

		// Save result to database
		err := s.db.Transaction(func(tx database.DB) error {
			// Hard-delete old entries (Unscoped prevents soft-delete which would
			// leave rows occupying the unique index and cause conflicts on re-insert)
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
	finalBreakdown := scorer.ScoreSchedule(saResult.Entries, teachers, classrooms, config.Constraints)
	result.ScoreDetail = &finalBreakdown

	log(fmt.Sprintf("排课完成！已排 %d/%d 门，利用率 %.1f%%，评分 %.1f/100",
		len(saResult.Entries), result.TotalCourses, result.Utilization*100, saResult.Score))
	if len(saResult.Entries) < result.TotalCourses {
		log(fmt.Sprintf("WARN 剩余 %d 门课程需手动调整", result.TotalCourses-len(saResult.Entries)))
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
	
		// Class group conflicts
		classSlots := make(map[string]bool)
		for _, e := range entries {
			if e.ClassGroupID == nil {
				continue
			}
			for p := e.StartPeriod; p < e.StartPeriod+models.Period(e.Span); p++ {
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
