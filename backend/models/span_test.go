package models

import "testing"

// TestIsSpanLegalMorningAfternoon covers Spec §八 Case 3:
// morning/afternoon must reject span=3.
func TestIsSpanLegalMorningAfternoon(t *testing.T) {
	cases := []struct {
		start Period
		span  int
		want  bool
		desc  string
	}{
		// Morning: only span=2 at start 0 or 2.
		{0, 2, true, "morning start=0 span=2 (第1-2节)"},
		{2, 2, true, "morning start=2 span=2 (第3-4节)"},
		{1, 2, false, "morning start=1 span=2 (misaligned)"},
		{0, 3, false, "morning start=0 span=3 (rejected, Case 3)"},
		{2, 3, false, "morning start=2 span=3 (rejected, Case 3)"},
		{0, 1, false, "morning start=0 span=1 (single not allowed here)"},

		// Afternoon: only span=2 at start 4 or 6.
		{4, 2, true, "afternoon start=4 span=2 (第5-6节)"},
		{6, 2, true, "afternoon start=6 span=2 (第7-8节)"},
		{5, 2, false, "afternoon start=5 span=2 (misaligned)"},
		{4, 3, false, "afternoon start=4 span=3 (rejected, Case 3)"},
		{6, 3, false, "afternoon start=6 span=3 (rejected, Case 3)"},

		// Evening: span 1/2/3 all legal within block.
		{8, 1, true, "evening start=8 span=1 (第9节)"},
		{8, 2, true, "evening start=8 span=2 (第9-10节)"},
		{8, 3, true, "evening start=8 span=3 (第9-10-11节, Case 2)"},
		{9, 1, false, "evening start=9 span=1 (第10节单独不存在)"},
		{9, 2, false, "evening start=9 span=2 (第10-11节不存在)"},
		{9, 3, false, "evening start=9 span=3 (overruns 11-12)"},
		{10, 1, false, "evening start=10 span=1 (第11节单独不存在)"},
		{10, 2, false, "evening start=10 span=2 (overruns 12)"},

		// Boundary crossings.
		{3, 2, false, "start=3 span=2 crosses morning/afternoon boundary"},
		{7, 2, false, "start=7 span=2 crosses afternoon/evening boundary"},

		// Reserved: span=4 always false.
		{0, 4, false, "span=4 reserved for future"},
		{8, 4, false, "span=4 reserved for future"},

		// Illegal spans.
		{0, 0, false, "span=0 illegal"},
		{0, -1, false, "negative span illegal"},
	}
	for _, c := range cases {
		got := IsSpanLegal(c.start, c.span)
		if got != c.want {
			t.Errorf("%s: IsSpanLegal(%d,%d) = %v, want %v", c.desc, c.start, c.span, got, c.want)
		}
	}
}

// TestValidStartsForSpanForSpan2 documents the span=2 start set.
// HBUT规则: 上午1-2/3-4, 下午5-6/7-8, 晚上9-10. 第10-11节不存在.
func TestValidStartsForSpanForSpan2(t *testing.T) {
	got := ValidStartsForSpan(2)
	want := []Period{0, 2, 4, 6, 8}
	if len(got) != len(want) {
		t.Fatalf("ValidStartsForSpan(2) length = %d, want %d (got=%v)", len(got), len(want), got)
	}
	for i, p := range want {
		if got[i] != p {
			t.Errorf("ValidStartsForSpan(2)[%d] = %d, want %d", i, got[i], p)
		}
	}
}

func TestValidStartsForSpanEvening(t *testing.T) {
	// span=1 only at evening start=8 (第9节, 极少数选修).
	got := ValidStartsForSpan(1)
	want := []Period{8}
	if len(got) != len(want) {
		t.Fatalf("ValidStartsForSpan(1) = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ValidStartsForSpan(1)[%d] mismatch", i)
		}
	}

	// span=3 only at evening start=8.
	got3 := ValidStartsForSpan(3)
	if len(got3) != 1 || got3[0] != 8 {
		t.Errorf("ValidStartsForSpan(3) = %v, want [8]", got3)
	}
}

func TestBlockOfStart(t *testing.T) {
	for start := Period(0); start <= 3; start++ {
		if BlockOfStart(start) != Morning {
			t.Errorf("BlockOfStart(%d) not Morning", start)
		}
	}
	for start := Period(4); start <= 7; start++ {
		if BlockOfStart(start) != Afternoon {
			t.Errorf("BlockOfStart(%d) not Afternoon", start)
		}
	}
	for start := Period(8); start <= 10; start++ {
		if BlockOfStart(start) != Evening {
			t.Errorf("BlockOfStart(%d) not Evening", start)
		}
	}
	if BlockOfStart(11) != BlockUnknown {
		t.Errorf("BlockOfStart(11) should be Unknown")
	}
}
