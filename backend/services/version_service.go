package services

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"scheduling-system/backend/database"
	"scheduling-system/backend/models"
)

// DefaultMaxVersions is the maximum number of versions retained per semester.
// When a new version is created and the count exceeds this limit, the oldest
// version(s) are deleted (FIFO sliding window).
const DefaultMaxVersions = 50

// VersionService manages ScheduleVersion and ScheduleVersionEntry records.
type VersionService struct {
	db database.DB
}

func NewVersionService(db database.DB) *VersionService {
	return &VersionService{db: db}
}

// defaultModeForSemester 从最近一次版本继承 Mode，用作新 version 的默认值。
// 场景：ManualAdjust 版本没有独立的模式选择器，与所在学期最近一次排课模式对齐；
// 若没有版本（新学期首次调整），回退到 FULL_SCHEDULING（Stable Core 行为）。
func (s *VersionService) defaultModeForSemester(semesterID uint) string {
	var ver models.ScheduleVersion
	err := s.db.
		Where("semester_id = ? AND mode <> ''", semesterID).
		Order("created_at DESC").
		First(&ver).Error()
	if err != nil || ver.Mode == "" {
		return "FULL_SCHEDULING"
	}
	return ver.Mode
}

// CreateVersionFromSchedule creates a version with full scoring details and
// teacher breakdowns from a scheduling result. This is the unified replacement
// for both the old SnapshotService.CreateSnapshot and VersionService.CreateVersion.
//
// It computes ScoreSchedule internally, stores entries, scoring fields, and
// per-teacher VersionDetail records in a single transaction.
// TODO(v0.6.0): Adapt CreateVersionFromSchedule for TimeAssignment+ScheduleEntry split.
func (s *VersionService) CreateVersionFromSchedule(
	semesterID uint, dept, trigger, solver string,
	entries []models.ScheduleEntry,
	teachers []models.Teacher,
	classrooms []models.Classroom,
	scoringCtx ScoringContext,
	solveTimeMs int64,
	conflictCount int,
) (*models.ScheduleVersion, error) {
	_ = dept
	_ = entries
	_ = teachers
	_ = classrooms
	_ = scoringCtx
	_ = solveTimeMs
	version := &models.ScheduleVersion{
		SemesterID:        semesterID,
		Name:              models.DefaultVersionName(trigger, time.Now()),
		Source:            triggerToSource(trigger),
		Solver:            solver,
		Mode:              "FULL_SCHEDULING",
		HardPassed:        conflictCount == 0,
		TeacherConflicts:  0,
		RoomConflicts:     0,
		ClassConflicts:    0,
		EntryCount:        0,
		Score:             0,
	}
	if err := s.db.Create(version).Error(); err != nil {
		return nil, fmt.Errorf("version: create failed: %w", err)
	}
	if err := s.enforceRetention(s.db, semesterID); err != nil {
		return nil, err
	}
	return version, nil
}

// triggerToSource maps a trigger string to a VersionSource constant.
func triggerToSource(trigger string) string {
	switch trigger {
	case models.TriggerAuto:
		return models.VersionSourceAutoGenerate
	case models.TriggerManual:
		return models.VersionSourceManualAdjust
	case models.TriggerImport:
		return models.VersionSourceImport
	case models.TriggerRestore:
		return models.VersionSourceRestore
	case models.TriggerCopy:
		return models.VersionSourceCopy
	default:
		return models.VersionSourceAutoGenerate
	}
}

// CreateManualVersion saves the current schedule as a new version with
// source = ManualAdjust. It loads the live schedule entries from the
// database, computes the current ScoreSchedule score, and persists a
// new ScheduleVersion (with entries) in a single transaction.
// TODO(v0.6.0): Adapt CreateManualVersion for TimeAssignment+ScheduleEntry split.
func (s *VersionService) CreateManualVersion(semesterID uint, name string) (*models.ScheduleVersion, error) {
	_ = semesterID
	_ = name
	return nil, fmt.Errorf("v0.6.0 migration in progress: CreateManualVersion temporarily disabled")
}
func (s *VersionService) ListVersions(semesterID uint) ([]models.ScheduleVersion, error) {
	var versions []models.ScheduleVersion
	if err := s.db.Where("semester_id = ?", semesterID).
		Order("created_at DESC").Find(&versions).Error(); err != nil {
		return nil, fmt.Errorf("version: 查询版本列表失败: %w", err)
	}
	return versions, nil
}

