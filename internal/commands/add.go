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

// AddModel is the command model for the add TUI (interactive staging).
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type AddModel struct {
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
	focusRight bool
}

// NewAddModel creates an AddModel by listing unstaged files.
func NewAddModel(
	ctx context.Context,
	runner git.Runner,
	_ config.Config,
	renderer diff.Renderer,
) *AddModel {
	files, _ := git.ListUnstagedFiles(ctx, runner)
	branchName, _ := git.BranchName(ctx, runner)

	entries := make([]components.FileEntry, len(files))
	for i, f := range files {
		entries[i] = components.FileEntry{Path: f.Path, Status: f.Status}
	}

	m := &AddModel{
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
					{Key: "Space", Desc: "toggle selection"},
					{Key: "a", Desc: "select all"},
					{Key: "d", Desc: "deselect all"},
					{Key: "?", Desc: "toggle help"},
				},
			},
			{
				Name: "Actions",
				Bindings: []components.KeyBinding{
					{Key: "Enter", Desc: "stage selected files"},
					{Key: "q/Esc", Desc: "quit without staging"},
				},
			},
		}),
		branch: branchName,
	}

	m.statusBar.SetHints("Space: toggle  a: all  d: none  Tab: panel  Enter: stage  ?: help  q: quit")
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("add")

	if len(files) > 0 {
		m.renderSelectedDiff()
	}

	return m
}

// Update handles messages.
func (m *AddModel) Update(msg tea.Msg) tea.Cmd {
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
			return m.stageSelected()

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

// View renders the two-panel layout.
func (m *AddModel) View() string {
	if m.help.IsVisible() {
		return m.help.View(m.width, m.height)
	}

	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	if len(m.files) == 0 {
		return "Nothing to stage."
	}

	leftW, rightW := tui.Columns(m.width)
	contentHeight := m.height - 1

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

	m.statusBar.SetWidth(m.width)
	return panels + "\n" + m.statusBar.View()
}

// selectedPaths returns the paths of all checked files.
// If none are checked, returns the focused file's path (single-file shortcut).
func (m *AddModel) selectedPaths() []string {
	paths := m.fileList.CheckedPaths()
	if len(paths) > 0 {
		return paths
	}
	// Fallback: stage focused file
	if path := m.fileList.SelectedPath(); path != "" {
		return []string{path}
	}
	return nil
}

// stageSelected runs git add for the selected files and returns a PopModelMsg.
func (m *AddModel) stageSelected() tea.Cmd {
	paths := m.selectedPaths()
	if len(paths) == 0 {
		return nil
	}
	err := git.StageFiles(m.ctx, m.runner, paths)
	mutated := err == nil
	var msgCmd tea.Cmd
	if err != nil {
		msgCmd = m.statusBar.SetMessage(fmt.Sprintf("Stage failed: %v", err), components.Error)
		_ = msgCmd
	}
	return func() tea.Msg {
		return app.PopModelMsg{MutatedGit: mutated}
	}
}

// renderSelectedDiff renders the diff for the currently focused file.
func (m *AddModel) renderSelectedDiff() {
	path := m.fileList.SelectedPath()
	if path == "" {
		return
	}

	// For untracked files, use --no-index to produce a diff against /dev/null.
	// git diff --no-index exits 1 when files differ, so use RunAllowExitCode.
	sf := m.findFile(path)
	var raw string
	var err error
	if sf != nil && sf.Status == git.Added && !m.isTracked(path) {
		raw, err = m.runner.RunAllowExitCode(m.ctx, 1, "diff", "--no-index", "--", "/dev/null", path)
	} else {
		raw, err = m.runner.Run(m.ctx, "diff", "--", path)
	}
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

// findFile returns the StatusFile for the given path, or nil.
func (m *AddModel) findFile(path string) *git.StatusFile {
	for i := range m.files {
		if m.files[i].Path == path {
			return &m.files[i]
		}
	}
	return nil
}

// isTracked returns true if the file path appears in the tracked working-tree diff
// (i.e., is not purely untracked). Used to decide whether to show diff or placeholder.
func (m *AddModel) isTracked(path string) bool {
	for _, f := range m.files {
		if f.Path == path && f.Status != git.Added {
			return true
		}
	}
	return false
}

// resize recalculates component dimensions after a terminal resize.
func (m *AddModel) resize() {
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
