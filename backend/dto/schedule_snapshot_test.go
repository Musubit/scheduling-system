package dto

import (
	"encoding/json"
	"testing"

	schedtypes "scheduling-system/backend/scheduling/types"
)

// TestScheduleSnapshotDTO_JSONRoundTrip 覆盖 DTO 的 JSON 往返稳定性:
// 序列化 → 反序列化后字段一致,pointer 语义保留(TIME_ONLY 快照的 ClassroomID
// 保持 nil,不会退化为 0)。
func TestScheduleSnapshotDTO_JSONRoundTrip(t *testing.T) {
	room := uint(42)
	roomName := "A101"
	dto := ScheduleSnapshotDTO{
		SchemaVersion:     SchemaVersionV055,
		SemesterID:        1,
		SemesterName:      "2026-2027第一学期",
		Mode:              schedtypes.ModeFullScheduling,
		ScheduleVersionID: 100,
		Assignments: []ScheduledAssignmentDTO{
			{
				TeachingTaskID: 10,
				TeacherID:      2,
				TeacherName:    "张老师",
				CourseID:       3,
				CourseName:     "高数",
				ClassGroupIDs:  []uint{5, 6},
				DayOfWeek:      1,
				StartPeriod:    2,
				Span:           2,
				WeekRange:      "1-16",
				ClassroomID:    &room,
				ClassroomName:  &roomName,
			},
		},
	}

	raw, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var back ScheduleSnapshotDTO
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if back.SchemaVersion != SchemaVersionV055 {
		t.Fatalf("SchemaVersion round-trip: got %q", back.SchemaVersion)
	}
	if back.Mode != schedtypes.ModeFullScheduling {
		t.Fatalf("Mode round-trip: got %q", back.Mode)
	}
	if len(back.Assignments) != 1 {
		t.Fatalf("Assignments length: got %d", len(back.Assignments))
	}
	if back.Assignments[0].ClassroomID == nil || *back.Assignments[0].ClassroomID != 42 {
		t.Fatalf("ClassroomID pointer round-trip failed: %+v", back.Assignments[0].ClassroomID)
	}
}

// TestScheduleSnapshotDTO_TimeOnlyOmitsClassroom 覆盖 TIME_ONLY 语义:
// ClassroomID 为 nil 时 JSON 不出现该字段(omitempty),前端读到 undefined
// 就知道"没有分教室"。这是 INV-F3 的物理保证。
func TestScheduleSnapshotDTO_TimeOnlyOmitsClassroom(t *testing.T) {
	dto := ScheduleSnapshotDTO{
		SchemaVersion: SchemaVersionV055,
		Mode:          schedtypes.ModeTimeOnlyScheduling,
		Assignments: []ScheduledAssignmentDTO{
			{
				TeachingTaskID: 10,
				DayOfWeek:      1,
				StartPeriod:    2,
				Span:           2,
				// ClassroomID / ClassroomName / ClassroomFloor / ClassroomType 全部 nil
			},
		},
	}

	raw, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	jsonStr := string(raw)
	for _, forbidden := range []string{`"classroomId"`, `"classroomName"`, `"classroomFloor"`, `"classroomType"`} {
		if contains(jsonStr, forbidden) {
			t.Fatalf("TIME_ONLY DTO leaked %s into JSON: %s", forbidden, jsonStr)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
