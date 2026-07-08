package main

import (
	"embed"
	"log"

	"scheduling-system/database"
	"scheduling-system/services"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Initialize database
	db, err := database.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create services
	resources := services.NewResourceService(db)
	teachingTasks := services.NewTeachingTaskService(db)
	snapshots := services.NewSnapshotService(db)
	scheduler := services.NewSchedulingService(db, snapshots)
	moves := services.NewMoveService(db)
	orchestrator := services.NewSolverOrchestrator()

	// Try to start OR-Tools service (non-fatal if unavailable)
	// pythonPath and scriptPath would be resolved at runtime from embedded resources
	_ = orchestrator

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
