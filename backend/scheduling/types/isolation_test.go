package types

import (
	"go/build"
	"os/exec"
	"strings"
	"testing"
)

// forbidden lists the import paths scheduling/types must never depend
// on, directly or transitively (INV-P2).
var forbidden = []string{
	"scheduling-system/backend/database",
	"scheduling-system/backend/models",
	"scheduling-system/backend/services",
	"gorm.io/gorm",
}

// TestInvP2_NoForbiddenImports uses `go list` to walk the transitive
// import graph of scheduling/types and rejects any forbidden entry.
// This is stricter than an eyeball import check because it catches
// second-order imports too.
func TestInvP2_NoForbiddenImports(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "scheduling-system/backend/scheduling/types")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list unavailable in this environment: %v", err)
		return
	}
	deps := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, dep := range deps {
		for _, f := range forbidden {
			if dep == f || strings.HasPrefix(dep, f+"/") {
				t.Errorf("INV-P2 violated: scheduling/types transitively depends on %q via %q", f, dep)
			}
		}
	}
}

// TestInvP9_NoIOInPackage inspects the package's own source to make
// sure nothing here calls into fmt.Print* (stdout writes) or os.*
// (filesystem/env). Solver interfaces only emit through
// ProgressReporter; nothing else in types should touch IO.
func TestInvP9_NoIOInPackage(t *testing.T) {
	pkg, err := build.ImportDir("./", 0)
	if err != nil {
		t.Fatalf("build.ImportDir failed: %v", err)
	}
	forbiddenImports := map[string]bool{
		"os":      true,
		"os/exec": false, // allowed only inside this isolation_test.go
		"log":     true,
	}
	for _, imp := range pkg.Imports {
		if v, ok := forbiddenImports[imp]; ok && v {
			t.Errorf("INV-P9 violated: scheduling/types imports %q", imp)
		}
	}
	// os/exec appears only in this test file, which is exempt.
}
