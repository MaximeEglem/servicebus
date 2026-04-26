package config_test

import (
	"testing"

	"servicebus/internal/config"
)

func TestStateRoundTrip(t *testing.T) {
	t.Setenv("SB_CONFIG_DIR", t.TempDir())
	s := &config.State{Context: "prod", Type: "dlq"}
	if err := config.SaveState(s); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	got, err := config.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got.Context != "prod" || got.Type != "dlq" {
		t.Errorf("got %+v, want {prod dlq}", got)
	}
}

func TestLoadStateMissing(t *testing.T) {
	t.Setenv("SB_CONFIG_DIR", t.TempDir()+"/nonexistent")
	s, err := config.LoadState()
	if err != nil {
		t.Fatalf("LoadState on missing file: %v", err)
	}
	if s.Context != "" || s.Type != "" {
		t.Errorf("expected empty state, got %+v", s)
	}
}
