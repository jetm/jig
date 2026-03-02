package commands

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/jig/internal/app"
	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/git"
	"github.com/jetm/jig/internal/tui"
	"github.com/jetm/jig/internal/tui/components"
)

// HunkResetModel is the command model for the hunk-reset TUI (interactive hunk-level unstaging).
// It uses a HunkList component for checkbox-based hunk selection and batch unstaging.
type HunkResetModel struct {
	twoPanelModel

	ctx      context.Context
	runner   git.Runner
	renderer diff.Renderer

	files         []git.FileDiff
	hunkList      components.HunkList
	branch        string
	filterPaths   []string
	noMatchFilter bool
	// prevFileIdx/prevHunkIdx track cursor position to detect changes and update diff.
	prevFileIdx int
	prevHunkIdx int
}

// NewHunkResetModel creates a HunkResetModel by listing files with staged changes.
// filterPaths optionally restricts the file list to specific paths.
func NewHunkResetModel(
	ctx context.Context,
	runner git.Runner,
	cfg config.Config,
	renderer diff.Renderer,
	filterPaths ...[]string,
) (*HunkResetModel, error) {
	var paths []string
	if len(filterPaths) > 0 {
		paths = ExpandGlobs(filterPaths[0])
	}
	// Staged changes: git diff --cached
	diffArgs := []string{"diff", "--cached"}
	if len(paths) > 0 {
		diffArgs = append(diffArgs, "--")
		diffArgs = append(diffArgs, paths...)
	}
	rawDiff, err := runner.Run(ctx, diffArgs...)
	if err != nil {
		return nil, fmt.Errorf("running git diff --cached: %w", err)
	}
	branchName, _ := git.BranchName(ctx, runner)

	files := git.ParseFileDiffs(rawDiff)

	// Parse hunks per file for HunkList.
	hunks := make([][]git.Hunk, len(files))
	for i, f := range files {
		hunks[i] = git.ParseHunks(f.RawDiff)
	}

	noMatch := len(filterPaths) > 0 && len(filterPaths[0]) > 0 && len(files) == 0

	hl := components.NewHunkList(files, hunks)
	hl.SetSelectionLabel("selected")

	m := &HunkResetModel{
		twoPanelModel: newTwoPanelModel(
			&hl,
			components.NewDiffView(80, 20),
			components.NewStatusBar(120),
			components.NewHelpOverlay([]components.KeyGroup{
				{
					Name: "Navigation",
					Bindings: []components.KeyBinding{
						{Key: "j/k", Desc: "move between hunks"},
						{Key: "Tab", Desc: "switch panel"},
						{Key: "D", Desc: "toggle diff"},
						{Key: "?", Desc: "toggle help"},
					},
				},
				{
					Name: "Actions",
					Bindings: []components.KeyBinding{
						{Key: "Space", Desc: "toggle hunk selected"},
						{Key: "Enter", Desc: "unstage selected hunks"},
						{Key: "w", Desc: "toggle soft-wrap (diff panel)"},
						{Key: "F", Desc: "maximize diff panel"},
						{Key: "/", Desc: "search in diff"},
						{Key: "n/N", Desc: "next/prev match"},
						{Key: "q/Esc", Desc: "quit"},
					},
				},
			}),
			cfg,
		),
		ctx:           ctx,
		runner:        runner,
		renderer:      renderer,
		files:         files,
		hunkList:      hl,
		filterPaths:   paths,
		noMatchFilter: noMatch,
		branch:        branchName,
	}

	m.status.SetHints(m.hintsForContext())
	m.status.SetBranch(branchName)
	m.status.SetMode("hunk-reset")

	if len(files) > 0 {
		m.renderCurrentHunk()
	}

	return m, nil
}

// Update handles messages and returns commands.
func (m *HunkResetModel) Update(msg tea.Msg) tea.Cmd {
	sbCmd := m.status.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return sbCmd

	case tea.KeyPressMsg:
		if m.help.HandleKey(msg) {
			return sbCmd
		}

		if cmd, handled := m.handleKey(msg); handled {
			if cmd != nil {
				return cmd
			}
			if msg.String() == "D" && m.showDiff && len(m.files) > 0 {
				m.renderCurrentHunk()
			}
			m.status.SetHints(m.hintsForContext())
			return sbCmd
		}

		switch msg.Code {
		case 'q', tea.KeyEscape:
			return func() tea.Msg {
				return app.PopModelMsg{MutatedGit: false}
			}

		case tea.KeyEnter:
			return m.applySelected()
		}

		if m.focusRight {
			dvCmd := m.diff.Update(msg)
			return tea.Batch(sbCmd, dvCmd)
		}

		// Forward navigation (j/k/Space) to the hunk list.
		m.hunkList.Update(msg)
		m.syncDiffPreview()
		return sbCmd
	}

	return sbCmd
}

// View renders the two-panel layout with the help overlay composited on top
// when visible.
func (m *HunkResetModel) View() string {
	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	var background string
	switch {
	case m.noMatchFilter:
		background = "No matching changes for the given paths."
	case len(m.files) == 0:
		background = "No staged changes to unstage."
	default:
		m.left = &m.hunkList
		m.status.SetHints(m.hintsForContext())
		background = m.renderLayout()
	}

	return m.help.View(background, m.width, m.height)
}

// hintsForContext returns status bar hints based on current context.
func (m *HunkResetModel) hintsForContext() string {
	if m.diffMaximized {
		return "F: restore  ?: help  q: quit"
	}
	if m.focusRight {
		return "h/l: scroll  Tab: panel  ?: help  q: quit"
	}
	return "Space: toggle  Enter: unstage  Tab: panel  ?: help  q: quit"
}

func (m *HunkResetModel) renderCurrentHunk() {
	renderHunkPreview(&m.hunkList, &m.diff, m.renderer, &m.prevFileIdx, &m.prevHunkIdx)
}

func (m *HunkResetModel) syncDiffPreview() {
	syncHunkPreview(&m.hunkList, &m.diff, m.renderer, &m.prevFileIdx, &m.prevHunkIdx)
}

func (m *HunkResetModel) applySelected() tea.Cmd {
	return applyHunks(m.ctx, m.runner, &m.hunkList, m.files, &m.status, git.UnstageHunk, "Unstage")
}
