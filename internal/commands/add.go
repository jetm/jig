package commands

import (
	"context"
	"fmt"
	"os/exec"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jetm/jig/internal/app"
	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/git"
	"github.com/jetm/jig/internal/tui"
	"github.com/jetm/jig/internal/tui/components"
)

// AddModel is the command model for the add TUI (interactive staging).
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type AddModel struct {
	ctx           context.Context
	runner        git.Runner
	renderer      diff.Renderer
	cfg           config.Config
	files         []git.StatusFile
	fileList      components.FileList
	diffView      components.DiffView
	statusBar     components.StatusBar
	help          components.HelpOverlay
	branch        string
	width         int
	height        int
	panelRatio    int
	contextLines  int
	focusRight    bool
	showDiff      bool
	diffMaximized bool
	filterPaths   []string // optional paths to filter the file list
	noMatchFilter bool     // true when filterPaths produced no matching changes
}

// NewAddModel creates an AddModel by listing unstaged files.
// filterPaths optionally restricts the file list to specific paths (expanded from globs).
func NewAddModel(
	ctx context.Context,
	runner git.Runner,
	cfg config.Config,
	renderer diff.Renderer,
	filterPaths ...[]string,
) (*AddModel, error) {
	var paths []string
	if len(filterPaths) > 0 {
		paths = ExpandGlobs(filterPaths[0])
	}
	files, err := git.ListUnstagedFilesFiltered(ctx, runner, paths)
	if err != nil {
		return nil, fmt.Errorf("listing unstaged files: %w", err)
	}
	branchName, _ := git.BranchName(ctx, runner)

	entries := make([]components.FileEntry, len(files))
	for i, f := range files {
		entries[i] = components.FileEntry{Path: f.Path, Status: f.Status}
	}

	noMatch := len(filterPaths) > 0 && len(filterPaths[0]) > 0 && len(files) == 0

	m := &AddModel{
		ctx:           ctx,
		runner:        runner,
		renderer:      renderer,
		cfg:           cfg,
		files:         files,
		fileList:      components.NewFileList(entries, true),
		filterPaths:   paths,
		noMatchFilter: noMatch,
		diffView:      components.NewDiffView(80, 20),
		statusBar:     components.NewStatusBar(120),
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
					{Key: "Enter", Desc: "stage selected files"},
					{Key: "c", Desc: "stage and commit"},
					{Key: "C", Desc: "stage and commit (title only)"},
					{Key: "w", Desc: "toggle soft-wrap (diff panel)"},
					{Key: "F", Desc: "maximize diff panel"},
					{Key: "q/Esc", Desc: "quit without staging"},
				},
			},
		}),
		branch:       branchName,
		panelRatio:   cfg.PanelRatio,
		contextLines: cfg.DiffContext,
	}

	m.showDiff = cfg.ShowDiffPanel
	m.diffView.SetSoftWrap(cfg.SoftWrap)

	m.updateHints()
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("add")

	if len(files) > 0 && m.showDiff {
		m.renderSelectedDiff()
	}

	return m, nil
}

