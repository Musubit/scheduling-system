package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// AppConfig holds system-level application settings.
// These are stored in config/app.json alongside the executable.
type AppConfig struct {
	// SchedulerPort is the port for the OR-Tools Python microservice.
	SchedulerPort int `json:"schedulerPort"`
	// PythonPath overrides the Python executable path (empty = auto-detect).
	PythonPath string `json:"pythonPath"`
	// DataDir is the subdirectory for runtime data (relative to exe dir).
	DataDir string `json:"dataDir"`
}

var (
	defaultConfig = AppConfig{
		SchedulerPort: 19877,
		PythonPath:    "",
		DataDir:       "resources",
	}
	mu sync.RWMutex
)

// Filename returns the config file path relative to the given base directory.
func Filename(baseDir string) string {
	return filepath.Join(baseDir, "config", "app.json")
}

// Load reads config from config/app.json in the given base directory.
// If the file doesn't exist, returns defaults and writes the defaults to disk.
func Load(baseDir string) *AppConfig {
	mu.RLock()
	cfg := defaultConfig
	mu.RUnlock()

	path := Filename(baseDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Write defaults on first run
			if writeErr := Save(baseDir, &cfg); writeErr != nil {
				log.Printf("Config: cannot write default config: %v", writeErr)
			}
		}
		return &cfg
	}

	var loaded AppConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		log.Printf("Config: parse error, using defaults: %v", err)
		return &cfg
	}

	// Merge: fill zero fields from defaults
	if loaded.SchedulerPort <= 0 {
		loaded.SchedulerPort = defaultConfig.SchedulerPort
	}
	if loaded.DataDir == "" {
		loaded.DataDir = defaultConfig.DataDir
	}

	log.Printf("Config: loaded from %s", path)
	return &loaded
}

// Save writes the config to config/app.json, creating the directory if needed.
func Save(baseDir string, cfg *AppConfig) error {
	mu.Lock()
	defer mu.Unlock()

	dir := filepath.Join(baseDir, "config")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, "app.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	log.Printf("Config: saved to %s", path)
	return nil
}
