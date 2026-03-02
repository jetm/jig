package config

import (
	"os"
	"path/filepath"
	"testing"
)

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
	if cfg.DiffRenderer != "chroma" {
		t.Errorf("DiffRenderer: got %q, want %q", cfg.DiffRenderer, "chroma")
	}
	if cfg.LogCommitLimit != 50 {
		t.Errorf("LogCommitLimit: got %d, want %d", cfg.LogCommitLimit, 50)
	}
	if cfg.RebaseDefaultBase != "HEAD~10" {
		t.Errorf("RebaseDefaultBase: got %q, want %q", cfg.RebaseDefaultBase, "HEAD~10")
	}
	if cfg.UITheme != "dark" {
		t.Errorf("UITheme: got %q, want %q", cfg.UITheme, "dark")
	}
	if !cfg.ShowDiffPanel {
		t.Error("ShowDiffPanel: got false, want true")
	}
}

// isolatedLoad calls Load() with HOME pointed at a temp dir so tests
// don't accidentally read the developer's real config file.
func isolatedLoad(t *testing.T, homeDir string) (Config, error) {
	t.Helper()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	return Load()
}

func TestLoad_NoFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	def := NewDefault()
	if cfg.DiffRenderer != def.DiffRenderer {
		t.Errorf("DiffRenderer: got %q, want %q", cfg.DiffRenderer, def.DiffRenderer)
	}
	if cfg.LogCommitLimit != def.LogCommitLimit {
		t.Errorf("LogCommitLimit: got %d, want %d", cfg.LogCommitLimit, def.LogCommitLimit)
	}
	if cfg.RebaseDefaultBase != def.RebaseDefaultBase {
		t.Errorf("RebaseDefaultBase: got %q, want %q", cfg.RebaseDefaultBase, def.RebaseDefaultBase)
	}
	if cfg.UITheme != def.UITheme {
		t.Errorf("UITheme: got %q, want %q", cfg.UITheme, def.UITheme)
	}
}

