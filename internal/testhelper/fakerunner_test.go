package testhelper_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jetm/jig/internal/testhelper"
)

func TestFakeRunner_Run_RecordsArgs(t *testing.T) {
	f := &testhelper.FakeRunner{}
	_, _ = f.Run(context.Background(), "log")
	if got := testhelper.NthCall(f, 0).Args; len(got) != 1 || got[0] != "log" {
		t.Errorf("expected Args=[log], got %v", got)
	}
}

func TestFakeRunner_FIFO(t *testing.T) {
	f := &testhelper.FakeRunner{
		Outputs: []string{"first", "second"},
	}
	out1, _ := f.Run(context.Background(), "cmd")
	out2, _ := f.Run(context.Background(), "cmd")
	if out1 != "first" {
		t.Errorf("expected first, got %q", out1)
	}
	if out2 != "second" {
		t.Errorf("expected second, got %q", out2)
	}
}

func TestFakeRunner_RunWithEnv(t *testing.T) {
	f := &testhelper.FakeRunner{}
	env := []string{"FOO=bar"}
	_, _ = f.RunWithEnv(context.Background(), env, "status")
	c := testhelper.NthCall(f, 0)
	if len(c.Env) != 1 || c.Env[0] != "FOO=bar" {
		t.Errorf("expected Env=[FOO=bar], got %v", c.Env)
	}
}

func TestFakeRunner_RunWithStdin(t *testing.T) {
	f := &testhelper.FakeRunner{}
	_, _ = f.RunWithStdin(context.Background(), "hello stdin", "commit", "--file=-")
	c := testhelper.NthCall(f, 0)
	if c.Stdin != "hello stdin" {
		t.Errorf("expected Stdin=%q, got %q", "hello stdin", c.Stdin)
	}
}

func TestFakeRunner_MustHaveCall_Match(t *testing.T) {
	f := &testhelper.FakeRunner{}
	_, _ = f.Run(context.Background(), "add", "file.go")
	testhelper.MustHaveCall(t, f, "add", "file.go")
}

func TestFakeRunner_MustHaveNoCall_Empty(t *testing.T) {
	f := &testhelper.FakeRunner{}
	testhelper.MustHaveNoCall(t, f)
}

func TestFakeRunner_CallCount(t *testing.T) {
	f := &testhelper.FakeRunner{}
	_, _ = f.Run(context.Background(), "a")
	_, _ = f.Run(context.Background(), "b")
	_, _ = f.Run(context.Background(), "c")
	if got := testhelper.CallCount(f); got != 3 {
		t.Errorf("expected 3 calls, got %d", got)
	}
}

func TestFakeRunner_FIFO_WithErrors(t *testing.T) {
	err1 := fmt.Errorf("first error")
	f := &testhelper.FakeRunner{
		Outputs: []string{"out1"},
		Errors:  []error{err1},
	}
	out, err := f.Run(context.Background(), "cmd")
	if out != "out1" {
		t.Errorf("expected out1, got %q", out)
	}
	if err != err1 {
		t.Errorf("expected err1, got %v", err)
	}
}

func TestFakeRunner_MustHaveEnv(t *testing.T) {
	f := &testhelper.FakeRunner{}
	_, _ = f.RunWithEnv(context.Background(), []string{"KEY=VAL"}, "status")
	testhelper.MustHaveEnv(t, f, "KEY=VAL")
}

func TestFakeRunner_MustHaveStdin(t *testing.T) {
	f := &testhelper.FakeRunner{}
	_, _ = f.RunWithStdin(context.Background(), "patch data here", "apply")
	testhelper.MustHaveStdin(t, f, "patch data")
}

func TestFakeRunner_NthCall(t *testing.T) {
	f := &testhelper.FakeRunner{}
	_, _ = f.Run(context.Background(), "first")
	_, _ = f.Run(context.Background(), "second")
	c := testhelper.NthCall(f, 1)
	if len(c.Args) != 1 || c.Args[0] != "second" {
		t.Errorf("expected Args=[second], got %v", c.Args)
	}
}

func TestFakeRunner_MustHaveCall_SubsetMatch(t *testing.T) {
	f := &testhelper.FakeRunner{}
	_, _ = f.Run(context.Background(), "diff", "--cached", "--quiet")
	// Subset match: just "diff" should match
	testhelper.MustHaveCall(t, f, "diff")
}

func TestFakeRunner_MustHaveCall_NoMatch(t *testing.T) {
	f := &testhelper.FakeRunner{}
	_, _ = f.Run(context.Background(), "status")
	// Use a mock TB to verify MustHaveCall fails on no match
	mt := &mockTB{}
	testhelper.MustHaveCall(mt, f, "nonexistent")
	if !mt.failed {
		t.Error("expected MustHaveCall to fail on no match")
	}
}

func TestFakeRunner_MustHaveNoCall_WithCalls(t *testing.T) {
	f := &testhelper.FakeRunner{}
	_, _ = f.Run(context.Background(), "status")
	mt := &mockTB{}
	testhelper.MustHaveNoCall(mt, f)
	if !mt.failed {
		t.Error("expected MustHaveNoCall to fail when calls exist")
	}
}

func TestFakeRunner_MustHaveEnv_NoMatch(t *testing.T) {
	f := &testhelper.FakeRunner{}
	_, _ = f.Run(context.Background(), "status")
	mt := &mockTB{}
	testhelper.MustHaveEnv(mt, f, "MISSING=val")
	if !mt.failed {
		t.Error("expected MustHaveEnv to fail on no match")
	}
}

func TestFakeRunner_MustHaveStdin_NoMatch(t *testing.T) {
	f := &testhelper.FakeRunner{}
	_, _ = f.Run(context.Background(), "status")
	mt := &mockTB{}
	testhelper.MustHaveStdin(mt, f, "missing")
	if !mt.failed {
		t.Error("expected MustHaveStdin to fail on no match")
	}
}

// mockTB captures Errorf/Fatalf calls without actually failing the test.
type mockTB struct {
	testing.TB
	failed bool
}

func (m *mockTB) Helper()                   {}
func (m *mockTB) Errorf(_ string, _ ...any) { m.failed = true }
func (m *mockTB) Fatalf(_ string, _ ...any) { m.failed = true }

func TestFakeRunner_EmptyOutputs(t *testing.T) {
	f := &testhelper.FakeRunner{} // no outputs queued
	out, err := f.Run(context.Background(), "cmd")
	if out != "" {
		t.Errorf("expected empty output, got %q", out)
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestFakeRunner_RunAllowExitCode(t *testing.T) {
	f := &testhelper.FakeRunner{
		Outputs: []string{"output1"},
		Errors:  []error{fmt.Errorf("exit code 1")},
	}
	out, err := f.RunAllowExitCode(context.Background(), 1, "status")
	if out != "output1" {
		t.Errorf("expected 'output1', got %q", out)
	}
	if err == nil {
		t.Error("expected error, got nil")
	}
	testhelper.MustHaveCall(t, f, "status")
}
