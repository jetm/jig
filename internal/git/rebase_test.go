package git_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jetm/jig/internal/git"
	"github.com/jetm/jig/internal/testhelper"
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

func TestParseNativeTodo_StandardPickLines(t *testing.T) {
	t.Parallel()
	input := "pick abc1234 Add feature X\npick bbb5678 Fix bug Y\n"
	entries := git.ParseNativeTodo(input)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Action != git.ActionPick {
		t.Errorf("entries[0].Action = %q, want %q", entries[0].Action, git.ActionPick)
	}
	if entries[0].Hash != "abc1234" {
		t.Errorf("entries[0].Hash = %q, want %q", entries[0].Hash, "abc1234")
	}
	if entries[0].Subject != "Add feature X" {
		t.Errorf("entries[0].Subject = %q, want %q", entries[0].Subject, "Add feature X")
	}
	if entries[1].Hash != "bbb5678" {
		t.Errorf("entries[1].Hash = %q, want %q", entries[1].Hash, "bbb5678")
	}
	if entries[1].Subject != "Fix bug Y" {
		t.Errorf("entries[1].Subject = %q, want %q", entries[1].Subject, "Fix bug Y")
	}
}

func TestParseNativeTodo_MixedActions(t *testing.T) {
	t.Parallel()
	input := "pick abc1234 first\nreword bbb5678 second\nedit ccc9999 third\nsquash ddd0000 fourth\nfixup eee1111 fifth\ndrop fff2222 sixth\n"
	entries := git.ParseNativeTodo(input)
	if len(entries) != 6 {
		t.Fatalf("expected 6 entries, got %d", len(entries))
	}
	expected := []git.RebaseAction{
		git.ActionPick, git.ActionReword, git.ActionEdit,
		git.ActionSquash, git.ActionFixup, git.ActionDrop,
	}
	for i, want := range expected {
		if entries[i].Action != want {
			t.Errorf("entries[%d].Action = %q, want %q", i, entries[i].Action, want)
		}
	}
}

func TestParseNativeTodo_ShortAliases(t *testing.T) {
	t.Parallel()
	input := "p abc1234 first\nr bbb5678 second\ne ccc9999 third\ns ddd0000 fourth\nf eee1111 fifth\nd fff2222 sixth\n"
	entries := git.ParseNativeTodo(input)
	if len(entries) != 6 {
		t.Fatalf("expected 6 entries, got %d", len(entries))
	}
	expected := []git.RebaseAction{
		git.ActionPick, git.ActionReword, git.ActionEdit,
		git.ActionSquash, git.ActionFixup, git.ActionDrop,
	}
	for i, want := range expected {
		if entries[i].Action != want {
			t.Errorf("entries[%d].Action = %q, want %q", i, entries[i].Action, want)
		}
	}
}

func TestParseNativeTodo_SkipsCommentsBlanksMalformed(t *testing.T) {
	t.Parallel()
	input := "# This is a comment\n\npick abc1234 valid entry\n   \nbadline\n# another comment\npick bbb5678 also valid\nonly two fields\n"
	entries := git.ParseNativeTodo(input)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Hash != "abc1234" {
		t.Errorf("entries[0].Hash = %q, want %q", entries[0].Hash, "abc1234")
	}
	if entries[1].Hash != "bbb5678" {
		t.Errorf("entries[1].Hash = %q, want %q", entries[1].Hash, "bbb5678")
	}
}

func TestParseNativeTodo_RoundTrip(t *testing.T) {
	t.Parallel()
	original := []git.RebaseTodoEntry{
		{Action: git.ActionPick, Hash: "abc1234", Subject: "feat: first"},
		{Action: git.ActionSquash, Hash: "bbb5678", Subject: "fix: second"},
		{Action: git.ActionDrop, Hash: "ccc9999", Subject: "chore: third"},
	}
	formatted := git.FormatTodo(original)
	parsed := git.ParseNativeTodo(formatted)
	if len(parsed) != len(original) {
		t.Fatalf("round-trip: expected %d entries, got %d", len(original), len(parsed))
	}
	for i := range original {
		if parsed[i].Action != original[i].Action {
			t.Errorf("round-trip[%d].Action = %q, want %q", i, parsed[i].Action, original[i].Action)
		}
		if parsed[i].Hash != original[i].Hash {
			t.Errorf("round-trip[%d].Hash = %q, want %q", i, parsed[i].Hash, original[i].Hash)
		}
		if parsed[i].Subject != original[i].Subject {
			t.Errorf("round-trip[%d].Subject = %q, want %q", i, parsed[i].Subject, original[i].Subject)
		}
	}
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
