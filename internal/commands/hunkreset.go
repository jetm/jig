package commands

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	ctx      context.Context
	runner   git.Runner
	renderer diff.Renderer
	cfg      config.Config

	files         []git.FileDiff
	hunkList      components.HunkList
	diffView      components.DiffView
	statusBar     components.StatusBar
	help          components.HelpOverlay
	branch        string
	width         int
	height        int
	panelRatio    int
	focusRight    bool
	showDiff      bool
	diffMaximized bool
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
) *HunkResetModel {
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
	rawDiff, _ := runner.Run(ctx, diffArgs...)
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
		ctx:           ctx,
		runner:        runner,
		renderer:      renderer,
		cfg:           cfg,
		files:         files,
		hunkList:      hl,
		filterPaths:   paths,
		noMatchFilter: noMatch,
		diffView:      components.NewDiffView(80, 20),
		statusBar:     components.NewStatusBar(120),
		help: components.NewHelpOverlay([]components.KeyGroup{
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
					{Key: "q/Esc", Desc: "quit"},
				},
			},
		}),
		branch:     branchName,
		panelRatio: cfg.PanelRatio,
	}

	m.showDiff = cfg.ShowDiffPanel
	m.diffView.SetSoftWrap(cfg.SoftWrap)

	m.statusBar.SetHints(m.hintsForContext())
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("hunk-reset")

	if len(files) > 0 {
		m.renderCurrentHunk()
	}

	return m
}

// Update handles messages and returns commands.
func (m *HunkResetModel) Update(msg tea.Msg) tea.Cmd {
	sbCmd := m.statusBar.Update(msg)

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

		if msg.Code == tea.KeyTab {
			if m.showDiff && !m.diffMaximized {
				m.focusRight = !m.focusRight
				m.statusBar.SetHints(m.hintsForContext())
			}
			return sbCmd
		}

		if msg.String() == "D" {
			m.showDiff = !m.showDiff
			if m.showDiff && len(m.files) > 0 {
				m.renderCurrentHunk()
			}
			return sbCmd
		}

		if msg.String() == "F" {
			if m.showDiff {
				m.diffMaximized = !m.diffMaximized
				m.statusBar.SetHints(m.hintsForContext())
			}
			return sbCmd
		}

		if msg.String() == "w" && m.focusRight {
			m.diffView.SetSoftWrap(!m.diffView.SoftWrap())
			return sbCmd
		}

		if msg.String() == "[" {
			if m.panelRatio > 20 {
				m.panelRatio -= 5
				if m.panelRatio < 20 {
					m.panelRatio = 20
				}
				m.cfg.PanelRatio = m.panelRatio
				_ = config.Save(m.cfg)
				m.resize()
			}
			return sbCmd
		}

		if msg.String() == "]" {
			if m.panelRatio < 80 {
				m.panelRatio += 5
				if m.panelRatio > 80 {
					m.panelRatio = 80
				}
				m.cfg.PanelRatio = m.panelRatio
				_ = config.Save(m.cfg)
				m.resize()
			}
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
			dvCmd := m.diffView.Update(msg)
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
		contentHeight := m.height - 1
		m.statusBar.SetWidth(m.width)
		m.statusBar.SetHints(m.hintsForContext())

		switch {
		case !m.showDiff:
			panelW := m.width - 1
			leftContent := m.renderLeftPanel(panelW, contentHeight)
			leftPanel := tui.StyleFocusBorder.Width(panelW).Height(contentHeight).MaxHeight(contentHeight).Render(leftContent)
			background = leftPanel + "\n" + m.statusBar.View()
		case m.diffMaximized:
			rightW := m.width - 1
			m.diffView.SetWidth(rightW)
			m.diffView.SetHeight(contentHeight)
			rightPanel := tui.StyleFocusBorder.Width(rightW).Height(contentHeight).MaxHeight(contentHeight).Render(m.diffView.View())
			background = rightPanel + "\n" + m.statusBar.View()
		default:
			leftW, rightW := tui.ColumnsFromConfig(m.width, m.panelRatio)

			leftW--
			rightW--

			m.diffView.SetWidth(rightW)
			m.diffView.SetHeight(contentHeight)

			leftBorder, rightBorder := tui.StyleFocusBorder, tui.StyleDimBorder
			if m.focusRight {
				leftBorder, rightBorder = tui.StyleDimBorder, tui.StyleFocusBorder
			}

			leftContent := m.renderLeftPanel(leftW, contentHeight)
			leftPanel := leftBorder.Width(leftW).Height(contentHeight).MaxHeight(contentHeight).Render(leftContent)
			rightPanel := rightBorder.Width(rightW).Height(contentHeight).MaxHeight(contentHeight).Render(m.diffView.View())

			panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
			background = panels + "\n" + m.statusBar.View()
		}
	}

	return m.help.View(background, m.width, m.height)
}

