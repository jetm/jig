// Package diff provides renderers that transform raw unified diff text into
// styled terminal output with syntax highlighting.
package diff

import (
	"os/exec"

	"github.com/jetm/jig/internal/config"
)

// Renderer transforms raw unified diff text into styled terminal output.
type Renderer interface {
	Render(rawDiff string) (string, error)
}

// newChromaFunc is the constructor used by Chain. It is a variable so tests
// can replace it to simulate chroma initialization failures.
var newChromaFunc = NewChromaRenderer

// Chain resolves the correct Renderer based on configuration.
// If Config.DeltaPath is non-empty and the binary exists, it returns a
// DeltaRenderer. If DeltaPath is empty, it tries to auto-detect "delta" in
// $PATH. Otherwise it returns a ChromaRenderer. PlainRenderer is the
// defensive fallback if ChromaRenderer construction fails.
func Chain(cfg config.Config) Renderer {
	if cfg.DeltaPath != "" {
		if _, err := exec.LookPath(cfg.DeltaPath); err == nil {
			return NewDeltaRenderer(cfg.DeltaPath)
		}
	}

	if cfg.DeltaPath == "" {
		if path, err := exec.LookPath("delta"); err == nil {
			return NewDeltaRenderer(path)
		}
	}

	cr, err := newChromaFunc()
	if err != nil {
		return &PlainRenderer{}
	}
	return cr
}