// Update handles messages.
func (m *AddModel) Update(msg tea.Msg) tea.Cmd {
	sbCmd := m.statusBar.Update(msg)

	switch msg := msg.(type) {
	case CommitDoneMsg:
		if msg.Err != nil {
			_ = m.statusBar.SetMessage("Commit aborted", components.Info)
			m.refreshFiles()
			return sbCmd
		}
		return func() tea.Msg {
			return app.PopModelMsg{MutatedGit: true}
		}

	case git.EditDiffMsg:
		if msg.Err != nil {
			_ = m.statusBar.SetMessage(fmt.Sprintf("Edit failed: %v", msg.Err), components.Error)
			return sbCmd
		}
		if err := git.ApplyEditedDiff(m.ctx, m.runner, msg.OriginalDiff, msg.EditedPath); err != nil {
			_ = m.statusBar.SetMessage(fmt.Sprintf("Apply failed: %v", err), components.Error)
			return sbCmd
		}
		m.renderSelectedDiff()
		return sbCmd

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

		if msg.String() == "F" {
			if m.showDiff {
				m.diffMaximized = !m.diffMaximized
				m.updateHints()
			}
			return sbCmd
		}

		if msg.String() == "w" && m.focusRight {
			m.diffView.SetSoftWrap(!m.diffView.SoftWrap())
			return sbCmd
		}

		if msg.String() == "e" {
			path := m.fileList.SelectedPath()
			if path == "" {
				return sbCmd
			}
			rawDiff, err := m.runner.Run(m.ctx, "diff", "--", path)
			if err != nil || rawDiff == "" {
				_ = m.statusBar.SetMessage("No diff to edit", components.Info)
				return sbCmd
			}
			return git.EditDiff(m.ctx, m.runner, rawDiff)
		}

		if msg.String() == "{" {
			if m.contextLines > 0 {
				m.contextLines--
				m.renderSelectedDiff()
			}
			return sbCmd
		}

		if msg.String() == "}" {
			if m.contextLines < 20 {
				m.contextLines++
				m.renderSelectedDiff()
			}
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

		if msg.String() == "c" {
			return m.execCommit(false)
		}

		if msg.String() == "C" {
			return m.execCommit(true)
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

// View renders the two-panel layout with the help overlay composited on top
// when visible.
func (m *AddModel) View() string {
	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	var background string
	switch {
	case m.noMatchFilter:
		background = "No matching changes for the given paths."
	case len(m.files) == 0:
		background = "Nothing to stage."
	default:
		contentHeight := m.height - 1
		m.statusBar.SetWidth(m.width)

		switch {
		case !m.showDiff:
			panelW := m.width - 1
			m.fileList.SetWidth(panelW)
			m.fileList.SetHeight(contentHeight)
			leftPanel := tui.StyleFocusBorder.Width(panelW).Height(contentHeight).MaxHeight(contentHeight).Render(m.fileList.View())
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
			background = panels + "\n" + m.statusBar.View()
		}
	}

	return m.help.View(background, m.width, m.height)
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

// stageSelected runs git add for the selected files and returns a PopModelMsg on success.
// On failure, the model stays visible with the error in the status bar.
func (m *AddModel) stageSelected() tea.Cmd {
	paths := m.selectedPaths()
	if len(paths) == 0 {
		return nil
	}
	if err := git.StageFiles(m.ctx, m.runner, paths); err != nil {
		return m.statusBar.SetMessage(fmt.Sprintf("Stage failed: %v", err), components.Error)
	}
	return func() tea.Msg {
		return app.PopModelMsg{MutatedGit: true}
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
	contextArg := fmt.Sprintf("-U%d", m.contextLines)
	if sf != nil && sf.Status == git.Added && !m.isTracked(path) {
		raw, err = m.runner.RunAllowExitCode(m.ctx, 1, "diff", contextArg, "--no-index", "--", "/dev/null", path)
	} else {
		raw, err = m.runner.Run(m.ctx, "diff", contextArg, "--", path)
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

const (
	addHintsLeft     = "Tab: panel  Enter: stage  c: commit  D: diff  ?: help  q: quit"
	addHintsRight    = "w: wrap  F: maximize  Tab: panel  ?: help  q: quit"
	addHintsMaximize = "F: restore  ?: help  q: quit"
)

// updateHints sets the status bar hints based on the current focus and maximize state.
func (m *AddModel) updateHints() {
	switch {
	case m.diffMaximized:
		m.statusBar.SetHints(addHintsMaximize)
	case m.focusRight:
		m.statusBar.SetHints(addHintsRight)
	default:
		m.statusBar.SetHints(addHintsLeft)
	}
}

// execCommit stages selected files and launches devtool commit as a subprocess.
// If titleOnly is true, passes -t for a title-only commit message.
// Returns nil if staging fails (error shown in status bar).
func (m *AddModel) execCommit(titleOnly bool) tea.Cmd {
	paths := m.selectedPaths()
	if len(paths) == 0 {
		return nil
	}
	if err := git.StageFiles(m.ctx, m.runner, paths); err != nil {
		_ = m.statusBar.SetMessage(fmt.Sprintf("Stage failed: %v", err), components.Error)
		return nil
	}
	args := []string{"commit"}
	if titleOnly {
		args = append(args, "-t")
	}
	cmd := exec.Command("devtool", args...) //nolint:gosec // devtool is a trusted user tool
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return CommitDoneMsg{Err: err}
	})
}

// refreshFiles re-queries git state and rebuilds the file list.
// Called after a commit completes to reflect the new index state.
func (m *AddModel) refreshFiles() {
	files, _ := git.ListUnstagedFilesFiltered(m.ctx, m.runner, m.filterPaths)
	m.files = files

	entries := make([]components.FileEntry, len(files))
	for i, f := range files {
		entries[i] = components.FileEntry{Path: f.Path, Status: f.Status}
	}
	m.fileList = components.NewFileList(entries, true)

	if len(files) > 0 && m.showDiff {
		m.renderSelectedDiff()
	}
	m.resize()
}

// resize recalculates component dimensions after a terminal resize.
func (m *AddModel) resize() {
	contentHeight := m.height - 1
	m.statusBar.SetWidth(m.width)

	if !m.showDiff {
		panelW := m.width - 1
		m.fileList.SetWidth(panelW)
		m.fileList.SetHeight(contentHeight)
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

	m.fileList.SetWidth(leftW)
	m.fileList.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)
}