// renderLeftPanel renders the hunk list at full panel height.
func (m *HunkResetModel) renderLeftPanel(width, height int) string {
	m.hunkList.SetWidth(width)
	m.hunkList.SetHeight(height)
	return m.hunkList.View()
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

// renderCurrentHunk updates the right-panel diff view with the hunk under the cursor.
func (m *HunkResetModel) renderCurrentHunk() {
	hunk, ok := m.hunkList.CurrentHunk()
	if !ok {
		m.diffView.SetContent("(no hunks)")
		return
	}

	raw := hunk.Body()
	rendered, err := m.renderer.Render(raw)
	if err != nil {
		rendered = raw
	}
	m.diffView.SetContent(rendered)
	m.prevFileIdx = m.hunkList.CurrentFileIdx()
	m.prevHunkIdx = m.hunkList.CurrentHunkIdx()
}

// syncDiffPreview updates the diff view if the cursor moved to a different hunk.
func (m *HunkResetModel) syncDiffPreview() {
	fi := m.hunkList.CurrentFileIdx()
	hi := m.hunkList.CurrentHunkIdx()
	if fi != m.prevFileIdx || hi != m.prevHunkIdx {
		m.renderCurrentHunk()
	}
}

// applySelected unstages all checked hunks via git apply --cached --reverse.
func (m *HunkResetModel) applySelected() tea.Cmd {
	staged := m.hunkList.StagedHunks()
	if len(staged) == 0 {
		return nil
	}

	var lastErr error
	applied := 0
	for _, sh := range staged {
		if sh.FileIdx >= len(m.files) {
			continue
		}
		patch := m.patchHeader(m.files[sh.FileIdx]) + "\n" + sh.Hunk.Body() + "\n"
		err := git.UnstageHunk(m.ctx, m.runner, patch)
		if err != nil {
			lastErr = err
			continue
		}
		applied++
	}

	if lastErr != nil {
		_ = m.statusBar.SetMessage(fmt.Sprintf("Unstage failed: %v", lastErr), components.Error)
		if applied == 0 {
			return nil
		}
	}

	return func() tea.Msg {
		return app.PopModelMsg{MutatedGit: true}
	}
}

// patchHeader builds a minimal unified diff header for git apply.
func (m *HunkResetModel) patchHeader(fd git.FileDiff) string {
	oldPath := "a/" + fd.OldPath
	newPath := "b/" + fd.NewPath
	return fmt.Sprintf("diff --git %s %s\n--- %s\n+++ %s", oldPath, newPath, oldPath, newPath)
}

// resize recalculates component dimensions after a terminal resize.
func (m *HunkResetModel) resize() {
	contentHeight := m.height - 1
	m.statusBar.SetWidth(m.width)

	if !m.showDiff {
		panelW := m.width - 1
		m.hunkList.SetWidth(panelW)
		m.hunkList.SetHeight(contentHeight)
		return
	}

	if m.diffMaximized {
		rightW := m.width - 1
		m.diffView.SetWidth(rightW)
		m.diffView.SetHeight(contentHeight)
		return
	}

	leftW, rightW := tui.ColumnsFromConfig(m.width, m.panelRatio)

	leftW--
	rightW--

	m.hunkList.SetWidth(leftW)
	m.hunkList.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)
}
