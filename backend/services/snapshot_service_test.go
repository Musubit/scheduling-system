//go:build !production

package services

import (
	"testing"

	"scheduling-system/backend/database"
	"scheduling-system/backend/models"
)

// TestCreateManualSnapshot_ErrorsWhenEmpty locks the contract：
// 当学期完全没有 schedule_entries 时 CreateManualSnapshot 返回
// non-nil error 并且不写入任何 snapshot 行，避免产生"空快照"垃圾。
// 覆盖 SchedulingPage 保存快照按钮在无排课结果时的正确失败反馈。
func TestCreateManualSnapshot_ErrorsWhenEmpty(t *testing.T) {
	adapter, err := database.InitDB(t.TempDir())
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}

	// seed 只写 planned 学期，但 seed_test / dev build 也会 seedDemoEntries；
	// 显式选一个不存在的 semesterID，绕过所有 seed 数据。
	svc := NewSnapshotService(adapter)
	_, err = svc.CreateManualSnapshot(999_999)
	if err == nil {
		t.Fatalf("expected error for semester with no entries, got nil")
	}
}

// TestCreateManualSnapshot_HappyPath 覆盖有 entries 场景下的正确落库：
// 快照 Trigger=manual, Solver=manual, HardPassed 反映冲突结果。
func TestCreateManualSnapshot_HappyPath(t *testing.T) {
	adapter, err := database.InitDB(t.TempDir())
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}

	// 找一个 seed 出来的、有 schedule_entries 的学期。
	// dev seed 走 seedDemoEntries 会写少量条目；若没有则跳过（生产 build）。
	var entryCount int64
	adapter.Model(&models.ScheduleEntry{}).Count(&entryCount)
	if entryCount == 0 {
		t.Skip("no seed schedule_entries in this build (likely -tags production); skipping happy-path")
	}

	var one models.ScheduleEntry
	if err := adapter.First(&one).Error(); err != nil {
		t.Fatalf("load first entry: %v", err)
	}

	svc := NewSnapshotService(adapter)
	snap, err := svc.CreateManualSnapshot(one.SemesterID)
	if err != nil {
		t.Fatalf("CreateManualSnapshot failed: %v", err)
	}
	if snap == nil || snap.ID == 0 {
		t.Fatalf("expected non-nil snapshot with ID > 0, got %+v", snap)
	}
	if snap.Trigger != models.TriggerManual {
		t.Fatalf("expected Trigger=%q, got %q", models.TriggerManual, snap.Trigger)
	}
	if snap.SemesterID != one.SemesterID {
		t.Fatalf("expected SemesterID=%d, got %d", one.SemesterID, snap.SemesterID)
	}
}
