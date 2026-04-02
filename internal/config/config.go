// Package config provides jig configuration loading and defaults.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds all jig configuration values.
type Config struct {
	Theme       string
	CopyCmd     string
	DeltaPath   string
	LogDepth    int
	DiffContext int
	LogLevel    string

	// Phase 10 fields
	DiffRenderer      string
	LogCommitLimit    int
	RebaseDefaultBase string
	UITheme           string
	ShowDiffPanel     bool

	// Phase 11 fields
	PanelRatio      int
	SoftWrap        bool
	ShowLineNumbers bool

	// Commit fields
	CommitCmd           string
	CommitTitleOnlyFlag string
}

// NewDefault returns a Config with default values.
func NewDefault() Config {
	return Config{
		Theme:       "dark",
		CopyCmd:     "wl-copy",
		DeltaPath:   "",
		LogDepth:    30,
		DiffContext: 3,
		LogLevel:    "warn",

		DiffRenderer:      "chroma",
		LogCommitLimit:    50,
		RebaseDefaultBase: "HEAD~10",
		UITheme:           "dark",
		ShowDiffPanel:     true,
		PanelRatio:        40,
		SoftWrap:          true,
		ShowLineNumbers:   true,

		CommitCmd:           "git commit",
		CommitTitleOnlyFlag: "",
	}
}

// fileConfig is the intermediate struct used to parse the YAML config file.
// Using a separate struct keeps yaml tags out of the public Config API.
type fileConfig struct {
	Diff struct {
		Renderer string `yaml:"renderer"`
	} `yaml:"diff"`
	Log struct {
		CommitLimit int `yaml:"commitLimit"`
	} `yaml:"log"`
	Rebase struct {
		DefaultBase string `yaml:"defaultBase"`
	} `yaml:"rebase"`
	UI struct {
		Theme           string `yaml:"theme"`
		ShowDiffPanel   *bool  `yaml:"showDiffPanel"`
		PanelRatio      int    `yaml:"panelRatio"`
		SoftWrap        *bool  `yaml:"softWrap"`
		ShowLineNumbers *bool  `yaml:"showLineNumbers"`
	} `yaml:"ui"`
	Commit struct {
		Command       string `yaml:"command"`
		TitleOnlyFlag string `yaml:"titleOnlyFlag"`
	} `yaml:"commit"`
}

// configPaths returns candidate config file paths in preference order.
// It respects $XDG_CONFIG_HOME if set.
func configPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		xdgConfigHome = filepath.Join(home, ".config")
	}

	return []string{
		filepath.Join(xdgConfigHome, "jig", "config.yaml"),
		filepath.Join(home, ".jig.yaml"),
	}
}

