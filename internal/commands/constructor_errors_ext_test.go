package commands_test

// Tests for Task 7.4: constructor error propagation (external test package).
// Covers NewLogModel and NewRebaseInteractiveModel.

import (
	"context"
	"fmt"
	"testing"

	"github.com/jetm/jig/internal/commands"
	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/testhelper"
)

// TestNewLogModel_RunnerError verifies that a runner error on git log
// causes NewLogModel to return a non-nil error.
func TestNewLogModel_RunnerError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{fmt.Errorf("not a git repository")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewLogModel(context.Background(), runner, cfg, renderer, "")
	if err == nil {
		t.Fatal("expected error from NewLogModel when runner fails, got nil")
	}
	if m != nil {
		t.Error("expected nil model when constructor fails, got non-nil")
	}
}

// TestNewRebaseInteractiveModel_RunnerError verifies that a runner error on git log
// (CommitsForRebase) causes NewRebaseInteractiveModel to return a non-nil error.
func TestNewRebaseInteractiveModel_RunnerError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{fmt.Errorf("not a git repository")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "HEAD~5", "")
	if err == nil {
		t.Fatal("expected error from NewRebaseInteractiveModel when runner fails, got nil")
	}
	if m != nil {
		t.Error("expected nil model when constructor fails, got non-nil")
	}
}
