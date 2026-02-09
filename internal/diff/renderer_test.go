package diff_test

import (
	"os/exec"
	"testing"

	"github.com/jetm/gti/internal/config"
	"github.com/jetm/gti/internal/diff"
)

func TestChain_ValidDeltaPath(t *testing.T) {
	// Use "sh" as a stand-in for a valid executable in PATH.
	cfg := config.Config{DeltaPath: "sh"}
	r := diff.Chain(cfg)

	if _, ok := r.(*diff.DeltaRenderer); !ok {
		t.Errorf("Chain with valid delta path: got %T, want *diff.DeltaRenderer", r)
	}
}

func TestChain_EmptyDeltaPath_NoDeltaInPath(t *testing.T) {
	// Override PATH to ensure delta is not found.
	t.Setenv("PATH", t.TempDir())
	cfg := config.Config{DeltaPath: ""}
	r := diff.Chain(cfg)

	if _, ok := r.(*diff.ChromaRenderer); !ok {
		t.Errorf("Chain with empty delta path and no delta in PATH: got %T, want *diff.ChromaRenderer", r)
	}
}

func TestChain_InvalidDeltaPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	cfg := config.Config{DeltaPath: "/nonexistent/binary/path"}
	r := diff.Chain(cfg)

	if _, ok := r.(*diff.ChromaRenderer); !ok {
		t.Errorf("Chain with invalid delta path: got %T, want *diff.ChromaRenderer", r)
	}
}

func TestChain_AutoDetectsDeltaInPath(t *testing.T) {
	_, err := exec.LookPath("delta")
	if err != nil {
		t.Skip("delta not in PATH")
	}

	// Empty DeltaPath should auto-detect delta.
	cfg := config.Config{DeltaPath: ""}
	r := diff.Chain(cfg)

	if _, ok := r.(*diff.DeltaRenderer); !ok {
		t.Errorf("Chain auto-detect: got %T, want *diff.DeltaRenderer", r)
	}
}

func TestChain_ExplicitDeltaPathTakesPrecedence(t *testing.T) {
	// Use "sh" as a stand-in for a valid executable.
	cfg := config.Config{DeltaPath: "sh"}
	r := diff.Chain(cfg)

	if _, ok := r.(*diff.DeltaRenderer); !ok {
		t.Errorf("Chain with explicit DeltaPath: got %T, want *diff.DeltaRenderer", r)
	}
}
