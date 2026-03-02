package commands

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/jig/internal/app"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/git"
	"github.com/jetm/jig/internal/tui/components"
)

// patchHeader builds a minimal unified diff header for git apply.
func patchHeader(fd git.FileDiff) string {
	oldPath := "a/" + fd.OldPath
	newPath := "b/" + fd.NewPath
	return fmt.Sprintf("diff --git %s %s\n--- %s\n+++ %s", oldPath, newPath, oldPath, newPath)
}

// renderHunkPreview renders the current hunk from the hunk list into the diff view.
func renderHunkPreview(
	hl *components.HunkList,
	dv *components.DiffView,
	renderer diff.Renderer,
	prevFileIdx *int,
	prevHunkIdx *int,
) {
	hunk, ok := hl.CurrentHunk()
	if !ok {
		dv.SetContent("(no hunks)")
		return
	}

	raw := hunk.Body()
	rendered, err := renderer.Render(raw)
	if err != nil {
		rendered = raw
	}
	dv.SetContent(rendered)
	*prevFileIdx = hl.CurrentFileIdx()
	*prevHunkIdx = hl.CurrentHunkIdx()
}

// syncHunkPreview updates the diff view if the cursor moved to a different hunk.
func syncHunkPreview(
	hl *components.HunkList,
	dv *components.DiffView,
	renderer diff.Renderer,
	prevFileIdx *int,
	prevHunkIdx *int,
) {
	fi := hl.CurrentFileIdx()
	hi := hl.CurrentHunkIdx()
	if fi != *prevFileIdx || hi != *prevHunkIdx {
		renderHunkPreview(hl, dv, renderer, prevFileIdx, prevHunkIdx)
	}
}

// hunkApplyFunc is the git operation to run for each hunk patch.
type hunkApplyFunc func(ctx context.Context, r git.Runner, patch string) error

// applyHunks applies all staged hunks using the given git operation.
// Returns a PopModelMsg command on success, or a status error on failure.
func applyHunks(
	ctx context.Context,
	runner git.Runner,
	hl *components.HunkList,
	files []git.FileDiff,
	status *components.StatusBar,
	applyFn hunkApplyFunc,
	verb string,
) tea.Cmd {
	staged := hl.StagedHunks()
	if len(staged) == 0 {
		return nil
	}

	var lastErr error
	applied := 0
	for _, sh := range staged {
		if sh.FileIdx >= len(files) {
			continue
		}
		header := patchHeader(files[sh.FileIdx])
		patch := header + "\n" + sh.Hunk.Body() + "\n"
		err := applyFn(ctx, runner, patch)
		if err != nil {
			lastErr = err
			continue
		}
		applied++
	}

	if lastErr != nil {
		if applied == 0 {
			return status.SetMessage(fmt.Sprintf("%s failed: %v", verb, lastErr), components.Error)
		}
		_ = status.SetMessage(fmt.Sprintf("%s failed: %v", verb, lastErr), components.Error)
	}

	return func() tea.Msg {
		return app.PopModelMsg{MutatedGit: true}
	}
}
