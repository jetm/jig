package config

// Config holds all gti configuration values.
type Config struct {
	Theme       string
	CopyCmd     string
	DeltaPath   string
	LogDepth    int
	DiffContext int
	LogLevel    string
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
	}
}
