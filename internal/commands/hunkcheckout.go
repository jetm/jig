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

// HunkCheckoutModel is the command model for the hunk-checkout TUI (interactive hunk-level discard).
// It uses a HunkList component for checkbox-based hunk selection and working tree discard.
// Discarding is irreversible, so a confirmation prompt is shown before applying.
type HunkCheckoutModel struct {
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
	confirming    bool
	filterPaths   []string
	noMatchFilter bool
	// prevFileIdx/prevHunkIdx track cursor position to detect changes and update diff.
	prevFileIdx int
	prevHunkIdx int
}

// NewHunkCheckoutModel creates a HunkCheckoutModel by listing files with working tree changes.
// filterPaths optionally restricts the file list to specific paths.
func NewHunkCheckoutModel(
	ctx context.Context,
	runner git.Runner,
	cfg config.Config,
	renderer diff.Renderer,
	filterPaths ...[]string,
) (*HunkCheckoutModel, error) {
	var paths []string
	if len(filterPaths) > 0 {
		paths = ExpandGlobs(filterPaths[0])
	}
	// Working tree changes: git diff
	diffArgs := []string{"diff"}
	if len(paths) > 0 {
		diffArgs = append(diffArgs, "--")
		diffArgs = append(diffArgs, paths...)
	}
	rawDiff, err := runner.Run(ctx, diffArgs...)
	if err != nil {
		return nil, fmt.Errorf("running git diff: %w", err)
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

	m := &HunkCheckoutModel{
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
					{Key: "Enter", Desc: "discard selected hunks (with confirmation)"},
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
	m.statusBar.SetMode("hunk-checkout")

	if len(files) > 0 {
		m.renderCurrentHunk()
	}

	return m, nil
}

// Update handles messages and returns commands.
func (m *HunkCheckoutModel) Update(msg tea.Msg) tea.Cmd {
	sbCmd := m.statusBar.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return sbCmd

	case tea.KeyPressMsg:
		// Confirmation mode
		if m.confirming {
			return m.handleConfirmKey(msg, sbCmd)
		}

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
				if err := config.Save(m.cfg); err != nil {
					return m.statusBar.SetMessage(fmt.Sprintf("Config save failed: %v", err), components.Error)
				}
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
				if err := config.Save(m.cfg); err != nil {
					return m.statusBar.SetMessage(fmt.Sprintf("Config save failed: %v", err), components.Error)
				}
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
			selected := m.hunkList.StagedHunks()
			if len(selected) == 0 {
				return sbCmd
			}
			m.confirming = true
			return sbCmd
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

// handleConfirmKey processes keys while the confirmation prompt is visible.
func (m *HunkCheckoutModel) handleConfirmKey(msg tea.KeyPressMsg, sbCmd tea.Cmd) tea.Cmd {
	switch {
	case msg.Code == 'y' || msg.Text == "y":
		m.confirming = false
		return m.applySelected()

	default:
		// Any other key cancels confirmation
		m.confirming = false
		return sbCmd
	}
}

// View renders the two-panel layout with the help overlay composited on top
// when visible.
func (m *HunkCheckoutModel) View() string {
	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	var background string
	switch {
	case m.noMatchFilter:
		background = "No matching changes for the given paths."
	case len(m.files) == 0:
		background = "No working tree changes to discard."
	default:
		contentHeight := m.height - 1
		m.statusBar.SetWidth(m.width)
		m.statusBar.SetHints(m.hintsForContext())

		promptStyle := lipgloss.NewStyle().
			Foreground(tui.ColorYellow).
			Bold(true)

		switch {
		case !m.showDiff:
			panelW := m.width - 1
			leftContent := m.renderLeftPanel(panelW, contentHeight)
			leftPanel := tui.StyleFocusBorder.Width(panelW).Height(contentHeight).MaxHeight(contentHeight).Render(leftContent)
			if m.confirming {
				selected := m.hunkList.StagedHunks()
				prompt := fmt.Sprintf("Discard %d hunk(s)? This cannot be undone. [y/N] ", len(selected))
				background = leftPanel + "\n" + promptStyle.Render(prompt)
			} else {
				background = leftPanel + "\n" + m.statusBar.View()
			}
		case m.diffMaximized:
			rightW := m.width - 1
			m.diffView.SetWidth(rightW)
			m.diffView.SetHeight(contentHeight)
			rightPanel := tui.StyleFocusBorder.Width(rightW).Height(contentHeight).MaxHeight(contentHeight).Render(m.diffView.View())
			if m.confirming {
				selected := m.hunkList.StagedHunks()
				prompt := fmt.Sprintf("Discard %d hunk(s)? This cannot be undone. [y/N] ", len(selected))
				background = rightPanel + "\n" + promptStyle.Render(prompt)
			} else {
				background = rightPanel + "\n" + m.statusBar.View()
			}
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
			if m.confirming {
				selected := m.hunkList.StagedHunks()
				prompt := fmt.Sprintf("Discard %d hunk(s)? This cannot be undone. [y/N] ", len(selected))
				background = panels + "\n" + promptStyle.Render(prompt)
			} else {
				background = panels + "\n" + m.statusBar.View()
			}
		}
	}

	return m.help.View(background, m.width, m.height)
}

// renderLeftPanel renders the hunk list at full panel height.
func (m *HunkCheckoutModel) renderLeftPanel(width, height int) string {
	m.hunkList.SetWidth(width)
	m.hunkList.SetHeight(height)
	return m.hunkList.View()
}

// hintsForContext returns status bar hints based on current context.
func (m *HunkCheckoutModel) hintsForContext() string {
	if m.diffMaximized {
		return "F: restore  ?: help  q: quit"
	}
	if m.focusRight {
		return "h/l: scroll  Tab: panel  ?: help  q: quit"
	}
	return "Space: toggle  Enter: discard  Tab: panel  ?: help  q: quit"
}

// renderCurrentHunk updates the right-panel diff view with the hunk under the cursor.
func (m *HunkCheckoutModel) renderCurrentHunk() {
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
func (m *HunkCheckoutModel) syncDiffPreview() {
	fi := m.hunkList.CurrentFileIdx()
	hi := m.hunkList.CurrentHunkIdx()
	if fi != m.prevFileIdx || hi != m.prevHunkIdx {
		m.renderCurrentHunk()
	}
}

// applySelected discards all checked hunks via git apply --reverse.
func (m *HunkCheckoutModel) applySelected() tea.Cmd {
	selected := m.hunkList.StagedHunks()
	if len(selected) == 0 {
		return nil
	}

	var lastErr error
	applied := 0
	for _, sh := range selected {
		if sh.FileIdx >= len(m.files) {
			continue
		}
		patch := m.patchHeader(m.files[sh.FileIdx]) + "\n" + sh.Hunk.Body() + "\n"
		err := git.DiscardHunk(m.ctx, m.runner, patch)
		if err != nil {
			lastErr = err
			continue
		}
		applied++
	}

	if lastErr != nil {
		if applied == 0 {
			return m.statusBar.SetMessage(fmt.Sprintf("Discard failed: %v", lastErr), components.Error)
		}
		_ = m.statusBar.SetMessage(fmt.Sprintf("Discard failed: %v", lastErr), components.Error)
	}

	return func() tea.Msg {
		return app.PopModelMsg{MutatedGit: true}
	}
}

// patchHeader builds a minimal unified diff header for git apply.
func (m *HunkCheckoutModel) patchHeader(fd git.FileDiff) string {
	oldPath := "a/" + fd.OldPath
	newPath := "b/" + fd.NewPath
	return fmt.Sprintf("diff --git %s %s\n--- %s\n+++ %s", oldPath, newPath, oldPath, newPath)
}

// resize recalculates component dimensions after a terminal resize.
func (m *HunkCheckoutModel) resize() {
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