// Load returns a Config populated from the first config file found,
// then applies environment variable overrides.
// If no config file is found, defaults are used.
func Load() (Config, error) {
	cfg := NewDefault()

	if err := applyFile(&cfg); err != nil {
		return Config{}, err
	}

	if err := applyEnv(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// applyFile reads the first existing config file and overlays its values onto cfg.
func applyFile(cfg *Config) error {
	for _, path := range configPaths() {
		data, err := os.ReadFile(path) //nolint:gosec // path is derived from user home
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("reading config file %s: %w", path, err)
		}

		var fc fileConfig
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(&fc); err != nil {
			return fmt.Errorf("parsing config file %s: %w", path, err)
		}

		if fc.Diff.Renderer != "" {
			cfg.DiffRenderer = fc.Diff.Renderer
		}
		if fc.Log.CommitLimit != 0 {
			cfg.LogCommitLimit = fc.Log.CommitLimit
		}
		if fc.Rebase.DefaultBase != "" {
			cfg.RebaseDefaultBase = fc.Rebase.DefaultBase
		}
		if fc.UI.Theme != "" {
			cfg.UITheme = fc.UI.Theme
		}
		if fc.UI.ShowDiffPanel != nil {
			cfg.ShowDiffPanel = *fc.UI.ShowDiffPanel
		}
		if fc.UI.PanelRatio != 0 {
			if fc.UI.PanelRatio < 20 || fc.UI.PanelRatio > 80 {
				return fmt.Errorf("config panelRatio %d out of range [20, 80]", fc.UI.PanelRatio)
			}
			cfg.PanelRatio = fc.UI.PanelRatio
		}
		if fc.UI.SoftWrap != nil {
			cfg.SoftWrap = *fc.UI.SoftWrap
		}
		if fc.UI.ShowLineNumbers != nil {
			cfg.ShowLineNumbers = *fc.UI.ShowLineNumbers
		}
		if fc.Commit.Command != "" {
			cfg.CommitCmd = fc.Commit.Command
		}
		if fc.Commit.TitleOnlyFlag != "" {
			cfg.CommitTitleOnlyFlag = fc.Commit.TitleOnlyFlag
		}

		return nil
	}

	return nil
}

// applyEnv overlays environment variable values onto cfg.
func applyEnv(cfg *Config) error {
	if v := os.Getenv("JIG_DIFF_RENDERER"); v != "" {
		cfg.DiffRenderer = v
	}
	if v := os.Getenv("JIG_LOG_COMMIT_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid JIG_LOG_COMMIT_LIMIT %q: %w", v, err)
		}
		cfg.LogCommitLimit = n
	}
	if v := os.Getenv("JIG_REBASE_DEFAULT_BASE"); v != "" {
		cfg.RebaseDefaultBase = v
	}
	if v := os.Getenv("JIG_UI_THEME"); v != "" {
		cfg.UITheme = v
	}
	if v := os.Getenv("JIG_SHOW_DIFF_PANEL"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid JIG_SHOW_DIFF_PANEL %q: %w", v, err)
		}
		cfg.ShowDiffPanel = b
	}
	if v := os.Getenv("JIG_PANEL_RATIO"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid JIG_PANEL_RATIO %q: %w", v, err)
		}
		if n < 20 || n > 80 {
			return fmt.Errorf("JIG_PANEL_RATIO %d out of range [20, 80]", n)
		}
		cfg.PanelRatio = n
	}
	if v := os.Getenv("JIG_SOFT_WRAP"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid JIG_SOFT_WRAP %q: %w", v, err)
		}
		cfg.SoftWrap = b
	}
	if v := os.Getenv("JIG_SHOW_LINE_NUMBERS"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid JIG_SHOW_LINE_NUMBERS %q: %w", v, err)
		}
		cfg.ShowLineNumbers = b
	}
	if v := os.Getenv("JIG_COMMIT_COMMAND"); v != "" {
		cfg.CommitCmd = v
	}
	if v := os.Getenv("JIG_COMMIT_TITLE_ONLY_FLAG"); v != "" {
		cfg.CommitTitleOnlyFlag = v
	}
	return nil
}

// saveConfig is the intermediate struct used to marshal the YAML config file.
type saveConfig struct {
	Diff struct {
		Renderer string `yaml:"renderer"`
	} `yaml:"diff"`
	Log struct {
		CommitLimit int `yaml:"commitLimit"`
	} `yaml:"log"`
	Rebase struct {
		DefaultBase string `yaml:"defaultBase"`
	} `yaml:"rebase"`
	UI struct {
		Theme           string `yaml:"theme"`
		ShowDiffPanel   bool   `yaml:"showDiffPanel"`
		PanelRatio      int    `yaml:"panelRatio"`
		SoftWrap        bool   `yaml:"softWrap"`
		ShowLineNumbers bool   `yaml:"showLineNumbers"`
	} `yaml:"ui"`
	Commit struct {
		Command       string `yaml:"command"`
		TitleOnlyFlag string `yaml:"titleOnlyFlag"`
	} `yaml:"commit"`
}

// Save writes the given Config to the primary config file path.
// It creates parent directories if they do not exist.
func Save(cfg Config) error {
	paths := configPaths()
	if len(paths) == 0 {
		return fmt.Errorf("cannot determine config path")
	}
	path := paths[0] // primary XDG path

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating config directory %s: %w", dir, err)
	}

	var sc saveConfig
	sc.Diff.Renderer = cfg.DiffRenderer
	sc.Log.CommitLimit = cfg.LogCommitLimit
	sc.Rebase.DefaultBase = cfg.RebaseDefaultBase
	sc.UI.Theme = cfg.UITheme
	sc.UI.ShowDiffPanel = cfg.ShowDiffPanel
	sc.UI.PanelRatio = cfg.PanelRatio
	sc.UI.SoftWrap = cfg.SoftWrap
	sc.UI.ShowLineNumbers = cfg.ShowLineNumbers
	sc.Commit.Command = cfg.CommitCmd
	sc.Commit.TitleOnlyFlag = cfg.CommitTitleOnlyFlag

	data, err := yaml.Marshal(&sc)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config file %s: %w", path, err)
	}

	return nil
}
