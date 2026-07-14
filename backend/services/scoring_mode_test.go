//go:build !production

package services

import (
	"testing"

	"scheduling-system/backend/models"
	schedtypes "scheduling-system/backend/scheduling/types"
)

// TestScoreSchedule_TimeOnlyDisablesResourceDimension 覆盖 M2 结构化维度门控 (INV-S1/S2/I8):
// TIME_ONLY 下即使 EnabledConstraints 包含 low_floor_preference，也必须结构化禁用
// —— 而不是依赖调用方传 nil classrooms 短路。这是"禁用 = 缺失,不是 0"的核心保证。
func TestScoreSchedule_TimeOnlyDisablesResourceDimension(t *testing.T) {
	// 制造一个 low_floor 应该 penalize 的场景：把课排在高楼层且教师偏好低楼层
	teachers := []models.Teacher{
		{Model: models.Teacher{}.Model, PreferLowFloor: true},
	}
	teachers[0].ID = 1
	classrooms := []models.Classroom{
		{Model: models.Classroom{}.Model, Floor: 10}, // 明显高楼层
	}
	classrooms[0].ID = 100
	entries := []models.ScheduleEntry{
		{TeacherID: 1, ClassroomID: 100, DayOfWeek: 1, StartPeriod: 2, Span: 2},
	}

	// FULL 模式 —— 结构化门控允许 resource 维度
	ctxFull := NewScoringContext(
		[]string{"low_floor_preference"}, nil, nil,
	).WithMode(schedtypes.ModeFullScheduling)

	scorer := NewScoringService()
	fullBreakdown := scorer.ScoreSchedule(entries, teachers, classrooms, ctxFull)
	if fullBreakdown.EnabledCategoryCount != 1 {
		t.Fatalf("FULL: expected 1 enabled category, got %d", fullBreakdown.EnabledCategoryCount)
	}
	// LowFloorPref 应该被评估 —— 具体分值不重要,只要 EnabledCategoryCount 反映"评估了"

	// TIME_ONLY 模式 —— 结构化门控禁用 resource 维度
	ctxTimeOnly := NewScoringContext(
		[]string{"low_floor_preference"}, nil, nil,
	).WithMode(schedtypes.ModeTimeOnlyScheduling)

	timeOnlyBreakdown := scorer.ScoreSchedule(entries, teachers, classrooms, ctxTimeOnly)
	if timeOnlyBreakdown.LowFloorPref != 0 {
		t.Fatalf("TIME_ONLY: LowFloorPref must be 0 (disabled), got %v", timeOnlyBreakdown.LowFloorPref)
	}
	if timeOnlyBreakdown.EnabledCategoryCount != 0 {
		t.Fatalf("TIME_ONLY: expected 0 enabled categories (low_floor disabled by dim gate), got %d",
			timeOnlyBreakdown.EnabledCategoryCount)
	}
}

// TestScoringContext_WithMode 覆盖新的 v3 ScoringContext.Mode 字段和 EffectiveMode 兜底。
func TestScoringContext_WithMode(t *testing.T) {
	ctx := NewScoringContext(nil, nil, nil)
	if ctx.Version != 3 {
		t.Fatalf("expected Version=3, got %d", ctx.Version)
	}
	if ctx.EffectiveMode() != schedtypes.ModeFullScheduling {
		t.Fatalf("default Mode should be FULL, got %q", ctx.EffectiveMode())
	}

	ctx2 := ctx.WithMode(schedtypes.ModeTimeOnlyScheduling)
	if ctx2.Mode != schedtypes.ModeTimeOnlyScheduling {
		t.Fatalf("WithMode did not set Mode")
	}
	if ctx.Mode == schedtypes.ModeTimeOnlyScheduling {
		t.Fatalf("WithMode should return copy, not mutate receiver")
	}

	// 非法 mode 兜底到 FULL
	ctx3 := ctx.WithMode(schedtypes.SchedulingMode("BOGUS"))
	if ctx3.Mode != schedtypes.ModeFullScheduling {
		t.Fatalf("invalid mode should fall back to FULL, got %q", ctx3.Mode)
	}
}

