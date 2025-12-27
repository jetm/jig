package diff_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/jetm/gti/internal/diff"
)

func TestDeltaRenderer_ProducesOutput(t *testing.T) {
	deltaPath, err := exec.LookPath("delta")
	if err != nil {
		t.Skip("delta not in PATH")
	}

	fixture, err := os.ReadFile("testdata/sample.diff")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	r := diff.NewDeltaRenderer(deltaPath, 120)
	got, err := r.Render(string(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == "" {
		t.Error("DeltaRenderer produced empty output for valid diff")
	}
}

func TestDeltaRenderer_WidthFlag(t *testing.T) {
	deltaPath, err := exec.LookPath("delta")
	if err != nil {
		t.Skip("delta not in PATH")
	}

	fixture, err := os.ReadFile("testdata/sample.diff")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	// Using a narrow width should still produce output.
	r := diff.NewDeltaRenderer(deltaPath, 40)
	got, err := r.Render(string(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == "" {
		t.Error("DeltaRenderer with narrow width produced empty output")
	}
}

func TestDeltaRenderer_NonZeroExit(t *testing.T) {
	// Use a binary that will fail when given diff input.
	_, err := exec.LookPath("false")
	if err != nil {
		t.Skip("false not in PATH")
	}

	r := diff.NewDeltaRenderer("false", 120)
	_, err = r.Render("some diff content")
	if err == nil {
		t.Error("DeltaRenderer did not return error on non-zero exit")
	}
}

func TestDeltaRenderer_EmptyInput(t *testing.T) {
	deltaPath, err := exec.LookPath("delta")
	if err != nil {
		t.Skip("delta not in PATH")
	}

	r := diff.NewDeltaRenderer(deltaPath, 120)
	got, err := r.Render("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != "" {
		t.Errorf("DeltaRenderer empty input: got %q, want %q", got, "")
	}
}
