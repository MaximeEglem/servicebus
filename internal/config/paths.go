package config

import (
	"os"
	"path/filepath"
)

func ConfigDir() string {
	if v := os.Getenv("SB_CONFIG_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".sb")
}

func ConfigPath() string { return filepath.Join(ConfigDir(), "config.yaml") }
func StatePath() string  { return filepath.Join(ConfigDir(), "state.yaml") }
func CacheDir() string   { return filepath.Join(ConfigDir(), "cache") }
