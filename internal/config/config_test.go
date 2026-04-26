package config_test

import (
	"path/filepath"
	"testing"

	"github.com/maxime/sb/internal/config"
)

func TestConfigRoundTrip(t *testing.T) {
	t.Setenv("SB_CONFIG_DIR", t.TempDir())
	cfg := &config.Config{
		CurrentContext: "dev",
		Contexts: map[string]config.Context{
			"dev": {ConnectionString: "Endpoint=sb://dev..."},
		},
		Settings: config.Settings{MaxItems: 50},
	}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.CurrentContext != "dev" {
		t.Errorf("CurrentContext = %q, want %q", got.CurrentContext, "dev")
	}
	if got.Contexts["dev"].ConnectionString != "Endpoint=sb://dev..." {
		t.Errorf("ConnectionString mismatch")
	}
	_ = filepath.Join("used", "by", "SB_CONFIG_DIR")
}

func TestLoadMissingReturnsDefaults(t *testing.T) {
	t.Setenv("SB_CONFIG_DIR", t.TempDir()+"/nonexistent")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if cfg.Settings.MaxItems != 50 {
		t.Errorf("default MaxItems = %d, want 50", cfg.Settings.MaxItems)
	}
	if cfg.Settings.Thresholds.SizePercentage != 70 {
		t.Errorf("default SizePercentage = %d, want 70", cfg.Settings.Thresholds.SizePercentage)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SB_CONFIG_DIR", dir+"/nested/dir")
	cfg := &config.Config{Contexts: map[string]config.Context{}}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save should create directories: %v", err)
	}
}
