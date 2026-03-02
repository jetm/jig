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
	twoPanelModel

	ctx      context.Context
	runner   git.Runner
	renderer diff.Renderer

	files         []git.FileDiff
	hunkList      components.HunkList
	branch        string
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
						{Key: "Enter", Desc: "discard selected hunks (with confirmation)"},
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
	m.status.SetMode("hunk-checkout")

	if len(files) > 0 {
		m.renderCurrentHunk()
	}

	return m, nil
}

// Update handles messages and returns commands.
func (m *HunkCheckoutModel) Update(msg tea.Msg) tea.Cmd {
	sbCmd := m.status.Update(msg)

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
			selected := m.hunkList.StagedHunks()
			if len(selected) == 0 {
				return sbCmd
			}
			m.confirming = true
			return sbCmd
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
		m.left = &m.hunkList
		m.status.SetHints(m.hintsForContext())
		if m.confirming {
			background = m.renderLayoutWithConfirm()
		} else {
			background = m.renderLayout()
		}
	}

	return m.help.View(background, m.width, m.height)
}

// renderLayoutWithConfirm renders the layout with a confirmation prompt replacing the status bar.
func (m *HunkCheckoutModel) renderLayoutWithConfirm() string {
	contentHeight := m.height - 1
	m.status.SetWidth(m.width)

	selected := m.hunkList.StagedHunks()
	prompt := fmt.Sprintf("Discard %d hunk(s)? This cannot be undone. [y/N] ", len(selected))
	promptStyle := lipgloss.NewStyle().
		Foreground(tui.ColorYellow).
		Bold(true)
	promptLine := promptStyle.Render(prompt)

	switch {
	case !m.showDiff:
		panelW := m.width - 1
		m.hunkList.SetWidth(panelW)
		m.hunkList.SetHeight(contentHeight)
		leftPanel := tui.StyleFocusBorder.Width(panelW).Height(contentHeight).MaxHeight(contentHeight).Render(m.hunkList.View())
		return leftPanel + "\n" + promptLine
	case m.diffMaximized:
		rightW := m.width - 1
		m.diff.SetWidth(rightW)
		m.diff.SetHeight(contentHeight)
		rightPanel := tui.StyleFocusBorder.Width(rightW).Height(contentHeight).MaxHeight(contentHeight).Render(m.diff.View())
		return rightPanel + "\n" + promptLine
	default:
		leftW, rightW := tui.ColumnsFromConfig(m.width, m.panelRatio)
		leftW--
		rightW--

		m.hunkList.SetWidth(leftW)
		m.hunkList.SetHeight(contentHeight)
		m.diff.SetWidth(rightW)
		m.diff.SetHeight(contentHeight)

		leftBorder, rightBorder := tui.StyleFocusBorder, tui.StyleDimBorder
		if m.focusRight {
			leftBorder, rightBorder = tui.StyleDimBorder, tui.StyleFocusBorder
		}

		leftPanel := leftBorder.Width(leftW).Height(contentHeight).MaxHeight(contentHeight).Render(m.hunkList.View())
		rightPanel := rightBorder.Width(rightW).Height(contentHeight).MaxHeight(contentHeight).Render(m.diff.View())

		panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
		return panels + "\n" + promptLine
	}
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

func (m *HunkCheckoutModel) renderCurrentHunk() {
	renderHunkPreview(&m.hunkList, &m.diff, m.renderer, &m.prevFileIdx, &m.prevHunkIdx)
}

func (m *HunkCheckoutModel) syncDiffPreview() {
	syncHunkPreview(&m.hunkList, &m.diff, m.renderer, &m.prevFileIdx, &m.prevHunkIdx)
}

func (m *HunkCheckoutModel) applySelected() tea.Cmd {
	return applyHunks(m.ctx, m.runner, &m.hunkList, m.files, &m.status, git.DiscardHunk, "Discard")
}
