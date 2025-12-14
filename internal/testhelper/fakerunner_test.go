package testhelper_test

import (
	"context"
	"testing"

	"github.com/jetm/gti/internal/testhelper"
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

func TestFakeRunner_NthCall(t *testing.T) {
	f := &testhelper.FakeRunner{}
	_, _ = f.Run(context.Background(), "first")
	_, _ = f.Run(context.Background(), "second")
	c := testhelper.NthCall(f, 1)
	if len(c.Args) != 1 || c.Args[0] != "second" {
		t.Errorf("expected Args=[second], got %v", c.Args)
	}
}
