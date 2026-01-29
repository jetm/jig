package commands

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jetm/gti/internal/app"
	"github.com/jetm/gti/internal/config"
	"github.com/jetm/gti/internal/diff"
	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/tui"
	"github.com/jetm/gti/internal/tui/components"
)

// CheckoutModel is the command model for the checkout TUI (interactive discard).
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type CheckoutModel struct {
	ctx        context.Context
	runner     git.Runner
	renderer   diff.Renderer
	files      []git.StatusFile
	fileList   components.FileList
	diffView   components.DiffView
	statusBar  components.StatusBar
	help       components.HelpOverlay
	branch     string
	width      int
	height     int
	confirming bool
	focusRight bool
	showDiff   bool
}

// NewCheckoutModel creates a CheckoutModel by listing modified working-tree files.
func NewCheckoutModel(
	ctx context.Context,
	runner git.Runner,
	cfg config.Config,
	renderer diff.Renderer,
) *CheckoutModel {
	files, _ := git.ListModifiedFiles(ctx, runner)
	branchName, _ := git.BranchName(ctx, runner)

	entries := make([]components.FileEntry, len(files))
	for i, f := range files {
		entries[i] = components.FileEntry{Path: f.Path, Status: f.Status}
	}

	m := &CheckoutModel{
		ctx:       ctx,
		runner:    runner,
		renderer:  renderer,
		files:     files,
		fileList:  components.NewFileList(entries, true),
		diffView:  components.NewDiffView(80, 20),
		statusBar: components.NewStatusBar(120),
		help: components.NewHelpOverlay([]components.KeyGroup{
			{
				Name: "Navigation",
				Bindings: []components.KeyBinding{
					{Key: "j/k", Desc: "move up/down"},
					{Key: "o", Desc: "expand/collapse"},
					{Key: "Tab", Desc: "switch panel"},
					{Key: "D", Desc: "toggle diff"},
					{Key: "Space", Desc: "toggle selection"},
					{Key: "a", Desc: "select all"},
					{Key: "d", Desc: "deselect all"},
					{Key: "?", Desc: "toggle help"},
				},
			},
			{
				Name: "Actions",
				Bindings: []components.KeyBinding{
					{Key: "Enter", Desc: "discard changes (with confirmation)"},
					{Key: "q/Esc", Desc: "quit without discarding"},
				},
			},
		}),
		branch: branchName,
	}

	m.showDiff = cfg.ShowDiffPanel

	m.updateHints()
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("checkout")

	if len(files) > 0 && m.showDiff {
		m.renderSelectedDiff()
	}

	return m
}

// Update handles messages.
func (m *CheckoutModel) Update(msg tea.Msg) tea.Cmd {
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

		if msg.Text == "?" {
			m.help.Toggle()
			return sbCmd
		}

		if m.help.IsVisible() {
			return sbCmd
		}

		if msg.Code == tea.KeyTab {
			if m.showDiff {
				m.focusRight = !m.focusRight
				m.updateHints()
			}
			return sbCmd
		}

		if msg.String() == "D" {
			m.showDiff = !m.showDiff
			if m.showDiff && len(m.files) > 0 {
				m.renderSelectedDiff()
			}
			return sbCmd
		}

		switch msg.Code {
		case 'q', tea.KeyEscape:
			return func() tea.Msg {
				return app.PopModelMsg{MutatedGit: false}
			}

		case tea.KeyEnter:
			paths := m.selectedPaths()
			if len(paths) == 0 {
				return sbCmd
			}
			m.confirming = true
			return sbCmd

		case ' ':
			m.fileList.ToggleChecked()
			return sbCmd

		case 'a':
			m.fileList.SetAllChecked(true)
			return sbCmd

		case 'd':
			m.fileList.SetAllChecked(false)
			return sbCmd
		}

		if m.focusRight {
			dvCmd := m.diffView.Update(msg)
			return tea.Batch(sbCmd, dvCmd)
		}

		treeCmd := m.fileList.Update(msg)
		m.renderSelectedDiff()
		return tea.Batch(sbCmd, treeCmd)
	}

	return sbCmd
}

// handleConfirmKey processes keys while the confirmation prompt is visible.
func (m *CheckoutModel) handleConfirmKey(msg tea.KeyPressMsg, sbCmd tea.Cmd) tea.Cmd {
	switch {
	case msg.Code == 'y' || msg.Text == "y":
		m.confirming = false
		return m.discardSelected()

	default:
		// Any other key cancels confirmation
		m.confirming = false
		return sbCmd
	}
}

