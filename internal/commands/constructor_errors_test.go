package commands

// Tests for Task 7.4: constructor error propagation.
// Each test confirms that when the primary git call fails, the constructor
// returns a non-nil error (not a nil model or a panic).

import (
	"context"
	"fmt"
	"testing"

	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/testhelper"
)

// TestNewAddModel_ListError verifies that a runner error on the first git call
// (diff --name-status) causes NewAddModel to return a non-nil error.
func TestNewAddModel_ListError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{fmt.Errorf("permission denied")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	if err == nil {
		t.Fatal("expected error from NewAddModel when runner fails, got nil")
	}
	if m != nil {
		t.Error("expected nil model when constructor fails, got non-nil")
	}
}

// TestNewCheckoutModel_ListError verifies that a runner error on diff --name-status
// causes NewCheckoutModel to return a non-nil error.
func TestNewCheckoutModel_ListError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{fmt.Errorf("permission denied")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewCheckoutModel(context.Background(), runner, cfg, renderer)
	if err == nil {
		t.Fatal("expected error from NewCheckoutModel when runner fails, got nil")
	}
	if m != nil {
		t.Error("expected nil model when constructor fails, got non-nil")
	}
}

// TestNewResetModel_ListError verifies that a runner error on diff --cached --name-status
// causes NewResetModel to return a non-nil error.
func TestNewResetModel_ListError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{fmt.Errorf("permission denied")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewResetModel(context.Background(), runner, cfg, renderer)
	if err == nil {
		t.Fatal("expected error from NewResetModel when runner fails, got nil")
	}
	if m != nil {
		t.Error("expected nil model when constructor fails, got non-nil")
	}
}

// TestNewDiffModel_RunnerError verifies that a runner error on git diff
// causes NewDiffModel to return a non-nil error (non-pager mode only).
func TestNewDiffModel_RunnerError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{fmt.Errorf("not a git repository")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	// rawInput="" triggers the runner call path
	m, err := NewDiffModel(context.Background(), runner, cfg, renderer, "", false, "")
	if err == nil {
		t.Fatal("expected error from NewDiffModel when runner fails, got nil")
	}
	if m != nil {
		t.Error("expected nil model when constructor fails, got non-nil")
	}
}

// TestNewHunkAddModel_RunnerError verifies that a runner error on git diff
// causes NewHunkAddModel to return a non-nil error.
func TestNewHunkAddModel_RunnerError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{fmt.Errorf("not a git repository")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkAddModel(context.Background(), runner, cfg, renderer)
	if err == nil {
		t.Fatal("expected error from NewHunkAddModel when runner fails, got nil")
	}
	if m != nil {
		t.Error("expected nil model when constructor fails, got non-nil")
	}
}

// TestNewHunkResetModel_RunnerError verifies that a runner error on git diff --cached
// causes NewHunkResetModel to return a non-nil error.
func TestNewHunkResetModel_RunnerError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{fmt.Errorf("not a git repository")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkResetModel(context.Background(), runner, cfg, renderer)
	if err == nil {
		t.Fatal("expected error from NewHunkResetModel when runner fails, got nil")
	}
	if m != nil {
		t.Error("expected nil model when constructor fails, got non-nil")
	}
}

// TestNewHunkCheckoutModel_RunnerError verifies that a runner error on git diff
// causes NewHunkCheckoutModel to return a non-nil error.
func TestNewHunkCheckoutModel_RunnerError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{fmt.Errorf("not a git repository")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkCheckoutModel(context.Background(), runner, cfg, renderer)
	if err == nil {
		t.Fatal("expected error from NewHunkCheckoutModel when runner fails, got nil")
	}
	if m != nil {
		t.Error("expected nil model when constructor fails, got non-nil")
	}
}
