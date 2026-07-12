//go:build !windows

package services

import "os/exec"

// configureCommand is a no-op on non-Windows platforms.
// The hidden-console-window workaround is Windows-specific; other OSes
// do not spawn a console window for GUI-parent → CLI-child transitions.
func configureCommand(cmd *exec.Cmd) {
	_ = cmd
}
