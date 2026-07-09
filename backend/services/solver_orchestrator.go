package services

import (
	"fmt"
	"log"
	"os/exec"
)

// SolverOrchestrator manages multiple solver engines (SA + OR-Tools) with automatic fallback.
type SolverOrchestrator struct {
	ortoolsClient *ORToolsClient
	ortoolsCmd    *exec.Cmd
}

// NewSolverOrchestrator creates a new orchestrator.
func NewSolverOrchestrator() *SolverOrchestrator {
	return &SolverOrchestrator{}
}

// StartORTools attempts to start the OR-Tools Python microservice.
// pythonPath: path to Python executable (or scheduler.exe for production).
// scriptPath: path to solver.py (empty if using scheduler.exe directly).
// port: the port number to listen on.
func (o *SolverOrchestrator) StartORTools(pythonPath string, scriptPath string, port int) error {
	if scriptPath == "" {
		// Running as standalone exe (scheduler.exe)
		o.ortoolsCmd = exec.Command(pythonPath, fmt.Sprintf("%d", port))
	} else {
		// Running as Python script (solver.py)
		o.ortoolsCmd = exec.Command(pythonPath, scriptPath, fmt.Sprintf("%d", port))
	}
	// Capture output for debugging
	o.ortoolsCmd.Stdout = log.Writer()
	o.ortoolsCmd.Stderr = log.Writer()

	if err := o.ortoolsCmd.Start(); err != nil {
		return fmt.Errorf("failed to start OR-Tools service: %w", err)
	}

	o.ortoolsClient = NewORToolsClient(port)
	log.Printf("OR-Tools service started on port %d (PID: %d)", port, o.ortoolsCmd.Process.Pid)
	return nil
}

// StopORTools stops the OR-Tools microservice if running.
func (o *SolverOrchestrator) StopORTools() {
	if o.ortoolsCmd != nil && o.ortoolsCmd.Process != nil {
		o.ortoolsCmd.Process.Kill()
		log.Println("OR-Tools service stopped")
	}
}

// IsORToolsAvailable returns true if the OR-Tools service is running and healthy.
func (o *SolverOrchestrator) IsORToolsAvailable() bool {
	if o.ortoolsClient == nil {
		return false
	}
	return o.ortoolsClient.HealthCheck()
}

// GetORToolsClient returns the OR-Tools client, or nil if not available.
func (o *SolverOrchestrator) GetORToolsClient() *ORToolsClient {
	if o.IsORToolsAvailable() {
		return o.ortoolsClient
	}
	return nil
}