// GetVersion retrieves a single version with its entries preloaded.
func (s *VersionService) GetVersion(id uint) (*models.ScheduleVersion, error) {
	var version models.ScheduleVersion
	if err := s.db.Preload("Entries").First(&version, id).Error(); err != nil {
		return nil, fmt.Errorf("version: 未找到 ID=%d: %w", id, err)
	}
	return &version, nil
}

// GetVersionWithDetails returns a single version with its Details preloaded.
// Used by the report page to display per-teacher scoring breakdown.
func (s *VersionService) GetVersionWithDetails(id uint) (*models.ScheduleVersion, error) {
	var version models.ScheduleVersion
	if err := s.db.Preload("Details").First(&version, id).Error(); err != nil {
		return nil, fmt.Errorf("version: 未找到 ID=%d: %w", id, err)
	}
	if version.Name == "" {
		version.Name = version.DisplayName()
	}
	return &version, nil
}

// DeleteVersion removes a version and its entries/details by ID in a single transaction.
func (s *VersionService) DeleteVersion(id uint) error {
	return s.db.Transaction(func(tx database.DB) error {
		if err := tx.Where("version_id = ?", id).Delete(&models.ScheduleVersionEntry{}).Error(); err != nil {
			return fmt.Errorf("version: 删除版本条目失败: %w", err)
		}
		if err := tx.Where("version_id = ?", id).Delete(&models.VersionDetail{}).Error(); err != nil {
			return fmt.Errorf("version: 删除版本明细失败: %w", err)
		}
		if err := tx.Delete(&models.ScheduleVersion{}, id).Error(); err != nil {
			return fmt.Errorf("version: 删除版本失败: %w", err)
		}
		return nil
	})
}

// RenameVersion updates the Name field of an existing version.
func (s *VersionService) RenameVersion(id uint, newName string) error {
	if newName == "" {
		return fmt.Errorf("版本名称不能为空")
	}
	if len(newName) > 100 {
		return fmt.Errorf("版本名称不能超过100个字符")
	}
	var ver models.ScheduleVersion
	if err := s.db.First(&ver, id).Error(); err != nil {
		return err
	}
	ver.Name = newName
	return s.db.Save(&ver).Error()
}

// CreateManualReport generates a version from the current schedule in the database,
// computing full scoring details. This replaces the old SnapshotService.CreateManualSnapshot.
// TODO(v0.6.0): Adapt CreateManualReport for TimeAssignment+ScheduleEntry split.
func (s *VersionService) CreateManualReport(semesterID uint) (*models.ScheduleVersion, error) {
	_ = semesterID
	return nil, fmt.Errorf("v0.6.0 migration in progress: CreateManualReport temporarily disabled")
}

// AnalyzeTeacherWorkload loads the current schedule entries and computes per-teacher workload analysis.
// Pure post-hoc analysis — does not affect scoring or solver behaviour.
// TODO(v0.6.0): Adapt AnalyzeTeacherWorkload for TimeAssignment+ScheduleEntry split.
func (s *VersionService) AnalyzeTeacherWorkload(semesterID uint) ([]TeacherWorkloadInfo, error) {
	_ = semesterID
	return nil, nil
}
func teacherDetailPenalty(d models.VersionDetail) float64 {
	penalty := d.EarlyPenalty + d.LatePenalty
	if d.DaysActual > d.DaysTarget {
		penalty += float64(d.DaysActual - d.DaysTarget)
	}
	penalty += d.AvgFloor * 0.1
	return penalty
}


type VersionCompareResult struct {
	A             *models.ScheduleVersion `json:"a"`
	B             *models.ScheduleVersion `json:"b"`
	ScoreDelta    float64                 `json:"scoreDelta"`
	ConflictDelta int                     `json:"conflictDelta"`
	EntryDelta    int                     `json:"entryDelta"`
	TeacherDiffs  []TeacherVersionDiff    `json:"teacherDiffs"`
	EntryDiffs    []EntryDiff             `json:"entryDiffs"`
}

type TeacherVersionDiff struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	EntryDelta    int     `json:"entryDelta"`
	EarlyDelta    float64 `json:"earlyDelta"`
	LateDelta     float64 `json:"lateDelta"`
	DaysActualA   int     `json:"daysActualA"`
	DaysActualB   int     `json:"daysActualB"`
	DaysTarget    int     `json:"daysTarget"`
	AvgFloorDelta float64 `json:"avgFloorDelta"`
	Status        string  `json:"status"`
}

