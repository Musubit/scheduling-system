//go:build windows

package services

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// configureCommand applies Windows-specific process attributes so the spawned
// solver subprocess (scheduler.exe / python.exe) does not create a console
// window. Without this, a GUI-subsystem parent still triggers a transient
// black window when it spawns a console-subsystem child.
//
// SysProcAttr is anchored to syscall (imposed by os/exec); the constant comes
// from golang.org/x/sys/windows to avoid magic numbers and to align with the
// modern Windows API surface.
//
// stdout / stderr piping via cmd.Stdout / cmd.Stderr is unaffected — Go
// creates anonymous pipes for those regardless of CREATE_NO_WINDOW.
func configureCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
}
