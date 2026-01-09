// Package commands provides TUI command models for gti subcommands.
package commands

import (
	"context"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jetm/gti/internal/app"
	"github.com/jetm/gti/internal/config"
	"github.com/jetm/gti/internal/diff"
	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/tui"
	"github.com/jetm/gti/internal/tui/components"
)

// diffItem wraps a FileDiff for use with ItemList.
type diffItem struct {
	fd git.FileDiff
}

func (d diffItem) Title() string       { return d.fd.DisplayPath() }
func (d diffItem) Description() string { return statusLabel(d.fd.Status) }
func (d diffItem) FilterValue() string { return d.fd.DisplayPath() }

func statusLabel(s git.FileStatus) string {
	switch s {
	case git.Added:
		return tui.IconAdded + " added"
	case git.Deleted:
		return tui.IconDeleted + " deleted"
	case git.Renamed:
		return tui.IconRenamed + " renamed"
	default:
		return tui.IconModified + " modified"
	}
}

// DiffModel is the command model for the diff TUI.
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type DiffModel struct {
	files       []git.FileDiff
	fileList    components.ItemList
	diffView    components.DiffView
	statusBar   components.StatusBar
	help        components.HelpOverlay
	renderer    diff.Renderer
	branch      string
	selectedIdx int
	width       int
	height      int
	focusRight  bool
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

	items := make([]list.Item, len(files))
	for i, f := range files {
		items[i] = diffItem{fd: f}
	}

	m := &DiffModel{
		files:     files,
		fileList:  components.NewItemList(items, 40, 20),
		diffView:  components.NewDiffView(80, 20),
		statusBar: components.NewStatusBar(120),
		help: components.NewHelpOverlay([]components.KeyGroup{
			{
				Name: "Navigation",
				Bindings: []components.KeyBinding{
					{Key: "j/k", Desc: "move up/down"},
					{Key: "Tab", Desc: "switch panel"},
					{Key: "/", Desc: "filter files"},
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
		renderer:    renderer,
		branch:      branchName,
		selectedIdx: 0,
	}

	m.statusBar.SetHints("j/k: navigate  Tab: panel  /: filter  ?: help  q: quit")
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("diff")

	// Render first file if available
	if len(files) > 0 {
		m.renderSelectedDiff()
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

		// Forward to file list
		listCmd := m.fileList.Update(msg)

		// Check if selection changed
		m.checkSelectionChange()

		return tea.Batch(sbCmd, listCmd)
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

	m.fileList.SetWidth(leftW)
	m.fileList.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)

	leftBorder, rightBorder := tui.StyleFocusBorder, tui.StyleDimBorder
	if m.focusRight {
		leftBorder, rightBorder = tui.StyleDimBorder, tui.StyleFocusBorder
	}

	leftPanel := leftBorder.Width(leftW).Height(contentHeight).Render(m.fileList.View())
	rightPanel := rightBorder.Width(rightW).Height(contentHeight).Render(m.diffView.View())

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	m.statusBar.SetWidth(m.width)
	return panels + "\n" + m.statusBar.View()
}

// checkSelectionChange detects if the selected item changed and re-renders the diff.
func (m *DiffModel) checkSelectionChange() {
	sel := m.fileList.SelectedItem()
	if sel == nil {
		return
	}
	di, ok := sel.(diffItem)
	if !ok {
		return
	}
	// Find the index of this file
	for i, f := range m.files {
		if f.DisplayPath() == di.fd.DisplayPath() && i != m.selectedIdx {
			m.selectedIdx = i
			m.renderSelectedDiff()
			return
		}
	}
}

// renderSelectedDiff renders the currently selected file's diff through the renderer.
func (m *DiffModel) renderSelectedDiff() {
	if m.selectedIdx >= len(m.files) {
		return
	}
	raw := m.files[m.selectedIdx].RawDiff
	rendered, err := m.renderer.Render(raw)
	if err != nil {
		rendered = raw
	}
	m.diffView.SetContent(rendered)
}

// resize recalculates component dimensions after a terminal resize.
func (m *DiffModel) resize() {
	leftW, rightW := tui.Columns(m.width)
	contentHeight := m.height - 1

	leftW--
	rightW--

	m.fileList.SetWidth(leftW)
	m.fileList.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)
	m.statusBar.SetWidth(m.width)
}
