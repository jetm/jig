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

func TestChain_EmptyDeltaPath(t *testing.T) {
	cfg := config.Config{DeltaPath: ""}
	r := diff.Chain(cfg)

	if _, ok := r.(*diff.ChromaRenderer); !ok {
		t.Errorf("Chain with empty delta path: got %T, want *diff.ChromaRenderer", r)
	}
}

func TestChain_InvalidDeltaPath(t *testing.T) {
	cfg := config.Config{DeltaPath: "/nonexistent/binary/path"}
	r := diff.Chain(cfg)

	if _, ok := r.(*diff.ChromaRenderer); !ok {
		t.Errorf("Chain with invalid delta path: got %T, want *diff.ChromaRenderer", r)
	}
}

func TestChain_DeltaInPath(t *testing.T) {
	_, err := exec.LookPath("delta")
	if err != nil {
		t.Skip("delta not in PATH")
	}

	cfg := config.Config{DeltaPath: "delta"}
	r := diff.Chain(cfg)

	if _, ok := r.(*diff.DeltaRenderer); !ok {
		t.Errorf("Chain with delta in PATH: got %T, want *diff.DeltaRenderer", r)
	}
}
