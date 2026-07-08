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

	database.DB.Preload("Course").Preload("Teacher").Preload("Classroom").
		Where("semester = ?", semester).Find(&entries)

	conflictID := uint(1)

	// 1. Teacher time conflicts
	teacherSlots := make(map[string][]models.ScheduleEntry)
	for _, e := range entries {
		key := fmt.Sprintf("%d-%d", e.TeacherID, e.DayOfWeek)
		for _, e2 := range teacherSlots[key] {
			if periodsOverlap(e.StartPeriod, e.Span, e2.StartPeriod, e2.Span) {
				conflicts = append(conflicts, Conflict{
					ID:          conflictID,
					Type:        "教师时间冲突",
					Description: fmt.Sprintf("%s老师在周%d同时段有两门课程", e.Teacher.Name, e.DayOfWeek+1),
					Detail:      fmt.Sprintf("%s(%s) vs %s(%s)", e.Course.Name, e.Classroom.Name, e2.Course.Name, e2.Classroom.Name),
					Severity:    "error",
					Info: []ConflictInfoItem{
						{Label: "冲突教师", Value: fmt.Sprintf("%s %s", e.Teacher.Name, e.Teacher.Title)},
						{Label: "所属院系", Value: e.Teacher.Dept},
						{Label: "冲突课程 A", Value: fmt.Sprintf("%s - 周%d %d-%d节 - %s", e.Course.Name, e.DayOfWeek+1, e.StartPeriod+1, e.StartPeriod+e.Span, e.Classroom.Name)},
						{Label: "冲突课程 B", Value: fmt.Sprintf("%s - 周%d %d-%d节 - %s", e2.Course.Name, e2.DayOfWeek+1, e2.StartPeriod+1, e2.StartPeriod+e2.Span, e2.Classroom.Name)},
						{Label: "冲突类型", Value: "同一教师同一时段"},
					},
					Solutions: []string{
						fmt.Sprintf("将 %s 调至其他时段", e2.Course.Name),
						fmt.Sprintf("更换 %s 授课教师", e2.Course.Name),
						"合并为合班授课",
					},
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
			if periodsOverlap(e.StartPeriod, e.Span, e2.StartPeriod, e2.Span) {
				conflicts = append(conflicts, Conflict{
					ID:          conflictID,
					Type:        "教室占用冲突",
					Description: fmt.Sprintf("%s教室在周%d第%d-%d节被两门课程同时占用", e.Classroom.Name, e.DayOfWeek+1, e.StartPeriod+1, e.StartPeriod+e.Span),
					Detail:      fmt.Sprintf("%s vs %s", e.Course.Name, e2.Course.Name),
					Severity:    "warning",
					Info: []ConflictInfoItem{
						{Label: "冲突教室", Value: fmt.Sprintf("%s (%s)", e.Classroom.Name, e.Classroom.Type)},
						{Label: "冲突时段", Value: fmt.Sprintf("周%d %d-%d节", e.DayOfWeek+1, e.StartPeriod+1, e.StartPeriod+e.Span)},
						{Label: "课程 A", Value: fmt.Sprintf("%s - %s %s", e.Course.Name, e.Teacher.Name, e.Teacher.Title)},
						{Label: "课程 B", Value: fmt.Sprintf("%s - %s %s", e2.Course.Name, e2.Teacher.Name, e2.Teacher.Title)},
						{Label: "冲突类型", Value: "同一教室同一时段"},
					},
					Solutions: []string{
						fmt.Sprintf("将 %s 调至其他教室", e.Course.Name),
						fmt.Sprintf("将 %s 调至其他教室", e2.Course.Name),
						"调整时段错开安排",
					},
				})
				conflictID++
			}
		}
		roomSlots[key] = append(roomSlots[key], e)
	}

	return conflicts
}

func periodsOverlap(start1, span1, start2, span2 int) bool {
	end1 := start1 + span1
	end2 := start2 + span2
	return start1 < end2 && start2 < end1
}