type EntryDiff struct {
	Type     string `json:"type"`
	Course   string `json:"course"`
	Teacher  string `json:"teacher"`
	TaskID   uint   `json:"taskId,omitempty"`
	OldDay   int    `json:"oldDay,omitempty"`
	OldStart int    `json:"oldStart,omitempty"`
	NewDay   int    `json:"newDay,omitempty"`
	NewStart int    `json:"newStart,omitempty"`
}

// CompareVersions returns a structured diff between two versions (A=baseline, B=target).
// Includes teacher-level diffs and entry-level diffs (moved/added/removed).
func (s *VersionService) CompareVersions(aID, bID uint) (*VersionCompareResult, error) {
	a, err := s.GetVersionWithDetails(aID)
	if err != nil {
		return nil, fmt.Errorf("加载版本A失败: %w", err)
	}
	b, err := s.GetVersionWithDetails(bID)
	if err != nil {
		return nil, fmt.Errorf("加载版本B失败: %w", err)
	}

	res := &VersionCompareResult{
		A:             a,
		B:             b,
		ScoreDelta:    b.FinalScore - a.FinalScore,
		EntryDelta:    b.EntryCount - a.EntryCount,
		ConflictDelta: 0,
	}
	switch {
	case a.HardPassed && !b.HardPassed:
		res.ConflictDelta = -1
	case !a.HardPassed && b.HardPassed:
		res.ConflictDelta = 1
	}

	// Teacher-level diffs
	aMap := map[string]models.VersionDetail{}
	for _, d := range a.Details {
		if d.EntityType == "teacher" {
			aMap[d.EntityCode] = d
		}
	}
	bMap := map[string]models.VersionDetail{}
	for _, d := range b.Details {
		if d.EntityType == "teacher" {
			bMap[d.EntityCode] = d
		}
	}

	seen := map[string]bool{}
	var codes []string
	for c := range aMap {
		if !seen[c] {
			seen[c] = true
			codes = append(codes, c)
		}
	}
	for c := range bMap {
		if !seen[c] {
			seen[c] = true
			codes = append(codes, c)
		}
	}
	sort.Strings(codes)

	for _, code := range codes {
		ad, aok := aMap[code]
		bd, bok := bMap[code]
		diff := TeacherVersionDiff{Code: code}
		if aok {
			diff.Name = ad.EntityName
			diff.DaysActualA = ad.DaysActual
			diff.DaysTarget = ad.DaysTarget
			diff.EarlyDelta -= ad.EarlyPenalty
			diff.LateDelta -= ad.LatePenalty
			diff.AvgFloorDelta -= ad.AvgFloor
			diff.EntryDelta -= ad.EntryCount
		}
		if bok {
			diff.Name = bd.EntityName
			diff.DaysActualB = bd.DaysActual
			diff.DaysTarget = bd.DaysTarget
			diff.EarlyDelta += bd.EarlyPenalty
			diff.LateDelta += bd.LatePenalty
			diff.AvgFloorDelta += bd.AvgFloor
			diff.EntryDelta += bd.EntryCount
		}
		switch {
		case !aok:
			diff.Status = "added"
		case !bok:
			diff.Status = "removed"
		case teacherDetailPenalty(bd) < teacherDetailPenalty(ad):
			diff.Status = "improved"
		case teacherDetailPenalty(bd) > teacherDetailPenalty(ad):
			diff.Status = "regressed"
		default:
			diff.Status = "unchanged"
		}
		res.TeacherDiffs = append(res.TeacherDiffs, diff)
	}

	// Entry-level diffs: compare by TeachingTaskID
	aEntries, errA := s.GetVersion(aID)
	bEntries, errB := s.GetVersion(bID)
	if errA != nil || errB != nil {
		// Entry diffs are optional — log but don't fail the whole comparison
		if errA != nil {
			log.Printf("[CompareVersions] 加载版本A条目失败: %v", errA)
		}
		if errB != nil {
			log.Printf("[CompareVersions] 加载版本B条目失败: %v", errB)
		}
	}
	if aEntries != nil && bEntries != nil {
		// Load course and teacher name maps for readable diffs
		courseMap := make(map[uint]string)
		teacherMap := make(map[uint]string)
		var courses []models.Course
		if s.db.Find(&courses).Error() == nil {
			for _, c := range courses {
				courseMap[c.ID] = c.Name
			}
		}
		var teachers []models.Teacher
		if s.db.Find(&teachers).Error() == nil {
			for _, t := range teachers {
				teacherMap[t.ID] = t.Name
			}
		}
		res.EntryDiffs = computeEntryDiffs(aEntries.Entries, bEntries.Entries, courseMap, teacherMap)
	}

	return res, nil
}

