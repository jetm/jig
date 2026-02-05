package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// editedDiffPath returns the fixed path for the hunk-edit temp file.
// The name addp-hunk-edit.diff matches git's own add -p for editor plugin compatibility.
func editedDiffPath() string {
	return filepath.Join(os.TempDir(), "addp-hunk-edit.diff")
}

// EditDiffMsg is returned after the editor exits, carrying the path of the
// edited file and the original diff content for comparison.
type EditDiffMsg struct {
	EditedPath   string
	OriginalDiff string
	Err          error
}

// EditDiff writes rawDiff to $TMPDIR/addp-hunk-edit.diff and returns a
// tea.Cmd that opens the file in the user's editor via tea.ExecProcess.
// The callback sends an EditDiffMsg when the editor exits.
// If the editor cannot be resolved (no GIT_EDITOR, core.editor, VISUAL, or EDITOR),
// EditDiff returns nil - callers must check the editor before calling this.
func EditDiff(ctx context.Context, runner Runner, rawDiff string) tea.Cmd {
	path := editedDiffPath()
	if err := os.WriteFile(path, []byte(rawDiff), 0o600); err != nil {
		return func() tea.Msg {
			return EditDiffMsg{Err: fmt.Errorf("writing diff to %s: %w", path, err)}
		}
	}

	editor := ResolveEditor(ctx, runner)
	// Split editor string to handle "code --wait" style values.
	parts := strings.Fields(editor)
	args := make([]string, len(parts)-1, len(parts))
	copy(args, parts[1:])
	args = append(args, path)
	cmd := exec.Command(parts[0], args...) //nolint:gosec // editor is user-configured

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return EditDiffMsg{
			EditedPath:   path,
			OriginalDiff: rawDiff,
			Err:          err,
		}
	})
}

// ApplyEditedDiff reads the file at editedPath, compares its content to
// originalDiff, and if different applies it via `git apply --cached`.
// Returns nil if the content is unchanged (no-op), or an error if apply fails.
func ApplyEditedDiff(ctx context.Context, runner Runner, originalDiff, editedPath string) error {
	edited, err := os.ReadFile(editedPath) //nolint:gosec // path is controlled by our code
	if err != nil {
		return fmt.Errorf("reading edited diff: %w", err)
	}

	if string(edited) == originalDiff {
		// Unchanged - nothing to do.
		return nil
	}

	_, err = runner.RunWithStdin(ctx, string(edited), "apply", "--cached")
	if err != nil {
		return fmt.Errorf("git apply --cached: %w", err)
	}
	return nil
}