func TestLoad_XDGFileAllFields(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "jig")
	if err := os.MkdirAll(cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := `
diff:
  renderer: delta
log:
  commitLimit: 100
rebase:
  defaultBase: HEAD~5
ui:
  theme: light
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DiffRenderer != "delta" {
		t.Errorf("DiffRenderer: got %q, want %q", cfg.DiffRenderer, "delta")
	}
	if cfg.LogCommitLimit != 100 {
		t.Errorf("LogCommitLimit: got %d, want %d", cfg.LogCommitLimit, 100)
	}
	if cfg.RebaseDefaultBase != "HEAD~5" {
		t.Errorf("RebaseDefaultBase: got %q, want %q", cfg.RebaseDefaultBase, "HEAD~5")
	}
	if cfg.UITheme != "light" {
		t.Errorf("UITheme: got %q, want %q", cfg.UITheme, "light")
	}
}

func TestLoad_FallbackDotFile(t *testing.T) {
	dir := t.TempDir()
	// No XDG config dir, only ~/.jig.yaml
	content := `
diff:
  renderer: plain
`
	if err := os.WriteFile(filepath.Join(dir, ".jig.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DiffRenderer != "plain" {
		t.Errorf("DiffRenderer: got %q, want %q", cfg.DiffRenderer, "plain")
	}
	// Other fields should remain default
	if cfg.LogCommitLimit != 50 {
		t.Errorf("LogCommitLimit: got %d, want %d", cfg.LogCommitLimit, 50)
	}
}

func TestLoad_PartialFile(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "jig")
	if err := os.MkdirAll(cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	// Only set one field
	content := `
log:
  commitLimit: 200
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.LogCommitLimit != 200 {
		t.Errorf("LogCommitLimit: got %d, want %d", cfg.LogCommitLimit, 200)
	}
	// Unset fields remain default
	if cfg.DiffRenderer != "chroma" {
		t.Errorf("DiffRenderer: got %q, want %q", cfg.DiffRenderer, "chroma")
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "jig")
	if err := os.MkdirAll(cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("diff: [bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := isolatedLoad(t, dir)
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "jig")
	if err := os.MkdirAll(cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := `
diff:
  renderer: chroma
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("JIG_DIFF_RENDERER", "delta")
	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DiffRenderer != "delta" {
		t.Errorf("DiffRenderer: got %q, want %q", cfg.DiffRenderer, "delta")
	}
}

func TestLoad_EnvOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JIG_LOG_COMMIT_LIMIT", "100")
	t.Setenv("JIG_REBASE_DEFAULT_BASE", "main")
	t.Setenv("JIG_UI_THEME", "light")

	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.LogCommitLimit != 100 {
		t.Errorf("LogCommitLimit: got %d, want %d", cfg.LogCommitLimit, 100)
	}
	if cfg.RebaseDefaultBase != "main" {
		t.Errorf("RebaseDefaultBase: got %q, want %q", cfg.RebaseDefaultBase, "main")
	}
	if cfg.UITheme != "light" {
		t.Errorf("UITheme: got %q, want %q", cfg.UITheme, "light")
	}
}

func TestLoad_InvalidNumericEnvVar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JIG_LOG_COMMIT_LIMIT", "abc")

	_, err := isolatedLoad(t, dir)
	if err == nil {
		t.Fatal("expected error for invalid JIG_LOG_COMMIT_LIMIT, got nil")
	}
}

func TestLoad_DefaultXDGPath(t *testing.T) {
	// Cover the xdgConfigHome=="" branch: unset XDG_CONFIG_HOME, rely on HOME/.config/...
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", "") // explicitly empty to trigger default

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// Should return defaults when no file exists
	def := NewDefault()
	if cfg.DiffRenderer != def.DiffRenderer {
		t.Errorf("DiffRenderer: got %q, want %q", cfg.DiffRenderer, def.DiffRenderer)
	}
}

func TestConfigPaths_NonEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", "")
	paths := configPaths()
	if len(paths) == 0 {
		t.Fatal("configPaths() returned empty slice")
	}
}

func TestLoad_ShowDiffPanelFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "jig")
	if err := os.MkdirAll(cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := `
ui:
  showDiffPanel: false
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ShowDiffPanel {
		t.Error("ShowDiffPanel: got true, want false")
	}
}

func TestLoad_ShowDiffPanelEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "jig")
	if err := os.MkdirAll(cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := `
ui:
  showDiffPanel: true
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("JIG_SHOW_DIFF_PANEL", "false")
	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ShowDiffPanel {
		t.Error("ShowDiffPanel: got true, want false (env override)")
	}
}

func TestLoad_ShowDiffPanelEnvOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JIG_SHOW_DIFF_PANEL", "false")

	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ShowDiffPanel {
		t.Error("ShowDiffPanel: got true, want false (env override of default)")
	}
}

func TestLoad_ShowDiffPanelInvalidEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JIG_SHOW_DIFF_PANEL", "invalid")

	_, err := isolatedLoad(t, dir)
	if err == nil {
		t.Fatal("expected error for invalid JIG_SHOW_DIFF_PANEL, got nil")
	}
}

func TestSave_CreatesConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))

	cfg := NewDefault()
	cfg.ShowDiffPanel = false

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Load it back and verify
	loaded, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}
	if loaded.ShowDiffPanel {
		t.Error("ShowDiffPanel: got true after save+load, want false")
	}
	if loaded.DiffRenderer != cfg.DiffRenderer {
		t.Errorf("DiffRenderer: got %q, want %q", loaded.DiffRenderer, cfg.DiffRenderer)
	}
}

func TestSave_OverwritesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))

	cfg := NewDefault()
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	cfg.ShowDiffPanel = false
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() second call error: %v", err)
	}

	loaded, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.ShowDiffPanel {
		t.Error("ShowDiffPanel: got true, want false after overwrite")
	}
}

func TestNewDefault_PanelRatio(t *testing.T) {
	cfg := NewDefault()
	if cfg.PanelRatio != 40 {
		t.Errorf("PanelRatio: got %d, want 40", cfg.PanelRatio)
	}
}

func TestLoad_PanelRatioDefault(t *testing.T) {
	dir := t.TempDir()
	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.PanelRatio != 40 {
		t.Errorf("PanelRatio: got %d, want 40", cfg.PanelRatio)
	}
}

func TestLoad_PanelRatioFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "jig")
	if err := os.MkdirAll(cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := `
ui:
  panelRatio: 70
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.PanelRatio != 70 {
		t.Errorf("PanelRatio: got %d, want 70", cfg.PanelRatio)
	}
}

func TestLoad_PanelRatioFileOmitted(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "jig")
	if err := os.MkdirAll(cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := `
ui:
  theme: dark
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.PanelRatio != 40 {
		t.Errorf("PanelRatio: got %d, want 40 (default when file omits it)", cfg.PanelRatio)
	}
}

func TestLoad_PanelRatioEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "jig")
	if err := os.MkdirAll(cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := `
ui:
  panelRatio: 50
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("JIG_PANEL_RATIO", "30")
	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.PanelRatio != 30 {
		t.Errorf("PanelRatio: got %d, want 30 (env override)", cfg.PanelRatio)
	}
}

func TestLoad_PanelRatioEnvOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JIG_PANEL_RATIO", "25")
	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.PanelRatio != 25 {
		t.Errorf("PanelRatio: got %d, want 25", cfg.PanelRatio)
	}
}

