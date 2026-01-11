// Package commands provides TUI command models for gti subcommands.
package commands

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jetm/gti/internal/app"
	"github.com/jetm/gti/internal/config"
	"github.com/jetm/gti/internal/diff"
	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/tui"
	"github.com/jetm/gti/internal/tui/components"
)

// DiffModel is the command model for the diff TUI.
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type DiffModel struct {
	files        []git.FileDiff
	fileTree     components.FileTree
	diffView     components.DiffView
	statusBar    components.StatusBar
	help         components.HelpOverlay
	renderer     diff.Renderer
	branch       string
	selectedPath string
	width        int
	height       int
	focusRight   bool
}

// NewDiffModel creates a DiffModel by running git diff and parsing the output.
func NewDiffModel(
	ctx context.Context,
	runner git.Runner,
	_ config.Config,
	renderer diff.Renderer,
	revision string,
	staged bool,
) *DiffModel {
	args := git.DiffArgs(revision, staged)
	rawDiff, _ := runner.Run(ctx, args...)
	branchName, _ := git.BranchName(ctx, runner)

	files := git.ParseFileDiffs(rawDiff)

	entries := make([]components.FileEntry, len(files))
	for i, f := range files {
		entries[i] = components.FileEntry{Path: f.DisplayPath(), Status: f.Status}
	}

	m := &DiffModel{
		files:     files,
		fileTree:  components.NewFileTree(entries, false),
		diffView:  components.NewDiffView(80, 20),
		statusBar: components.NewStatusBar(120),
		help: components.NewHelpOverlay([]components.KeyGroup{
			{
				Name: "Navigation",
				Bindings: []components.KeyBinding{
					{Key: "j/k", Desc: "move up/down"},
					{Key: "o", Desc: "expand/collapse"},
					{Key: "Tab", Desc: "switch panel"},
					{Key: "?", Desc: "toggle help"},
				},
			},
			{
				Name: "Actions",
				Bindings: []components.KeyBinding{
					{Key: "q/Esc", Desc: "quit"},
				},
			},
		}),
		renderer: renderer,
		branch:   branchName,
	}

	m.statusBar.SetHints("j/k: navigate  o: expand/collapse  Tab: panel  ?: help  q: quit")
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("diff")

	// Render first file if available
	if len(files) > 0 {
		m.checkSelectionChange()
	}

	return m
}

// Update handles messages and returns commands.
func (m *DiffModel) Update(msg tea.Msg) tea.Cmd {
	// Status bar always processes messages
	sbCmd := m.statusBar.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return sbCmd

	case tea.KeyPressMsg:
		// Help overlay intercepts when visible
		if msg.Text == "?" {
			m.help.Toggle()
			return sbCmd
		}

		if m.help.IsVisible() {
			return sbCmd
		}

		if msg.Code == tea.KeyTab {
			m.focusRight = !m.focusRight
			return sbCmd
		}

		if msg.Code == 'q' || msg.Code == tea.KeyEscape {
			return func() tea.Msg {
				return app.PopModelMsg{MutatedGit: false}
			}
		}

		// Route navigation to focused panel
		if m.focusRight {
			dvCmd := m.diffView.Update(msg)
			return tea.Batch(sbCmd, dvCmd)
		}

		// Forward to file tree
		treeCmd := m.fileTree.Update(msg)

		// Check if selection changed
		m.checkSelectionChange()

		return tea.Batch(sbCmd, treeCmd)
	}

	return sbCmd
}

// View renders the two-panel layout.
func (m *DiffModel) View() string {
	if m.help.IsVisible() {
		return m.help.View(m.width, m.height)
	}

	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	if len(m.files) == 0 {
		return "No changes to display."
	}

	leftW, rightW := tui.Columns(m.width)
	contentHeight := m.height - 1 // reserve 1 row for status bar

	// Account for border width (1 column each)
	leftW--
	rightW--

	m.fileTree.SetWidth(leftW)
	m.fileTree.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)

	leftBorder, rightBorder := tui.StyleFocusBorder, tui.StyleDimBorder
	if m.focusRight {
		leftBorder, rightBorder = tui.StyleDimBorder, tui.StyleFocusBorder
	}

	leftPanel := leftBorder.Width(leftW).Height(contentHeight).Render(m.fileTree.View())
	rightPanel := rightBorder.Width(rightW).Height(contentHeight).Render(m.diffView.View())

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	m.statusBar.SetWidth(m.width)
	return panels + "\n" + m.statusBar.View()
}

// checkSelectionChange detects if the selected file changed and re-renders the diff.
func (m *DiffModel) checkSelectionChange() {
	path := m.fileTree.SelectedPath()
	if path == "" || path == m.selectedPath {
		return
	}
	m.selectedPath = path
	m.renderSelectedDiff()
}

// renderSelectedDiff renders the currently selected file's diff through the renderer.
func (m *DiffModel) renderSelectedDiff() {
	for _, f := range m.files {
		if f.DisplayPath() == m.selectedPath {
			rendered, err := m.renderer.Render(f.RawDiff)
			if err != nil {
				rendered = f.RawDiff
			}
			m.diffView.SetContent(rendered)
			return
		}
	}
}

// resize recalculates component dimensions after a terminal resize.
func (m *DiffModel) resize() {
	leftW, rightW := tui.Columns(m.width)
	contentHeight := m.height - 1

	leftW--
	rightW--

	m.fileTree.SetWidth(leftW)
	m.fileTree.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)
	m.statusBar.SetWidth(m.width)
}
