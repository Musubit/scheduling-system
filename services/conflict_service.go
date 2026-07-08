package services

import (
	"fmt"
	"scheduling-system/database"
	"scheduling-system/models"
)

// ConflictService handles conflict detection and resolution.
type ConflictService struct{}

func NewConflictService() *ConflictService {
	return &ConflictService{}
}

// Conflict represents a scheduling conflict.
type Conflict struct {
	ID          uint                   `json:"id"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Severity    string                 `json:"severity"`
	Detail      string                 `json:"detail"`
	Info        []ConflictInfoItem     `json:"info"`
	Solutions   []string               `json:"solutions"`
}

// ConflictInfoItem is a key-value detail item.
type ConflictInfoItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// DetectConflicts finds all scheduling conflicts for a given semester.
func (c *ConflictService) DetectConflicts(semester string) []Conflict {
	var conflicts []Conflict
	var entries []models.ScheduleEntry

	if err := database.DB.Preload("Course").Preload("Teacher").Preload("Classroom").Preload("ClassGroup").
		Where("semester = ?", semester).Find(&entries).Error; err != nil {
		return conflicts
	}

	conflictID := uint(1)

	// Helper: parse weeks string like "1-16" into start/end
	parseWeeks := func(w string) (int, int) {
		var s, e int
		fmt.Sscanf(w, "%d-%d", &s, &e)
		if s <= 0 { s = 1 }
		if e <= 0 { e = 16 }
		return s, e
	}
	weeksOverlap := func(w1, w2 string) bool {
		s1, e1 := parseWeeks(w1)
		s2, e2 := parseWeeks(w2)
		return s1 <= e2 && s2 <= e1
	}

	// 1. Teacher time conflicts
	teacherSlots := make(map[string][]models.ScheduleEntry)
	for _, e := range entries {
		key := fmt.Sprintf("%d-%d", e.TeacherID, e.DayOfWeek)
		for _, e2 := range teacherSlots[key] {
			if periodsOverlap(e.StartPeriod, e.Span, e2.StartPeriod, e2.Span) && weeksOverlap(e.Weeks, e2.Weeks) {
				conflicts = append(conflicts, Conflict{
					ID: conflictID, Type: "教师时间冲突",
					Description: fmt.Sprintf("%s老师在周%d同时段有两门课程", e.Teacher.Name, e.DayOfWeek+1),
					Detail:      fmt.Sprintf("%s(%s) vs %s(%s)", e.Course.Name, e.Classroom.Name, e2.Course.Name, e2.Classroom.Name),
					Severity:    "error",
					Info: []ConflictInfoItem{
						{Label: "冲突教师", Value: fmt.Sprintf("%s %s", e.Teacher.Name, e.Teacher.Title)},
						{Label: "冲突课程 A", Value: fmt.Sprintf("%s - 周%d %d-%d节", e.Course.Name, e.DayOfWeek+1, e.StartPeriod+1, e.StartPeriod+e.Span)},
						{Label: "冲突课程 B", Value: fmt.Sprintf("%s - 周%d %d-%d节", e2.Course.Name, e2.DayOfWeek+1, e2.StartPeriod+1, e2.StartPeriod+e2.Span)},
					},
					Solutions: []string{fmt.Sprintf("将%s调至其他时段", e2.Course.Name), fmt.Sprintf("更换%s授课教师", e2.Course.Name)},
				})
				conflictID++
			}
		}
		teacherSlots[key] = append(teacherSlots[key], e)
	}

	// 2. Room double-booking
	roomSlots := make(map[string][]models.ScheduleEntry)
	for _, e := range entries {
		key := fmt.Sprintf("%d-%d", e.ClassroomID, e.DayOfWeek)
		for _, e2 := range roomSlots[key] {
			if periodsOverlap(e.StartPeriod, e.Span, e2.StartPeriod, e2.Span) && weeksOverlap(e.Weeks, e2.Weeks) {
				conflicts = append(conflicts, Conflict{
					ID: conflictID, Type: "教室占用冲突",
					Description: fmt.Sprintf("%s教室在周%d被两门课程同时占用", e.Classroom.Name, e.DayOfWeek+1),
					Detail:      fmt.Sprintf("%s vs %s", e.Course.Name, e2.Course.Name),
					Severity:    "warning",
					Info: []ConflictInfoItem{
						{Label: "冲突教室", Value: e.Classroom.Name},
						{Label: "课程 A", Value: e.Course.Name},
						{Label: "课程 B", Value: e2.Course.Name},
					},
					Solutions: []string{fmt.Sprintf("将%s调至其他教室", e.Course.Name)},
				})
				conflictID++
			}
		}
		roomSlots[key] = append(roomSlots[key], e)
	}

	// 3. Class group time conflicts (同一班级同一时间不能上两门课)
	classSlots := make(map[string][]models.ScheduleEntry)
	for _, e := range entries {
		if e.ClassGroupID == nil {
			continue // skip entries without class group
		}
		key := fmt.Sprintf("%d-%d", *e.ClassGroupID, e.DayOfWeek)
		for _, e2 := range classSlots[key] {
			if periodsOverlap(e.StartPeriod, e.Span, e2.StartPeriod, e2.Span) && weeksOverlap(e.Weeks, e2.Weeks) {
				conflicts = append(conflicts, Conflict{
					ID: conflictID, Type: "班级时间冲突",
					Description: fmt.Sprintf("%s在周%d同时段有两门课程", e.ClassGroup.Name, e.DayOfWeek+1),
					Detail:      fmt.Sprintf("%s vs %s", e.Course.Name, e2.Course.Name),
					Severity:    "error",
					Info: []ConflictInfoItem{
						{Label: "冲突班级", Value: e.ClassGroup.Name},
						{Label: "课程 A", Value: fmt.Sprintf("%s - 周%d %d-%d节", e.Course.Name, e.DayOfWeek+1, e.StartPeriod+1, e.StartPeriod+e.Span)},
						{Label: "课程 B", Value: fmt.Sprintf("%s - 周%d %d-%d节", e2.Course.Name, e2.DayOfWeek+1, e2.StartPeriod+1, e2.StartPeriod+e2.Span)},
					},
					Solutions: []string{fmt.Sprintf("将%s调至其他时段", e2.Course.Name), fmt.Sprintf("将%s调整至另一班级时间", e2.Course.Name)},
				})
				conflictID++
			}
		}
		classSlots[key] = append(classSlots[key], e)
	}

	return conflicts
}

func periodsOverlap(start1, span1, start2, span2 int) bool {
	end1 := start1 + span1
	end2 := start2 + span2
	return start1 < end2 && start2 < end1
}
