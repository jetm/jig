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

// ResetModel is the command model for the reset TUI (interactive unstaging).
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type ResetModel struct {
	ctx        context.Context
	runner     git.Runner
	renderer   diff.Renderer
	files      []git.StatusFile
	fileTree   components.FileTree
	diffView   components.DiffView
	statusBar  components.StatusBar
	help       components.HelpOverlay
	branch     string
	width      int
	height     int
	focusRight bool
}

// NewResetModel creates a ResetModel by listing staged files.
func NewResetModel(
	ctx context.Context,
	runner git.Runner,
	_ config.Config,
	renderer diff.Renderer,
) *ResetModel {
	files, _ := git.ListStagedFiles(ctx, runner)
	branchName, _ := git.BranchName(ctx, runner)

	entries := make([]components.FileEntry, len(files))
	for i, f := range files {
		entries[i] = components.FileEntry{Path: f.Path, Status: f.Status}
	}

	m := &ResetModel{
		ctx:       ctx,
		runner:    runner,
		renderer:  renderer,
		files:     files,
		fileTree:  components.NewFileTree(entries, true),
		diffView:  components.NewDiffView(80, 20),
		statusBar: components.NewStatusBar(120),
		help: components.NewHelpOverlay([]components.KeyGroup{
			{
				Name: "Navigation",
				Bindings: []components.KeyBinding{
					{Key: "j/k", Desc: "move up/down"},
					{Key: "o", Desc: "expand/collapse"},
					{Key: "Tab", Desc: "switch panel"},
					{Key: "Space", Desc: "toggle selection"},
					{Key: "a", Desc: "select all"},
					{Key: "d", Desc: "deselect all"},
					{Key: "?", Desc: "toggle help"},
				},
			},
			{
				Name: "Actions",
				Bindings: []components.KeyBinding{
					{Key: "Enter", Desc: "unstage selected files"},
					{Key: "q/Esc", Desc: "quit without unstaging"},
				},
			},
		}),
		branch: branchName,
	}

	m.statusBar.SetHints("Space: toggle  a: all  d: none  Tab: panel  Enter: unstage  ?: help  q: quit")
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("reset")

	if len(files) > 0 {
		m.renderSelectedDiff()
	}

	return m
}

// Update handles messages.
func (m *ResetModel) Update(msg tea.Msg) tea.Cmd {
	sbCmd := m.statusBar.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return sbCmd

	case tea.KeyPressMsg:
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

		switch msg.Code {
		case 'q', tea.KeyEscape:
			return func() tea.Msg {
				return app.PopModelMsg{MutatedGit: false}
			}

		case tea.KeyEnter:
			return m.unstageSelected()

		case ' ':
			m.fileTree.ToggleChecked()
			return sbCmd

		case 'a':
			m.fileTree.SetAllChecked(true)
			return sbCmd

		case 'd':
			m.fileTree.SetAllChecked(false)
			return sbCmd
		}

		if m.focusRight {
			dvCmd := m.diffView.Update(msg)
			return tea.Batch(sbCmd, dvCmd)
		}

		treeCmd := m.fileTree.Update(msg)
		m.renderSelectedDiff()
		return tea.Batch(sbCmd, treeCmd)
	}

	return sbCmd
}

// View renders the two-panel layout.
func (m *ResetModel) View() string {
	if m.help.IsVisible() {
		return m.help.View(m.width, m.height)
	}

	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	if len(m.files) == 0 {
		return "Nothing to unstage."
	}

	leftW, rightW := tui.Columns(m.width)
	contentHeight := m.height - 1

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

	leftPanel := leftBorder.Width(leftW).Height(contentHeight).MaxHeight(contentHeight).Render(m.fileTree.View())
	rightPanel := rightBorder.Width(rightW).Height(contentHeight).MaxHeight(contentHeight).Render(m.diffView.View())

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	m.statusBar.SetWidth(m.width)
	return panels + "\n" + m.statusBar.View()
}

// selectedPaths returns the paths of all checked files.
// If none are checked, returns the focused file's path (single-file shortcut).
func (m *ResetModel) selectedPaths() []string {
	paths := m.fileTree.CheckedPaths()
	if len(paths) > 0 {
		return paths
	}
	if path := m.fileTree.SelectedPath(); path != "" {
		return []string{path}
	}
	return nil
}

// unstageSelected runs git reset HEAD for the selected files and returns a PopModelMsg.
func (m *ResetModel) unstageSelected() tea.Cmd {
	paths := m.selectedPaths()
	if len(paths) == 0 {
		return nil
	}
	err := git.UnstageFiles(m.ctx, m.runner, paths)
	mutated := err == nil
	var msgCmd tea.Cmd
	if err != nil {
		msgCmd = m.statusBar.SetMessage(fmt.Sprintf("Unstage failed: %v", err), components.Error)
		_ = msgCmd
	}
	return func() tea.Msg {
		return app.PopModelMsg{MutatedGit: mutated}
	}
}

// renderSelectedDiff renders the staged diff for the currently focused file.
func (m *ResetModel) renderSelectedDiff() {
	path := m.fileTree.SelectedPath()
	if path == "" {
		return
	}

	// Run git diff --cached for this specific file
	raw, err := m.runner.Run(m.ctx, "diff", "--cached", "--", path)
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

// resize recalculates component dimensions after a terminal resize.
func (m *ResetModel) resize() {
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
