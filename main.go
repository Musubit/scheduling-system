package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"scheduling-system/database"
	"scheduling-system/services"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed ortools_service/solver.py ortools_service/pyproject.toml
var ortoolsFiles embed.FS

func main() {
	// Initialize database
	db, err := database.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create OR-Tools orchestrator
	orchestrator := services.NewSolverOrchestrator()

	// Try to start OR-Tools service if ortools_service directory exists
	startORToolsIfAvailable(orchestrator)

	// Create services
	resources := services.NewResourceService(db)
	teachingTasks := services.NewTeachingTaskService(db)
	snapshots := services.NewSnapshotService(db)
	scheduler := services.NewSchedulingService(db, snapshots, orchestrator)
	moves := services.NewMoveService(db)

	// Create Wails application
	app := application.New(application.Options{
		Name:        "高校排课系统",
		Description: "高校智能排课管理系统",
		Services: []application.Service{
			application.NewService(resources),
			application.NewService(teachingTasks),
			application.NewService(scheduler),
			application.NewService(snapshots),
			application.NewService(moves),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Windows: application.WindowsOptions{
			WebviewUserDataPath: "scheduling-system",
		},
	})

	// Cleanup on exit
	defer orchestrator.StopORTools()

	// Create main window
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "高校排课系统",
		Width:     1440,
		Height:    900,
		MinWidth:  1024,
		MinHeight: 680,
		BackgroundColour: application.NewRGB(235, 236, 240),
		URL:       "/",
	})

	if err := app.Run(); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}

// startORToolsIfAvailable looks for the ortools_service directory next to the executable
// and starts the Python solver service if found.
func startORToolsIfAvailable(orchestrator *services.SolverOrchestrator) {
	// Check for ortools_service in the same directory as the executable
	exePath, err := os.Executable()
	if err != nil {
		log.Println("ORTools: Cannot determine executable path, skipping")
		return
	}
	exeDir := filepath.Dir(exePath)

	// Try common locations for ortools_service
	locations := []string{
		filepath.Join(exeDir, "ortools_service"),
		"ortools_service", // relative to working directory
	}

	var pythonPath, scriptPath string
	for _, loc := range locations {
		script := filepath.Join(loc, "solver.py")
		if _, err := os.Stat(script); err == nil {
			scriptPath = script

			// Find Python in .venv
			venvPython := filepath.Join(loc, ".venv", "Scripts", "python.exe") // Windows
			if _, err := os.Stat(venvPython); err != nil {
				venvPython = filepath.Join(loc, ".venv", "bin", "python") // Unix
			}
			if _, err := os.Stat(venvPython); err == nil {
				pythonPath = venvPython
			} else {
				// Fallback to system Python
				pythonPath = "python"
			}
			break
		}
	}

	if scriptPath == "" {
		log.Println("ORTools: ortools_service not found, running in SA-only mode")
		return
	}

	log.Printf("ORTools: Found solver at %s, Python at %s", scriptPath, pythonPath)

	if err := orchestrator.StartORTools(pythonPath, scriptPath, 19877); err != nil {
		log.Printf("ORTools: Failed to start service: %v, SA-only mode", err)
	} else {
		log.Println("ORTools: Service started successfully on port 19877")
	}
}
