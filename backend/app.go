package backend

import (
	"embed"
	"io"
	"log"
	"os"
	"path/filepath"

	"scheduling-system/backend/config"
	"scheduling-system/backend/database"
	"scheduling-system/backend/services"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Run starts the Wails application with the given embedded frontend assets.
func Run(assets embed.FS) {
	// Resolve base directory (exe location, or project root in dev mode)
	baseDir := resolveBaseDir()

	// Initialize logging to both console and logs/app.log
	initLogging(baseDir)

	// Load config from config/app.json
	cfg := config.Load(baseDir)
	log.Printf("App: baseDir=%s, config=%+v", baseDir, *cfg)

	// Initialize database (resources/schedule.db)
	db, err := database.InitDB(baseDir)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create OR-Tools orchestrator
	orchestrator := services.NewSolverOrchestrator()

	// Try to start scheduler service
	startSchedulerIfAvailable(orchestrator, baseDir, cfg)

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

// resolveBaseDir returns the executable directory, falling back to working directory.
// Development mode (wails3 dev): cwd = project root → project root
// Production mode: exe dir → installation directory
func resolveBaseDir() string {
	// In dev mode, os.Executable returns the temp build dir, not the project root.
	// Check if cwd has go.mod / main.go to detect dev mode.
	if wd, err := os.Getwd(); err == nil {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		if _, err := os.Stat(filepath.Join(wd, "main.go")); err == nil {
			return wd
		}
	}
	// Production: use executable directory
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	return "."
}

// initLogging sets up dual output: console (stdout) + logs/app.log.
func initLogging(baseDir string) {
	logDir := filepath.Join(baseDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("Warning: cannot create log directory: %v", err)
		return
	}

	logPath := filepath.Join(logDir, "app.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("Warning: cannot open log file: %v", err)
		return
	}

	// Write to both stdout and file
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	log.Printf("Logging initialized: %s", logPath)
}

// startSchedulerIfAvailable starts the Python OR-Tools solver service.
// Search order:
//  1. scheduler/solver.py (development — with .venv)
//  2. scheduler.exe (production — PyInstaller bundle)
//  3. Not found → SA-only mode (silent fallback)
func startSchedulerIfAvailable(orchestrator *services.SolverOrchestrator, baseDir string, cfg *config.AppConfig) {
	port := cfg.SchedulerPort
	if port <= 0 {
		port = 19877
	}

	// Check for scheduler directory (development mode)
	schedulerDir := filepath.Join(baseDir, "scheduler")
	scriptPath := filepath.Join(schedulerDir, "solver.py")
	if _, err := os.Stat(scriptPath); err == nil {
		// Find Python in .venv
		venvPython := filepath.Join(schedulerDir, ".venv", "Scripts", "python.exe")
		if _, err := os.Stat(venvPython); err != nil {
			venvPython = filepath.Join(schedulerDir, ".venv", "bin", "python")
		}

		pythonPath := cfg.PythonPath
		if pythonPath == "" {
			if _, err := os.Stat(venvPython); err == nil {
				pythonPath = venvPython
			} else {
				pythonPath = "python"
			}
		}

		log.Printf("Scheduler: found solver.py, Python=%s", pythonPath)
		if err := orchestrator.StartORTools(pythonPath, scriptPath, port); err != nil {
			log.Printf("Scheduler: failed to start: %v", err)
		} else {
			log.Printf("Scheduler: started on port %d", port)
		}
		return
	}

	// Check for scheduler.exe (production mode)
	schedulerExe := filepath.Join(baseDir, "scheduler.exe")
	if _, err := os.Stat(schedulerExe); err == nil {
		log.Printf("Scheduler: found scheduler.exe")
		// scheduler.exe accepts port as CLI arg
		if err := orchestrator.StartORTools(schedulerExe, "", port); err != nil {
			log.Printf("Scheduler: failed to start exe: %v", err)
		} else {
			log.Printf("Scheduler: started on port %d", port)
		}
		return
	}

	log.Println("Scheduler: not found, running in SA-only mode")
}
