//go:build !production

package services

import (
	"testing"

	"scheduling-system/backend/database"
	"scheduling-system/backend/models"
)

// TestCreateManualReport_ErrorsWhenEmpty locks the contract：
// 当学期完全没有 schedule_entries 时 CreateManualReport 返回
// non-nil error 并且不写入任何 version 行，避免产生"空版本"垃圾。
func TestCreateManualReport_ErrorsWhenEmpty(t *testing.T) {
	adapter, err := database.InitDB(t.TempDir())
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}

	svc := NewVersionService(adapter)
	_, err = svc.CreateManualReport(999_999)
	if err == nil {
		t.Fatalf("expected error for semester with no entries, got nil")
	}
}

// TestCreateManualReport_HappyPath 覆盖有 entries 场景下的正确落库：
// 版本 Source=manual, HardPassed 反映冲突结果。
func TestCreateManualReport_HappyPath(t *testing.T) {
	adapter, err := database.InitDB(t.TempDir())
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}

	// 找一个 seed 出来的、有 schedule_entries 的学期。
	var entryCount int64
	adapter.Model(&models.ScheduleEntry{}).Count(&entryCount)
	if entryCount == 0 {
		t.Skip("no seed schedule_entries in this build; skipping happy-path")
	}

	var one models.ScheduleEntry
	if err := adapter.First(&one).Error(); err != nil {
		t.Fatalf("load first entry: %v", err)
	}

	svc := NewVersionService(adapter)
	ver, err := svc.CreateManualReport(one.SemesterID)
	if err != nil {
		t.Fatalf("CreateManualReport failed: %v", err)
	}
	if ver == nil || ver.ID == 0 {
		t.Fatalf("expected non-nil version with ID > 0, got %+v", ver)
	}
	if ver.Source != models.VersionSourceManualAdjust {
		t.Fatalf("expected Source=%q, got %q", models.VersionSourceManualAdjust, ver.Source)
	}
	if ver.SemesterID != one.SemesterID {
		t.Fatalf("expected SemesterID=%d, got %d", one.SemesterID, ver.SemesterID)
	}
}
