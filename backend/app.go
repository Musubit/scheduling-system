package backend

import (
	"embed"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"scheduling-system/backend/appenv"
	"scheduling-system/backend/config"
	"scheduling-system/backend/database"
	"scheduling-system/backend/services"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Run starts the Wails application with the given embedded frontend assets.
func Run(assets embed.FS) {
	// Ensure writable user-data directory exists (%LOCALAPPDATA%/scheduling-system)
	if err := appenv.EnsureDataDir(); err != nil {
		log.Fatalf("App: cannot initialize data directory: %v", err)
	}

	// Migrate existing data from install dir to user data dir (first run only)
	appenv.MigrateConfigIfNeeded()
	appenv.MigrateDatabaseIfNeeded()

	// Resolve base directory for read-only assets (scheduler.exe, solver.py)
	baseDir := appenv.BaseDir()

	// Initialize logging to both console and logs/app.log (in data dir)
	initLogging(appenv.LogDir())

	// Load config from config/app.json (in data dir)
	cfg := config.Load(appenv.ConfigDir())
	log.Printf("App: baseDir=%s, dataDir=%s, config=%+v", baseDir, appenv.DataDir(), *cfg)

	// Initialize database (resources/schedule.db in data dir)
	db, err := database.InitDB(appenv.ResourcesDir())
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create OR-Tools orchestrator
	orchestrator := services.NewSolverOrchestrator()

	// Try to start scheduler service (looks in baseDir for scheduler.exe/solver.py)
	startSchedulerIfAvailable(orchestrator, baseDir, cfg)

	// Create services
	resources := services.NewResourceService(db)
	teachingTasks := services.NewTeachingTaskService(db)
	versions := services.NewVersionService(db)
	scheduler := services.NewSchedulingService(db, versions, orchestrator)
	moves := services.NewMoveService(db)

	// Create Wails application
	app := application.New(application.Options{
		Name:        "高校智能排课系统",
		Description: "高校智能排课系统",
		Services: []application.Service{
			application.NewService(resources),
			application.NewService(teachingTasks),
			application.NewService(scheduler),
			application.NewService(versions),
			application.NewService(moves),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Windows: application.WindowsOptions{
			// Absolute path so WebView2 does not create EBWebView next to the .exe.
			// Wails resolves relative paths against the exe directory — passing an
			// absolute path under %LOCALAPPDATA%\scheduling-system\webview keeps
			// the install directory clean (Epic G2).
			WebviewUserDataPath: appenv.WebviewDir(),
		},
	})

	// Cleanup on exit
	defer orchestrator.StopORTools()

	// Create main window
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "高校智能排课系统",
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


// initLogging sets up dual output: console (stdout) + logs/app.log.
func initLogging(logDir string) {
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
		// Locate a venv Python interpreter. Windows uses .venv/Scripts/python.exe,
		// POSIX uses .venv/bin/python. We probe the current-platform path first
		// to skip a wasted os.Stat on the wrong layout.
		var venvCandidates []string
		if runtime.GOOS == "windows" {
			venvCandidates = []string{
				filepath.Join(schedulerDir, ".venv", "Scripts", "python.exe"),
				filepath.Join(schedulerDir, ".venv", "bin", "python"),
			}
		} else {
			venvCandidates = []string{
				filepath.Join(schedulerDir, ".venv", "bin", "python"),
				filepath.Join(schedulerDir, ".venv", "Scripts", "python.exe"),
			}
		}
		var venvPython string
		for _, cand := range venvCandidates {
			if _, err := os.Stat(cand); err == nil {
				venvPython = cand
				break
			}
		}

		pythonPath := cfg.PythonPath
		if pythonPath == "" {
			if venvPython != "" {
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

	// Check for the PyInstaller-bundled scheduler binary in scheduler/
	// subdirectory (production mode). Windows produces scheduler.exe; Linux
	// produces plain "scheduler". We probe both so the same portable layout
	// works on either platform without an OS-specific config switch.
	candidates := []string{
		filepath.Join(baseDir, "scheduler", "scheduler.exe"),
		filepath.Join(baseDir, "scheduler", "scheduler"),
	}
	for _, schedulerExe := range candidates {
		if _, err := os.Stat(schedulerExe); err != nil {
			continue
		}
		log.Printf("Scheduler: found bundled binary %s", schedulerExe)
		// Bundled binary accepts port as CLI arg (no script path).
		if err := orchestrator.StartORTools(schedulerExe, "", port); err != nil {
			log.Printf("Scheduler: failed to start bundled binary: %v", err)
		} else {
			log.Printf("Scheduler: started on port %d", port)
		}
		return
	}

	log.Println("Scheduler: not found, running in SA-only mode")
}
