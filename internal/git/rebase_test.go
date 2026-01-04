package git_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/testhelper"
)

func TestCommitsForRebase_EmptyBase(t *testing.T) {
	runner := &testhelper.FakeRunner{}
	_, err := git.CommitsForRebase(context.Background(), runner, "")
	if err == nil {
		t.Fatal("expected error for empty base, got nil")
	}
}

func TestCommitsForRebase_NoCommits(t *testing.T) {
	runner := &testhelper.FakeRunner{Outputs: []string{""}}
	entries, err := git.CommitsForRebase(context.Background(), runner, "HEAD~3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestCommitsForRebase_WithCommits(t *testing.T) {
	raw := "abc1234\x1ffeat: first\nbbb5678\x1ffix: second\n"
	runner := &testhelper.FakeRunner{Outputs: []string{raw}}
	entries, err := git.CommitsForRebase(context.Background(), runner, "HEAD~2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Hash != "abc1234" {
		t.Errorf("entries[0].Hash = %q, want %q", entries[0].Hash, "abc1234")
	}
	if entries[0].Subject != "feat: first" {
		t.Errorf("entries[0].Subject = %q, want %q", entries[0].Subject, "feat: first")
	}
	if entries[0].Action != git.ActionPick {
		t.Errorf("entries[0].Action = %q, want %q", entries[0].Action, git.ActionPick)
	}
	if entries[1].Hash != "bbb5678" {
		t.Errorf("entries[1].Hash = %q, want %q", entries[1].Hash, "bbb5678")
	}
}

func TestCommitsForRebase_GitError(t *testing.T) {
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{fmt.Errorf("bad revision")},
	}
	_, err := git.CommitsForRebase(context.Background(), runner, "HEAD~5")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCommitsForRebase_UsesReverseFlag(t *testing.T) {
	runner := &testhelper.FakeRunner{Outputs: []string{""}}
	_, _ = git.CommitsForRebase(context.Background(), runner, "HEAD~3")
	testhelper.MustHaveCall(t, runner, "log", "--reverse")
}

func TestCommitsForRebase_UsesBaseRange(t *testing.T) {
	runner := &testhelper.FakeRunner{Outputs: []string{""}}
	_, _ = git.CommitsForRebase(context.Background(), runner, "HEAD~3")
	testhelper.MustHaveCall(t, runner, "HEAD~3..HEAD")
}

func TestFormatTodo_Empty(t *testing.T) {
	out := git.FormatTodo(nil)
	if out != "" {
		t.Errorf("FormatTodo(nil) = %q, want %q", out, "")
	}
}

func TestFormatTodo_Single(t *testing.T) {
	entries := []git.RebaseTodoEntry{
		{Action: git.ActionPick, Hash: "abc1234", Subject: "feat: add feature"},
	}
	out := git.FormatTodo(entries)
	if !strings.Contains(out, "pick abc1234 feat: add feature") {
		t.Errorf("FormatTodo output = %q, want containing %q", out, "pick abc1234 feat: add feature")
	}
}

func TestFormatTodo_MultipleActions(t *testing.T) {
	entries := []git.RebaseTodoEntry{
		{Action: git.ActionPick, Hash: "abc1234", Subject: "feat: first"},
		{Action: git.ActionSquash, Hash: "bbb5678", Subject: "fix: second"},
		{Action: git.ActionDrop, Hash: "ccc9999", Subject: "chore: third"},
	}
	out := git.FormatTodo(entries)
	if !strings.Contains(out, "pick abc1234") {
		t.Errorf("FormatTodo missing pick line; output = %q", out)
	}
	if !strings.Contains(out, "squash bbb5678") {
		t.Errorf("FormatTodo missing squash line; output = %q", out)
	}
	if !strings.Contains(out, "drop ccc9999") {
		t.Errorf("FormatTodo missing drop line; output = %q", out)
	}
}

func TestNextAction_CyclesThrough(t *testing.T) {
	actions := git.AllRebaseActions
	for i, a := range actions {
		next := git.NextAction(a)
		expected := actions[(i+1)%len(actions)]
		if next != expected {
			t.Errorf("NextAction(%q) = %q, want %q", a, next, expected)
		}
	}
}

func TestNextAction_Unknown(t *testing.T) {
	next := git.NextAction("unknown")
	if next != git.ActionPick {
		t.Errorf("NextAction(unknown) = %q, want %q", next, git.ActionPick)
	}
}

func TestExecuteRebaseInteractive_CallsRunWithEnv(t *testing.T) {
	runner := &testhelper.FakeRunner{Outputs: []string{""}}
	entries := []git.RebaseTodoEntry{
		{Action: git.ActionPick, Hash: "abc1234", Subject: "feat: first"},
	}
	err := git.ExecuteRebaseInteractive(context.Background(), runner, "HEAD~1", entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testhelper.MustHaveCall(t, runner, "rebase", "-i", "HEAD~1")
	testhelper.MustHaveEnv(t, runner, func() string {
		// Check the env key prefix
		calls := testhelper.NthCall(runner, 0)
		for _, e := range calls.Env {
			if strings.HasPrefix(e, "GIT_SEQUENCE_EDITOR=") {
				return e
			}
		}
		return ""
	}())
}

func TestExecuteRebaseInteractive_GitError(t *testing.T) {
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{fmt.Errorf("rebase failed")},
	}
	entries := []git.RebaseTodoEntry{
		{Action: git.ActionPick, Hash: "abc1234", Subject: "feat: first"},
	}
	err := git.ExecuteRebaseInteractive(context.Background(), runner, "HEAD~1", entries)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
