package services

import "testing"

// TestResolveSessionPlanCase1 covers Spec §八 Case 1:
// 周学时=2 → Span=2 (single 2-period session).
func TestResolveSessionPlanCase1(t *testing.T) {
	plan := resolveSessionPlan(32, 1, 16, 0, 0) // 32 hours / 16 weeks = 2 hrs/wk
	if plan.SessionsPerWeek() != 1 {
		t.Fatalf("Case1: sessionsPerWeek = %d, want 1", plan.SessionsPerWeek())
	}
	if plan.Spans[0] != 2 {
		t.Errorf("Case1: span = %d, want 2", plan.Spans[0])
	}
}

// TestResolveSessionPlanCase2 covers Spec §八 Case 2:
// 周学时=3 → 3-period session (Evening).
func TestResolveSessionPlanCase2(t *testing.T) {
	// 48 hours / 16 weeks = 3 hrs/wk
	plan := resolveSessionPlan(48, 1, 16, 0, 0)
	if plan.SessionsPerWeek() != 1 {
		t.Fatalf("Case2: sessionsPerWeek = %d, want 1 (single 3-period session)", plan.SessionsPerWeek())
	}
	if plan.Spans[0] != 3 {
		t.Errorf("Case2: span = %d, want 3", plan.Spans[0])
	}
}

// TestResolveSessionPlanFourHours covers Goal 3 table: 4hrs/wk → [2,2].
func TestResolveSessionPlanFourHours(t *testing.T) {
	plan := resolveSessionPlan(64, 1, 16, 0, 0) // 4 hrs/wk
	if plan.SessionsPerWeek() != 2 {
		t.Fatalf("4hrs: sessionsPerWeek = %d, want 2", plan.SessionsPerWeek())
	}
	if plan.Spans[0] != 2 || plan.Spans[1] != 2 {
		t.Errorf("4hrs: spans = %v, want [2,2]", plan.Spans)
	}
}

// TestResolveSessionPlanSingleHour covers Goal 3 table: 1hr/wk → [1].
func TestResolveSessionPlanSingleHour(t *testing.T) {
	plan := resolveSessionPlan(16, 1, 16, 0, 0) // 1 hr/wk
	if plan.SessionsPerWeek() != 1 {
		t.Fatalf("1hr: sessionsPerWeek = %d, want 1", plan.SessionsPerWeek())
	}
	if plan.Spans[0] != 1 {
		t.Errorf("1hr: span = %d, want 1", plan.Spans[0])
	}
}

// TestResolveSessionPlanPreferredOverride verifies PreferredSpan takes precedence
// while still respecting the hour budget.
func TestResolveSessionPlanPreferredOverride(t *testing.T) {
	// 4 hrs/wk but user prefers span=2 → same result [2,2].
	plan := resolveSessionPlan(64, 1, 16, 0, 2)
	if plan.SessionsPerWeek() != 2 || plan.Spans[0] != 2 {
		t.Errorf("prefer=2 for 4hrs: got %v, want [2,2]", plan.Spans)
	}
	// 6 hrs/wk with prefer=3 → [3,3].
	plan = resolveSessionPlan(96, 1, 16, 0, 3)
	if plan.SessionsPerWeek() != 2 || plan.Spans[0] != 3 || plan.Spans[1] != 3 {
		t.Errorf("prefer=3 for 6hrs: got %v, want [3,3]", plan.Spans)
	}
}

// TestResolveSessionPlanZeroHoursFallback ensures degenerate inputs don't panic.
func TestResolveSessionPlanZeroHoursFallback(t *testing.T) {
	plan := resolveSessionPlan(0, 1, 16, 0, 0)
	if plan.SessionsPerWeek() != 1 || plan.Spans[0] != 2 {
		t.Errorf("0 hours: got %v, want [2]", plan.Spans)
	}
}

// TestResolveSessionPlanMaxHoursCap verifies MaxHoursPerWeek clamping.
func TestResolveSessionPlanMaxHoursCap(t *testing.T) {
	// 6 hrs/wk course but capped at 4 → should behave like 4 hrs/wk = [2,2].
	plan := resolveSessionPlan(96, 1, 16, 4, 0)
	if plan.SessionsPerWeek() != 2 || plan.Spans[0] != 2 {
		t.Errorf("cap=4 on 6hrs: got %v, want [2,2]", plan.Spans)
	}
}

// TestResolveSessionPlanLegacyEquivalence — Spec §八 Case 4: courses whose weekly
// hours are 2/4/6/8 must produce the same sessions-per-week as the pre-v0.5.1
// formula ceil(weeklyHours/2), so v0.4 snapshots re-score identically.
func TestResolveSessionPlanLegacyEquivalence(t *testing.T) {
	cases := []struct {
		weeklyHours int
		wantN       int
	}{
		{2, 1}, // was ceil(2/2)=1
		{4, 2}, // was ceil(4/2)=2
		{6, 3}, // was ceil(6/2)=3
		{8, 4}, // was ceil(8/2)=4
	}
	for _, c := range cases {
		hours := c.weeklyHours * 16
		plan := resolveSessionPlan(hours, 1, 16, 0, 0)
		if plan.SessionsPerWeek() != c.wantN {
			t.Errorf("weeklyHours=%d: sessionsPerWeek = %d, want %d",
				c.weeklyHours, plan.SessionsPerWeek(), c.wantN)
		}
		for i, s := range plan.Spans {
			if s != 2 {
				t.Errorf("weeklyHours=%d: Spans[%d] = %d, want 2 (legacy equivalence)",
					c.weeklyHours, i, s)
			}
		}
	}
}