func TestLoad_PanelRatioBoundaryMin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JIG_PANEL_RATIO", "20")
	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.PanelRatio != 20 {
		t.Errorf("PanelRatio: got %d, want 20", cfg.PanelRatio)
	}
}

func TestLoad_PanelRatioBoundaryMax(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JIG_PANEL_RATIO", "80")
	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.PanelRatio != 80 {
		t.Errorf("PanelRatio: got %d, want 80", cfg.PanelRatio)
	}
}

func TestLoad_PanelRatioBelowMin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JIG_PANEL_RATIO", "10")
	_, err := isolatedLoad(t, dir)
	if err == nil {
		t.Fatal("expected error for JIG_PANEL_RATIO=10 (below min 20), got nil")
	}
}

func TestLoad_PanelRatioAboveMax(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JIG_PANEL_RATIO", "90")
	_, err := isolatedLoad(t, dir)
	if err == nil {
		t.Fatal("expected error for JIG_PANEL_RATIO=90 (above max 80), got nil")
	}
}

func TestLoad_PanelRatioInvalidEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JIG_PANEL_RATIO", "abc")
	_, err := isolatedLoad(t, dir)
	if err == nil {
		t.Fatal("expected error for invalid JIG_PANEL_RATIO, got nil")
	}
}

func TestNewDefault_SoftWrap(t *testing.T) {
	cfg := NewDefault()
	if !cfg.SoftWrap {
		t.Error("SoftWrap: got false, want true")
	}
}

func TestLoad_SoftWrapDefault(t *testing.T) {
	dir := t.TempDir()
	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.SoftWrap {
		t.Error("SoftWrap: got false, want true (default)")
	}
}

func TestLoad_SoftWrapFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "jig")
	if err := os.MkdirAll(cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := `
ui:
  softWrap: true
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.SoftWrap {
		t.Error("SoftWrap: got false, want true")
	}
}

func TestLoad_SoftWrapFileOmitted(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "jig")
	if err := os.MkdirAll(cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := `
ui:
  theme: dark
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.SoftWrap {
		t.Error("SoftWrap: got false, want true (default when file omits it)")
	}
}

func TestLoad_SoftWrapEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "jig")
	if err := os.MkdirAll(cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := `
ui:
  softWrap: false
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("JIG_SOFT_WRAP", "true")
	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.SoftWrap {
		t.Error("SoftWrap: got false, want true (env override)")
	}
}

func TestLoad_SoftWrapEnvOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JIG_SOFT_WRAP", "true")
	cfg, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.SoftWrap {
		t.Error("SoftWrap: got false, want true (env override of default)")
	}
}

func TestLoad_SoftWrapInvalidEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JIG_SOFT_WRAP", "invalid")
	_, err := isolatedLoad(t, dir)
	if err == nil {
		t.Fatal("expected error for invalid JIG_SOFT_WRAP, got nil")
	}
}

func TestSave_SoftWrapRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))

	cfg := NewDefault()
	cfg.SoftWrap = true

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}
	if !loaded.SoftWrap {
		t.Error("SoftWrap: got false after save+load, want true")
	}
}

func TestSave_PanelRatioRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))

	cfg := NewDefault()
	cfg.PanelRatio = 60

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := isolatedLoad(t, dir)
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}
	if loaded.PanelRatio != 60 {
		t.Errorf("PanelRatio: got %d after save+load, want 60", loaded.PanelRatio)
	}
}

func TestSave_CreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))

	// Ensure the directory does not exist
	cfgDir := filepath.Join(dir, ".config", "jig")
	if _, err := os.Stat(cfgDir); err == nil {
		t.Fatal("config dir should not exist before Save()")
	}

	cfg := NewDefault()
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Directory should now exist
	if _, err := os.Stat(cfgDir); err != nil {
		t.Errorf("config dir should exist after Save(): %v", err)
	}
}

func TestLoad_UnknownFieldReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "jig")
	if err := os.MkdirAll(cfgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := `
diff:
  renderer: chroma
  unknownField: true
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := isolatedLoad(t, dir)
	if err == nil {
		t.Fatal("expected error for unknown config field, got nil")
	}
}
