// Package appenv provides a single source of truth for all path resolution.
// It separates the read-only installation directory from the writable user
// data directory, which is required (a) on Windows where Program Files is
// protected by UAC, and (b) for MSIX packages which are virtualised.
package appenv

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

// BaseDir returns the read-only directory containing the executable.
//
//	Dev mode   → project root (cwd, detected by go.mod / main.go)
//	Production → directory containing the running .exe
func BaseDir() string {
	if wd, err := os.Getwd(); err == nil {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		if _, err := os.Stat(filepath.Join(wd, "main.go")); err == nil {
			return wd
		}
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	return "."
}

// isDevMode returns true when running from a source tree (wails3 dev).
func isDevMode() bool {
	if wd, err := os.Getwd(); err == nil {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return true
		}
	}
	return false
}

// devDataDir returns the in-tree writable data directory used during
// `wails3 dev`, parameterised on baseDir so it can be unit-tested without
// depending on cwd. It must never collide with the built binary path
// (see devDataLeafName).
func devDataDir(baseDir string) string {
	return filepath.Join(baseDir, "bin", devDataLeafName())
}

// devDataLeafName picks the leaf directory name for dev-mode data.
//
// On Windows the built binary is bin/scheduling-system.exe, so a directory
// named bin/scheduling-system/ does not collide. On Linux (and any POSIX
// GOOS) the built binary is bin/scheduling-system (no suffix), so we need
// a distinct leaf name; otherwise startup fails with "not a directory"
// when trying to mkdir logs/ under a path that is already a regular file.
// We reuse the same layout Windows has always used to avoid disturbing
// existing installs.
func devDataLeafName() string {
	if runtime.GOOS == "windows" {
		return "scheduling-system"
	}
	return "scheduling-system-data"
}

// DataDir returns the writable user-data directory.
//
//	Windows: %LOCALAPPDATA%\scheduling-system
//	macOS:   ~/Library/Application Support/scheduling-system
//	Linux:   ~/.local/share/scheduling-system
//
// In dev mode returns a subdirectory under bin/ that never shadows the
// built binary — see devDataLeafName.
func DataDir() string {
	if isDevMode() {
		return devDataDir(BaseDir())
	}
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("LOCALAPPDATA")
		if base == "" {
			// Fallback if LOCALAPPDATA is somehow unset
			base = filepath.Join(os.Getenv("APPDATA"), "..", "Local")
		}
	case "darwin":
		base = filepath.Join(os.Getenv("HOME"), "Library", "Application Support")
	default:
		base = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}
	return filepath.Join(base, "scheduling-system")
}

// LogDir returns the writable directory for log files.
// It is a subdirectory of DataDir.
func LogDir() string {
	return filepath.Join(DataDir(), "logs")
}

// ConfigDir returns the writable directory for config files.
// It is a subdirectory of DataDir.
func ConfigDir() string {
	return filepath.Join(DataDir(), "config")
}

// ResourcesDir returns the writable directory for resource files (database, etc.).
// It is a subdirectory of DataDir.
func ResourcesDir() string {
	return filepath.Join(DataDir(), "resources")
}

// WebviewDir returns the parent directory for WebView2 runtime data
// (cache, cookies, IndexedDB, etc.). WebView2 will create its own
// EBWebView/ subdirectory inside it on demand.
//
// This is intentionally NOT created by EnsureDataDir — WebView2 runtime
// data is managed by the WebView2 runtime itself, not by the application.
// Keeping it out of the eager-create list preserves the distinction between
// application business data (logs/config/resources) and runtime cache.
//
// The returned path is absolute so Wails passes an unambiguous location to
// WebView2 — a relative path would be resolved next to the .exe, polluting
// the install directory (see Epic G2).
func WebviewDir() string {
	return filepath.Join(DataDir(), "webview")
}

// EnsureDataDir creates all standard subdirectories under DataDir.
// Safe to call on every startup — MkdirAll is a no-op for existing dirs.
func EnsureDataDir() error {
	dataDir := DataDir()
	dirs := []string{
		filepath.Join(dataDir, "logs"),
		filepath.Join(dataDir, "config"),
		filepath.Join(dataDir, "resources"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("appenv: cannot create %s: %w", d, err)
		}
	}
	return nil
}

// migrateFile copies a file from src to dst when src exists and dst does not.
// Returns true when a copy actually happened.
func migrateFile(src, dst string) (bool, error) {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return false, nil
	}
	if _, err := os.Stat(dst); err == nil {
		return false, nil // already exists — don't overwrite user data
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return false, err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return false, err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return false, err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return false, err
	}

	return true, nil
}

// MigrateConfigIfNeeded copies config/app.json from the install directory
// to the user data directory on first run (or when the data directory is fresh).
func MigrateConfigIfNeeded() {
	src := filepath.Join(BaseDir(), "config", "app.json")
	dst := filepath.Join(ConfigDir(), "app.json")
	if ok, err := migrateFile(src, dst); err != nil {
		log.Printf("appenv: config migration failed: %v", err)
	} else if ok {
		log.Printf("appenv: config seeded from %s", src)
	}
}

// MigrateDatabaseIfNeeded copies the SQLite database from the install directory
// to the user data directory. It only runs in production: the shipped
// schedule.db lives next to the executable and is copied into the writable
// user-data directory on first launch.
//
// In dev mode this is a no-op — the database is created fresh in the dev
// DataDir (bin/scheduling-system). Skipping migration here also prevents a
// corrupt legacy database from ever being reintroduced.
func MigrateDatabaseIfNeeded() {
	if isDevMode() {
		return
	}

	src := filepath.Join(BaseDir(), "resources", "schedule.db")
	dst := filepath.Join(ResourcesDir(), "schedule.db")
	if ok, err := migrateFile(src, dst); err != nil {
		log.Printf("appenv: database migration failed from %s: %v", src, err)
	} else if ok {
		log.Printf("appenv: database migrated from %s", src)
	}
}
