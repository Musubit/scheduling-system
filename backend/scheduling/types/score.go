package types

// ScoreBucket is one dimension's contribution to a ScoreBreakdown. Value
// is the achieved score in this bucket; Max is the ceiling
// (PerBucketMax); Details carries per-sub-constraint scores for
// diagnostics.
type ScoreBucket struct {
	Value   float64            `json:"value"`
	Max     float64            `json:"max"`
	Details map[string]float64 `json:"details"`
}

// ScoreBreakdown is the four-bucket score of a scheduling result. A nil
// bucket field means the dimension is Disabled for this run (INV-S1);
// TIME_ONLY runs always have Resource == nil. The mapping between
// EnabledDimensions and non-nil bucket fields is one-to-one (INV-S2).
type ScoreBreakdown struct {
	Time     *ScoreBucket `json:"time,omitempty"`
	Teacher  *ScoreBucket `json:"teacher,omitempty"`
	Student  *ScoreBucket `json:"student,omitempty"`
	Resource *ScoreBucket `json:"resource,omitempty"`

	EnabledDimensions []string `json:"enabledDimensions"`
	PerBucketMax      float64  `json:"perBucketMax"`

	PlacedSessions   int     `json:"placedSessions"`
	ExpectedSessions int     `json:"expectedSessions"`
	Completeness     float64 `json:"completeness"`

	Total      float64 `json:"total"`
	FinalTotal float64 `json:"finalTotal"`
}

// TimeScoreDetail is the partial score TimeScheduler computes over the
// three time-family buckets. Orchestrator merges this with
// ResourceScoreDetail (if any) to produce the final ScoreBreakdown.
type TimeScoreDetail struct {
	Time    *ScoreBucket
	Teacher *ScoreBucket
	Student *ScoreBucket
}

// ResourceScoreDetail is the partial score RoomScheduler computes.
// TIME_ONLY runs skip RoomScheduler entirely; this type is not
// constructed in that mode.
type ResourceScoreDetail struct {
	Resource *ScoreBucket
}