// View renders the two-panel layout.
func (m *CheckoutModel) View() string {
	if m.help.IsVisible() {
		return m.help.View(m.width, m.height)
	}

	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	if len(m.files) == 0 {
		return "Nothing to discard."
	}

	contentHeight := m.height - 1
	m.statusBar.SetWidth(m.width)

	if !m.showDiff {
		panelW := m.width - 1
		m.fileList.SetWidth(panelW)
		m.fileList.SetHeight(contentHeight)
		leftPanel := tui.StyleFocusBorder.Width(panelW).Height(contentHeight).MaxHeight(contentHeight).Render(m.fileList.View())

		if m.confirming {
			paths := m.selectedPaths()
			prompt := fmt.Sprintf("Discard changes to %d file(s)? [y/N] ", len(paths))
			promptStyle := lipgloss.NewStyle().
				Foreground(tui.ColorYellow).
				Bold(true)
			return leftPanel + "\n" + promptStyle.Render(prompt)
		}

		return leftPanel + "\n" + m.statusBar.View()
	}

	leftW, rightW := tui.Columns(m.width)

	leftW--
	rightW--

	m.fileList.SetWidth(leftW)
	m.fileList.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)

	leftBorder, rightBorder := tui.StyleFocusBorder, tui.StyleDimBorder
	if m.focusRight {
		leftBorder, rightBorder = tui.StyleDimBorder, tui.StyleFocusBorder
	}

	leftPanel := leftBorder.Width(leftW).Height(contentHeight).MaxHeight(contentHeight).Render(m.fileList.View())
	rightPanel := rightBorder.Width(rightW).Height(contentHeight).MaxHeight(contentHeight).Render(m.diffView.View())

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	if m.confirming {
		paths := m.selectedPaths()
		prompt := fmt.Sprintf("Discard changes to %d file(s)? [y/N] ", len(paths))
		promptStyle := lipgloss.NewStyle().
			Foreground(tui.ColorYellow).
			Bold(true)
		return panels + "\n" + promptStyle.Render(prompt)
	}

	return panels + "\n" + m.statusBar.View()
}

// selectedPaths returns paths of checked files or the focused file if none checked.
func (m *CheckoutModel) selectedPaths() []string {
	paths := m.fileList.CheckedPaths()
	if len(paths) > 0 {
		return paths
	}
	if path := m.fileList.SelectedPath(); path != "" {
		return []string{path}
	}
	return nil
}

// discardSelected runs git checkout -- for the selected files.
func (m *CheckoutModel) discardSelected() tea.Cmd {
	paths := m.selectedPaths()
	if len(paths) == 0 {
		return nil
	}
	err := git.DiscardFiles(m.ctx, m.runner, paths)
	mutated := err == nil
	if err != nil {
		msgCmd := m.statusBar.SetMessage(fmt.Sprintf("Discard failed: %v", err), components.Error)
		_ = msgCmd
	}
	return func() tea.Msg {
		return app.PopModelMsg{MutatedGit: mutated}
	}
}

// renderSelectedDiff renders the diff for the currently focused file.
func (m *CheckoutModel) renderSelectedDiff() {
	path := m.fileList.SelectedPath()
	if path == "" {
		return
	}

	raw, err := m.runner.Run(m.ctx, "diff", "--", path)
	if err != nil || raw == "" {
		m.diffView.SetContent("(no diff available)")
		return
	}

	rendered, err := m.renderer.Render(raw)
	if err != nil {
		rendered = raw
	}
	m.diffView.SetContent(rendered)
}

const (
	checkoutHintsLeft  = "Space: toggle  a: all  d: none  Tab: panel  D: diff  Enter: discard  ?: help  q: quit"
	checkoutHintsRight = "h/l: scroll  Tab: panel  D: diff  ?: help  q: quit"
)

// updateHints sets the status bar hints based on the current focus.
func (m *CheckoutModel) updateHints() {
	if m.focusRight {
		m.statusBar.SetHints(checkoutHintsRight)
	} else {
		m.statusBar.SetHints(checkoutHintsLeft)
	}
}

// resize recalculates component dimensions after a terminal resize.
func (m *CheckoutModel) resize() {
	contentHeight := m.height - 1
	m.statusBar.SetWidth(m.width)

	if !m.showDiff {
		panelW := m.width - 1
		m.fileList.SetWidth(panelW)
		m.fileList.SetHeight(contentHeight)
		return
	}

	leftW, rightW := tui.Columns(m.width)

	leftW--
	rightW--

	m.fileList.SetWidth(leftW)
	m.fileList.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)
}
