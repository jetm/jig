// Package editor bridges git diff editing with an external text editor.
package editor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/jig/internal/git"
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
func EditDiff(ctx context.Context, runner git.Runner, rawDiff string) tea.Cmd {
	path := editedDiffPath()
	if err := os.WriteFile(path, []byte(rawDiff), 0o600); err != nil {
		return func() tea.Msg {
			return EditDiffMsg{Err: fmt.Errorf("writing diff to %s: %w", path, err)}
		}
	}

	editor := git.ResolveEditor(ctx, runner)
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

// ApplyEditedDiff reads the file at editedPath, sanitizes the content to
// handle common editor side effects (trailing whitespace stripping, comment
// lines), and applies it via `git apply --cached --recount`.
// Returns nil if the content is unchanged (no-op), or an error if apply fails.
func ApplyEditedDiff(ctx context.Context, runner git.Runner, originalDiff, editedPath string) error {
	edited, err := os.ReadFile(editedPath) //nolint:gosec // path is controlled by our code
	if err != nil {
		return fmt.Errorf("reading edited diff: %w", err)
	}

	sanitized := sanitizeEditedDiff(string(edited))

	if sanitized == sanitizeEditedDiff(originalDiff) {
		// Unchanged - nothing to do.
		return nil
	}

	_, err = runner.RunWithStdin(ctx, sanitized, "apply", "--cached", "--recount")
	if err != nil {
		return fmt.Errorf("git apply --cached: %w", err)
	}
	return nil
}

// sanitizeEditedDiff post-processes editor output to fix common side effects:
//   - Restores leading space on context lines whose prefix was stripped by
//     editors that remove trailing whitespace
//   - Strips comment lines (starting with #) matching git add -p behavior
func sanitizeEditedDiff(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))

	// Find the last non-empty line index to identify trailing empty lines.
	lastNonEmpty := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i] != "" {
			lastNonEmpty = i
			break
		}
	}

	for i, line := range lines {
		// Strip comment lines.
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Preserve trailing empty lines as-is.
		if line == "" && i > lastNonEmpty {
			out = append(out, line)
			continue
		}

		// If the line doesn't start with a valid diff prefix, it's a context
		// line that lost its leading space to editor whitespace stripping.
		// Empty lines between content are also stripped context lines.
		if !hasValidDiffPrefix(line) {
			line = " " + line
		}

		out = append(out, line)
	}

	return strings.Join(out, "\n")
}

// hasValidDiffPrefix reports whether line starts with a character that is a
// valid unified diff line prefix: '+', '-', '\', '@', or ' ' (space).
func hasValidDiffPrefix(line string) bool {
	if line == "" {
		return false
	}
	switch line[0] {
	case '+', '-', '\\', '@', ' ':
		return true
	}
	return false
}
