package config

import "testing"

func TestNewDefault(t *testing.T) {
	cfg := NewDefault()

	if cfg.Theme != "dark" {
		t.Errorf("Theme: got %q, want %q", cfg.Theme, "dark")
	}
	if cfg.CopyCmd != "wl-copy" {
		t.Errorf("CopyCmd: got %q, want %q", cfg.CopyCmd, "wl-copy")
	}
	if cfg.DeltaPath != "" {
		t.Errorf("DeltaPath: got %q, want %q", cfg.DeltaPath, "")
	}
	if cfg.LogDepth != 30 {
		t.Errorf("LogDepth: got %d, want %d", cfg.LogDepth, 30)
	}
	if cfg.DiffContext != 3 {
		t.Errorf("DiffContext: got %d, want %d", cfg.DiffContext, 3)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel: got %q, want %q", cfg.LogLevel, "warn")
	}
}
