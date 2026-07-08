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

	// Create Wails application
	app := application.New(application.Options{
		Name:        "高校排课系统",
		Description: "高校智能排课管理系统",
		Services: []application.Service{
			application.NewService(services.NewResourceService(db)),
			application.NewService(services.NewSchedulingService(db)),
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