// computeEntryDiffs compares two sets of version entries by TeachingTaskID
// and returns moved/added/removed diffs with human-readable names.
func computeEntryDiffs(aEntries, bEntries []models.ScheduleVersionEntry, courseMap, teacherMap map[uint]string) []EntryDiff {
	aByTask := make(map[uint]models.ScheduleVersionEntry)
	for _, e := range aEntries {
		if e.TeachingTaskID != nil {
			aByTask[*e.TeachingTaskID] = e
		}
	}
	bByTask := make(map[uint]models.ScheduleVersionEntry)
	for _, e := range bEntries {
		if e.TeachingTaskID != nil {
			bByTask[*e.TeachingTaskID] = e
		}
	}

	courseName := func(id uint) string {
		if n, ok := courseMap[id]; ok && n != "" {
			return n
		}
		return fmt.Sprintf("课程#%d", id)
	}
	teacherName := func(id uint) string {
		if n, ok := teacherMap[id]; ok && n != "" {
			return n
		}
		return fmt.Sprintf("教师#%d", id)
	}

	var diffs []EntryDiff

	// Find moved and removed
	for taskID, ae := range aByTask {
		be, exists := bByTask[taskID]
		if !exists {
			diffs = append(diffs, EntryDiff{
				Type:     "removed",
				TaskID:   taskID,
				Course:   courseName(ae.CourseID),
				Teacher:  teacherName(ae.TeacherID),
				OldDay:   ae.DayOfWeek,
				OldStart: ae.StartPeriod,
			})
		} else if ae.DayOfWeek != be.DayOfWeek || ae.StartPeriod != be.StartPeriod {
			diffs = append(diffs, EntryDiff{
				Type:     "moved",
				TaskID:   taskID,
				Course:   courseName(ae.CourseID),
				Teacher:  teacherName(ae.TeacherID),
				OldDay:   ae.DayOfWeek,
				OldStart: ae.StartPeriod,
				NewDay:   be.DayOfWeek,
				NewStart: be.StartPeriod,
			})
		}
	}

	// Find added
	for taskID, be := range bByTask {
		if _, exists := aByTask[taskID]; !exists {
			diffs = append(diffs, EntryDiff{
				Type:     "added",
				TaskID:   taskID,
				Course:   courseName(be.CourseID),
				Teacher:  teacherName(be.TeacherID),
				NewDay:   be.DayOfWeek,
				NewStart: be.StartPeriod,
			})
		}
	}

	return diffs
}

// RestoreVersion restores a historical version as the current schedule.
// It loads the version entries, replaces the current schedule for the semester,
// and creates a new version with source=Restore to record the action.
// TODO(v0.6.0): Adapt RestoreVersion for TimeAssignment+ScheduleEntry split.
func (s *VersionService) RestoreVersion(versionID uint) error {
	_ = versionID
	return fmt.Errorf("v0.6.0 migration in progress: RestoreVersion temporarily disabled")
}

