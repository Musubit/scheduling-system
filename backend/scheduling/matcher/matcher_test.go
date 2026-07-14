package matcher_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"scheduling-system/backend/models"
	"scheduling-system/backend/scheduling/matcher"

	"github.com/google/go-cmp/cmp"
)

type fixture struct {
	Task      models.TeachingTask   `json:"task"`
	Course    models.Course         `json:"course"`
	Classroom models.Classroom      `json:"classroom"`
	Expected  matcher.MatchResult   `json:"expected"`
}

func TestMatch_Golden(t *testing.T) {
	files := []string{"basic.json", "lab.json", "computer.json", "sports.json", "art.json"}
	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", name)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			var fx fixture
			if err := json.Unmarshal(raw, &fx); err != nil {
				t.Fatalf("unmarshal %s: %v", path, err)
			}
			got := matcher.Match(fx.Task, fx.Course, fx.Classroom)
			if diff := cmp.Diff(fx.Expected, got); diff != "" {
				t.Errorf("Match(%s) mismatch (-want +got):\n%s", name, diff)
			}
		})
	}
}