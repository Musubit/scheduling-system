package main

import (
	"embed"

	"scheduling-system/backend"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed scheduler/solver.py scheduler/pyproject.toml
var schedulerFiles embed.FS

func main() {
	backend.Run(assets)
}