// TestScoreSchedule_BucketsNilnessMatchesMode 覆盖 spec §2.7 冻结的 4 桶结构 + INV-S2：
// EnabledDimensions ⇔ 非 nil bucket 集合(互斥且穷尽)。
func TestScoreSchedule_BucketsNilnessMatchesMode(t *testing.T) {
	scorer := NewScoringService()

	// FULL: 4 桶全 non-nil
	full := scorer.ScoreSchedule(nil, nil, nil,
		NewScoringContext(FullDefaultConstraints(), nil, nil).WithMode(schedtypes.ModeFullScheduling))
	if full.Buckets == nil {
		t.Fatal("FULL: Buckets must not be nil")
	}
	if full.Buckets.Time == nil || full.Buckets.Teacher == nil ||
		full.Buckets.Student == nil || full.Buckets.Resource == nil {
		t.Fatalf("FULL: all 4 buckets must be non-nil, got %+v", full.Buckets)
	}
	if len(full.EnabledDimensions) != 4 {
		t.Fatalf("FULL: expected 4 EnabledDimensions, got %v", full.EnabledDimensions)
	}

	// TIME_ONLY: Resource == nil, others non-nil (INV-S1)
	timeOnly := scorer.ScoreSchedule(nil, nil, nil,
		NewScoringContext(FullDefaultConstraints(), nil, nil).WithMode(schedtypes.ModeTimeOnlyScheduling))
	if timeOnly.Buckets == nil {
		t.Fatal("TIME_ONLY: Buckets must not be nil")
	}
	if timeOnly.Buckets.Resource != nil {
		t.Fatalf("TIME_ONLY: Resource bucket must be nil (Disabled), got %+v", timeOnly.Buckets.Resource)
	}
	if timeOnly.Buckets.Time == nil || timeOnly.Buckets.Teacher == nil || timeOnly.Buckets.Student == nil {
		t.Fatalf("TIME_ONLY: Time/Teacher/Student must remain non-nil, got %+v", timeOnly.Buckets)
	}
	// EnabledDimensions ≡ non-nil buckets (INV-S2)
	if len(timeOnly.EnabledDimensions) != 3 {
		t.Fatalf("TIME_ONLY: expected 3 EnabledDimensions, got %v", timeOnly.EnabledDimensions)
	}
	for _, d := range timeOnly.EnabledDimensions {
		if d == "resource" {
			t.Fatal("TIME_ONLY: EnabledDimensions must not contain 'resource'")
		}
	}
}

// TestStoredConfig_ModeRoundTrip 覆盖 v1/v2 兼容 —— 老 StoredConfig JSON 里没有 mode,
// 反序列化后 EffectiveMode 兜底 FULL_SCHEDULING;不能因为字段缺失就报错。
func TestStoredConfig_ModeRoundTrip(t *testing.T) {
	legacyV2 := `{"version":2,"enabledConstraints":["teacher_preference"]}`
	cfg, err := UnmarshalStoredConfig(legacyV2)
	if err != nil {
		t.Fatalf("unmarshal v2 legacy: %v", err)
	}
	rebuilt := ScoringContextFromStored(cfg, nil, nil)
	if rebuilt.EffectiveMode() != schedtypes.ModeFullScheduling {
		t.Fatalf("legacy v2 config should read back as FULL, got %q", rebuilt.EffectiveMode())
	}
	if rebuilt.Version != 2 {
		t.Fatalf("Version should stay 2 (not silently upgraded), got %d", rebuilt.Version)
	}

	// v3 with TIME_ONLY round-trip
	ctx := NewScoringContext([]string{"teacher_preference"}, nil, nil).WithMode(schedtypes.ModeTimeOnlyScheduling)
	raw := ctx.MarshalStored()
	cfg2, err := UnmarshalStoredConfig(raw)
	if err != nil {
		t.Fatalf("unmarshal v3: %v", err)
	}
	if cfg2.Mode != schedtypes.ModeTimeOnlyScheduling {
		t.Fatalf("v3 Mode should round-trip TIME_ONLY, got %q", cfg2.Mode)
	}
}
