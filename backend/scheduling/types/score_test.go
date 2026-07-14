package types

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestScoreBucket_FieldsAccessible(t *testing.T) {
	b := ScoreBucket{Value: 22.5, Max: 25.0, Details: map[string]float64{"course_dispersed": 8}}
	if b.Value != 22.5 || b.Details["course_dispersed"] != 8 {
		t.Fatalf("field roundtrip broken: %+v", b)
	}
}

func TestScoreBreakdown_NilBucketsAreDisabled(t *testing.T) {
	// INV-S1: A nil bucket = Disabled. Not zero-valued, not sentinel.
	sb := ScoreBreakdown{
		Time:              &ScoreBucket{Value: 20, Max: 25},
		Teacher:           &ScoreBucket{Value: 22, Max: 25},
		Student:           &ScoreBucket{Value: 18, Max: 25},
		Resource:          nil, // TIME_ONLY case
		EnabledDimensions: []string{"time", "teacher", "student"},
		PerBucketMax:      25.0,
	}
	if sb.Resource != nil {
		t.Error("Resource bucket should be nil in TIME_ONLY setup")
	}
	if sb.Time == nil {
		t.Error("Time bucket should not be nil")
	}
}

func TestScoreBreakdown_JSONOmitsNilBuckets(t *testing.T) {
	// INV-F3 (frontend-facing): Disabled buckets are absent from JSON,
	// not serialized as null or 0. Ensures the *ScoreBucket field type
	// carries `omitempty` semantics naturally.
	sb := ScoreBreakdown{
		Time:              &ScoreBucket{Value: 20, Max: 25, Details: map[string]float64{"course_dispersed": 8}},
		Teacher:           &ScoreBucket{Value: 22, Max: 25, Details: map[string]float64{}},
		Student:           &ScoreBucket{Value: 18, Max: 25, Details: map[string]float64{}},
		Resource:          nil,
		EnabledDimensions: []string{"time", "teacher", "student"},
	}
	raw, err := json.Marshal(sb)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	got := string(raw)
	if strings.Contains(got, "\"resource\"") {
		t.Errorf("JSON must omit nil Resource bucket, got: %s", got)
	}
	if !strings.Contains(got, "\"time\"") {
		t.Errorf("JSON must include Time bucket, got: %s", got)
	}
}

func TestScoreDetails_FieldsAccessible(t *testing.T) {
	tsd := TimeScoreDetail{
		Time:    &ScoreBucket{Value: 20},
		Teacher: &ScoreBucket{Value: 22},
		Student: &ScoreBucket{Value: 18},
	}
	rsd := ResourceScoreDetail{Resource: &ScoreBucket{Value: 24}}
	if tsd.Time.Value != 20 || rsd.Resource.Value != 24 {
		t.Fatalf("detail types broken: %+v %+v", tsd, rsd)
	}
}

func TestScoreBreakdown_FieldTypes(t *testing.T) {
	// Guard: all four bucket fields must be *ScoreBucket, not ScoreBucket.
	// Prevents an accidental value-typed field that would break INV-S1.
	typ := reflect.TypeOf(ScoreBreakdown{})
	wantPtr := reflect.PointerTo(reflect.TypeOf(ScoreBucket{}))
	for _, name := range []string{"Time", "Teacher", "Student", "Resource"} {
		f, ok := typ.FieldByName(name)
		if !ok {
			t.Errorf("ScoreBreakdown missing field %q", name)
			continue
		}
		if f.Type != wantPtr {
			t.Errorf("ScoreBreakdown.%s type = %v, want %v (INV-S1)", name, f.Type, wantPtr)
		}
	}
}
