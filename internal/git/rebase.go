package git

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// RebaseAction represents an action in the interactive rebase todo list.
type RebaseAction string

// Rebase action constants for the interactive todo list.
const (
	ActionPick   RebaseAction = "pick"
	ActionReword RebaseAction = "reword"
	ActionEdit   RebaseAction = "edit"
	ActionSquash RebaseAction = "squash"
	ActionFixup  RebaseAction = "fixup"
	ActionDrop   RebaseAction = "drop"
)

// AllRebaseActions lists actions in cycle order.
var AllRebaseActions = []RebaseAction{
	ActionPick, ActionReword, ActionEdit, ActionSquash, ActionFixup, ActionDrop,
}

// RebaseTodoEntry is a single line in the interactive rebase todo list.
type RebaseTodoEntry struct {
	Action  RebaseAction
	Hash    string // short hash
	Subject string // commit subject
}

// CommitsForRebase returns commits between base and HEAD in rebase order
// (oldest first, as git rebase -i presents them).
func CommitsForRebase(ctx context.Context, r Runner, base string) ([]RebaseTodoEntry, error) {
	if base == "" {
		return nil, fmt.Errorf("rebase base revision is required")
	}
	// git log --reverse base..HEAD gives commits oldest-first between base and HEAD
	format := "--format=%h\x1f%s"
	out, err := r.Run(ctx, "log", "--reverse", format, base+"..HEAD")
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	return parseRebaseTodo(out), nil
}

// ExecuteRebaseInteractive writes the todo file and runs git rebase -i
// with GIT_SEQUENCE_EDITOR set to a script that replaces the editor file
// with our prepared todo content.
func ExecuteRebaseInteractive(ctx context.Context, r Runner, base string, entries []RebaseTodoEntry) error {
	todo := FormatTodo(entries)

	// Write todo to a temp file.
	f, err := os.CreateTemp("", "jig-rebase-todo-*")
	if err != nil {
		return fmt.Errorf("creating temp todo file: %w", err)
	}
	todoPath := f.Name()
	defer func() { _ = os.Remove(todoPath) }()

	if _, err = f.WriteString(todo); err != nil {
		_ = f.Close()
		return fmt.Errorf("writing todo file: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("closing todo file: %w", err)
	}

	// GIT_SEQUENCE_EDITOR copies our prepared todo over the git-provided file.
	// The editor script receives the git todo path as its first argument.
	editorScript := fmt.Sprintf("cp %q \"$1\"", todoPath)
	env := []string{"GIT_SEQUENCE_EDITOR=sh -c '" + editorScript + "'"}

	_, err = r.RunWithEnv(ctx, env, "rebase", "-i", base)
	if err != nil {
		return fmt.Errorf("git rebase -i: %w", err)
	}
	return nil
}

// FormatTodo serializes entries into a git rebase todo file format.
func FormatTodo(entries []RebaseTodoEntry) string {
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(string(e.Action))
		sb.WriteString(" ")
		sb.WriteString(e.Hash)
		sb.WriteString(" ")
		sb.WriteString(e.Subject)
		sb.WriteString("\n")
	}
	return sb.String()
}

// NextAction cycles to the next action in the canonical order.
func NextAction(a RebaseAction) RebaseAction {
	for i, action := range AllRebaseActions {
		if action == a {
			return AllRebaseActions[(i+1)%len(AllRebaseActions)]
		}
	}
	return ActionPick
}

// shortActionAliases maps single-char aliases to full action names.
var shortActionAliases = map[string]RebaseAction{
	"p": ActionPick,
	"r": ActionReword,
	"e": ActionEdit,
	"s": ActionSquash,
	"f": ActionFixup,
	"d": ActionDrop,
}

// longActions maps full action names for validation.
var longActions = map[string]RebaseAction{
	"pick":   ActionPick,
	"reword": ActionReword,
	"edit":   ActionEdit,
	"squash": ActionSquash,
	"fixup":  ActionFixup,
	"drop":   ActionDrop,
}

// ParseNativeTodo parses git's native rebase todo format (action hash subject).
// It skips comment lines (starting with #), blank lines, and malformed lines.
// Short aliases (p, r, e, s, f, d) are normalized to full action names.
func ParseNativeTodo(raw string) []RebaseTodoEntry {
	if raw == "" {
		return nil
	}
	var entries []RebaseTodoEntry
	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Need at least 3 fields: action hash subject
		action, rest, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		hash, subject, ok := strings.Cut(rest, " ")
		if !ok {
			continue
		}
		// Resolve action (short alias or full name)
		var resolved RebaseAction
		if a, found := shortActionAliases[action]; found {
			resolved = a
		} else if a, found := longActions[action]; found {
			resolved = a
		} else {
			continue // unknown action, skip
		}
		entries = append(entries, RebaseTodoEntry{
			Action:  resolved,
			Hash:    hash,
			Subject: subject,
		})
	}
	return entries
}

// parseRebaseTodo parses git log output into RebaseTodoEntry slice.
func parseRebaseTodo(raw string) []RebaseTodoEntry {
	if raw == "" {
		return nil
	}
	var entries []RebaseTodoEntry
	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		hash, subject, _ := strings.Cut(line, "\x1f")
		if hash == "" {
			continue
		}
		entries = append(entries, RebaseTodoEntry{
			Action:  ActionPick,
			Hash:    strings.TrimSpace(hash),
			Subject: strings.TrimSpace(subject),
		})
	}
	return entries
}
