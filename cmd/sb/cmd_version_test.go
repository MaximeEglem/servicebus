package main

import "testing"

func TestDisplayVersionUsesSemanticPrefix(t *testing.T) {
	got := displayVersion("v1.2.3-4-gabcdef")
	if got != "v1.2.3" {
		t.Fatalf("displayVersion() = %q, want %q", got, "v1.2.3")
	}
}

func TestDisplayVersionFallsBackToRawVersion(t *testing.T) {
	got := displayVersion("dev")
	if got != "dev" {
		t.Fatalf("displayVersion() = %q, want %q", got, "dev")
	}
}
