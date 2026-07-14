package appenv

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDevDataDirDoesNotCollideWithBinary reproduces the Linux dev-mode
// startup bug: the built binary is bin/scheduling-system (no suffix on
// POSIX), so the dev data directory must not literally be
// `bin/scheduling-system` — the application would try to `mkdir
// bin/scheduling-system/logs` and hit "not a directory" because a regular
// file already exists at that path.
//
// Windows is unaffected: its binary is bin/scheduling-system.exe, so the
// long-standing `bin/scheduling-system/` layout still works there and we
// preserve it to avoid disturbing existing installs.
func TestDevDataDirDoesNotCollideWithBinary(t *testing.T) {
	base := "/tmp/fake-project"
	got := devDataDir(base)

	// The built-binary path (Linux/macOS have no suffix; Windows adds .exe).
	binaryPath := filepath.Join(base, "bin", "scheduling-system")
	if runtime.GOOS != "windows" {
		if got == binaryPath {
			t.Fatalf("devDataDir(%q)=%q collides with binary at %q — mkdir logs would fail with 'not a directory'",
				base, got, binaryPath)
		}
	}

	// Whatever the dev dir is, it must live *under* bin/ so it stays out
	// of the project root — preserving the original design intent.
	binPrefix := filepath.Join(base, "bin") + string(filepath.Separator)
	if !strings.HasPrefix(got+string(filepath.Separator), binPrefix) {
		t.Fatalf("devDataDir(%q)=%q should live under %q", base, got, binPrefix)
	}
}

// TestDevDataLeafNamePreservesWindowsLayout locks in the invariant that we
// don't rename the Windows dev data directory — long-standing installs
// expect bin/scheduling-system/.
func TestDevDataLeafNamePreservesWindowsLayout(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only invariant")
	}
	if got := devDataLeafName(); got != "scheduling-system" {
		t.Fatalf("devDataLeafName()=%q, want %q on windows", got, "scheduling-system")
	}
}

// TestEnsureDataDirIsIdempotent guards that startup can create the data
// dir cleanly, and calling twice does not error. Regression coverage for
// the "not a directory" failure mode.
func TestEnsureDataDirIsIdempotent(t *testing.T) {
	if err := EnsureDataDir(); err != nil {
		t.Fatalf("first EnsureDataDir: %v", err)
	}
	if err := EnsureDataDir(); err != nil {
		t.Fatalf("second EnsureDataDir: %v", err)
	}
}
