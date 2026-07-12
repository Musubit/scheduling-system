package services

import (
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"
)

// SolverOrchestrator manages multiple solver engines (SA + OR-Tools) with automatic fallback.
type SolverOrchestrator struct {
	ortoolsClient *ORToolsClient
	ortoolsCmd    *exec.Cmd
	ortoolsReady  bool
	mu            sync.Mutex
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
	// Platform-specific process attributes (e.g. hide console window on Windows).
	// Implementation lives in solver_orchestrator_{windows,other}.go.
	configureCommand(o.ortoolsCmd)
	// Capture output for debugging
	o.ortoolsCmd.Stdout = log.Writer()
	o.ortoolsCmd.Stderr = log.Writer()

	if err := o.ortoolsCmd.Start(); err != nil {
		return fmt.Errorf("failed to start OR-Tools service: %w", err)
	}

	o.ortoolsClient = NewORToolsClient(port)
	log.Printf("OR-Tools service starting on port %d (PID: %d)", port, o.ortoolsCmd.Process.Pid)
	// 后台轮询就绪状态：Flask + ortools 导入需要几秒，避免首调健康检查失败而误降级 SA。
	go o.waitForHealthy(port, 20*time.Second)
	return nil
}

// waitForHealthy polls the OR-Tools /health endpoint until it responds 200
// or the timeout elapses. Non-blocking: app startup is never delayed.
func (o *SolverOrchestrator) waitForHealthy(port int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if o.ortoolsClient != nil && o.ortoolsClient.HealthCheck() {
			o.mu.Lock()
			o.ortoolsReady = true
			o.mu.Unlock()
			log.Printf("OR-Tools service healthy on port %d", port)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
		log.Printf("OR-Tools service not healthy within %v (will fall back to SA if unavailable)", timeout)
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
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.ortoolsReady
}

// GetORToolsClient returns the OR-Tools client, or nil if not available.
func (o *SolverOrchestrator) GetORToolsClient() *ORToolsClient {
	if o.IsORToolsAvailable() {
		return o.ortoolsClient
	}
	return nil
}