// ClearSemesterVersions removes every ScheduleVersion (and its entries)
// belonging to the given semester in a single transaction. Only affects
// the specified semester; other semesters, courses, teachers, classrooms,
// snapshots, and live schedule entries are untouched.
//
// Returns the number of versions deleted. If the semester has no versions,
// returns 0 with a nil error.
func (s *VersionService) ClearSemesterVersions(semesterID uint) (int64, error) {
	var deleted int64
	err := s.db.Transaction(func(tx database.DB) error {
		// Collect version IDs for this semester so we can delete entries
		// via IN (...) without relying on Raw SQL (not on our DB interface).
		var versions []models.ScheduleVersion
		if err := tx.Where("semester_id = ?", semesterID).
			Find(&versions).Error(); err != nil {
			return fmt.Errorf("version: 查询版本失败: %w", err)
		}

		if len(versions) == 0 {
			return nil
		}

		ids := make([]uint, 0, len(versions))
		for _, v := range versions {
			ids = append(ids, v.ID)
		}

		// Delete entries and details first (FK safety, matches DeleteVersion order).
		if err := tx.Where("version_id IN ?", ids).
			Delete(&models.ScheduleVersionEntry{}).Error(); err != nil {
			return fmt.Errorf("version: 删除版本条目失败: %w", err)
		}
		if err := tx.Where("version_id IN ?", ids).
			Delete(&models.VersionDetail{}).Error(); err != nil {
			return fmt.Errorf("version: 删除版本明细失败: %w", err)
		}
		if err := tx.Where("semester_id = ?", semesterID).
			Delete(&models.ScheduleVersion{}).Error(); err != nil {
			return fmt.Errorf("version: 删除版本失败: %w", err)
		}

		deleted = int64(len(versions))
		return nil
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

// buildScoringContext assembles all data needed to evaluate a full semester's
// schedule. It loads teachers, classrooms, and teaching tasks, identifies
// sports courses, reads the constraint list from the latest snapshot, and
// returns a ready-to-use ScoringContext together with the teacher and
// classroom slices required by ScoreSchedule.
func (s *VersionService) buildScoringContext(semesterID uint) (ScoringContext, []models.Teacher, []models.Classroom, error) {
	var teachers []models.Teacher
	if err := s.db.Find(&teachers).Error(); err != nil {
		return ScoringContext{}, nil, nil, fmt.Errorf("version: 加载教师数据失败: %w", err)
	}

	var classrooms []models.Classroom
	if err := s.db.Find(&classrooms).Error(); err != nil {
		return ScoringContext{}, nil, nil, fmt.Errorf("version: 加载教室数据失败: %w", err)
	}

	// Load teaching tasks for sports course detection and student_fatigue
	var teachingTasks []models.TeachingTask
	if semesterID > 0 {
		if err := s.db.Where("semester_id = ?", semesterID).
			Preload("Course").Preload("Teacher").
			Preload("Classes.ClassGroup").
			Find(&teachingTasks).Error(); err != nil {
			return ScoringContext{}, nil, nil, fmt.Errorf("version: 加载教学任务失败: %w", err)
		}
	}

	// Build sports course IDs
	sportsCourseIDs := make(map[uint]bool)
	for _, tt := range teachingTasks {
		if tt.CourseID > 0 && strings.Contains(tt.Course.Name, "体育") {
			sportsCourseIDs[tt.CourseID] = true
		}
	}

	// Read constraints from latest version; fall back to all constraints
	constraints := FullDefaultConstraints()
	var ver models.ScheduleVersion
	if err := s.db.Where("semester_id = ?", semesterID).
		Order("created_at DESC").First(&ver).Error(); err == nil && ver.EnabledConstraints != "" {
		var parsed []string
		if json.Unmarshal([]byte(ver.EnabledConstraints), &parsed) == nil && len(parsed) > 0 {
			constraints = parsed
		}
	}

	ctx := NewScoringContext(constraints, sportsCourseIDs, teachingTasks)
	return ctx, teachers, classrooms, nil
}

// enforceRetention ensures the per-semester version count does not exceed
// DefaultMaxVersions by deleting the oldest version(s) if necessary.
// Accepts a DB argument so it can be called inside a transaction.
func (s *VersionService) enforceRetention(tx database.DB, semesterID uint) error {
	var count int64
	if err := tx.Model(&models.ScheduleVersion{}).
		Where("semester_id = ?", semesterID).
		Count(&count).Error(); err != nil {
		return fmt.Errorf("version: 统计版本数量失败: %w", err)
	}

	if count <= DefaultMaxVersions {
		return nil
	}

	excess := int(count) - DefaultMaxVersions

	// Load all versions for this semester ordered oldest-first
	var allVersions []models.ScheduleVersion
	if err := tx.Where("semester_id = ?", semesterID).
		Order("created_at ASC").Find(&allVersions).Error(); err != nil {
		return fmt.Errorf("version: 查询待清理版本失败: %w", err)
	}

	// Take only the first `excess` IDs
	oldestIDs := make([]uint, 0, excess)
	for i := 0; i < excess && i < len(allVersions); i++ {
		oldestIDs = append(oldestIDs, allVersions[i].ID)
	}

	for _, vid := range oldestIDs {
		if err := tx.Where("version_id = ?", vid).Delete(&models.ScheduleVersionEntry{}).Error(); err != nil {
			return fmt.Errorf("version: 清理版本条目失败: %w", err)
		}
		if err := tx.Where("version_id = ?", vid).Delete(&models.VersionDetail{}).Error(); err != nil {
			return fmt.Errorf("version: 清理版本明细失败: %w", err)
		}
		if err := tx.Delete(&models.ScheduleVersion{}, vid).Error(); err != nil {
			return fmt.Errorf("version: 清理旧版本失败: %w", err)
		}
	}
	return nil
}

// marshalCategoryMaxes serializes per-category maxes to JSON string for version storage.
func marshalCategoryMaxes(m map[string]float64) string {
	if len(m) == 0 {
		return ""
	}
	b, _ := json.Marshal(m)
	return string(b)
}
