package commands

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jetm/jig/internal/app"
	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/editor"
	"github.com/jetm/jig/internal/git"
	"github.com/jetm/jig/internal/tui"
	"github.com/jetm/jig/internal/tui/components"
)

// ResetModel is the command model for the reset TUI (interactive unstaging).
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type ResetModel struct {
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
	filterPaths   []string
	noMatchFilter bool
}

// NewResetModel creates a ResetModel by listing staged files.
// filterPaths optionally restricts the file list to specific paths.
func NewResetModel(
	ctx context.Context,
	runner git.Runner,
	cfg config.Config,
	renderer diff.Renderer,
	filterPaths ...[]string,
) (*ResetModel, error) {
	var paths []string
	if len(filterPaths) > 0 {
		paths = ExpandGlobs(filterPaths[0])
	}
	files, err := git.ListStagedFilesFiltered(ctx, runner, paths)
	if err != nil {
		return nil, fmt.Errorf("listing staged files: %w", err)
	}
	branchName, _ := git.BranchName(ctx, runner)

	entries := make([]components.FileEntry, len(files))
	for i, f := range files {
		entries[i] = components.FileEntry{Path: f.Path, Status: f.Status}
	}

	noMatch := len(filterPaths) > 0 && len(filterPaths[0]) > 0 && len(files) == 0

	m := &ResetModel{
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
					{Key: "Enter", Desc: "unstage selected files"},
					{Key: "w", Desc: "toggle soft-wrap (diff panel)"},
					{Key: "F", Desc: "maximize diff panel"},
					{Key: "q/Esc", Desc: "quit without unstaging"},
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
	m.statusBar.SetMode("reset")

	if len(files) > 0 && m.showDiff {
		m.renderSelectedDiff()
	}

	return m, nil
}

// Update handles messages.
func (m *ResetModel) Update(msg tea.Msg) tea.Cmd {
	sbCmd := m.statusBar.Update(msg)

	switch msg := msg.(type) {
	case editor.EditDiffMsg:
		if msg.Err != nil {
			_ = m.statusBar.SetMessage(fmt.Sprintf("Edit failed: %v", msg.Err), components.Error)
			return sbCmd
		}
		if err := editor.ApplyEditedDiff(m.ctx, m.runner, msg.OriginalDiff, msg.EditedPath); err != nil {
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
			rawDiff, err := m.runner.Run(m.ctx, "diff", "--cached", "--", path)
			if err != nil || rawDiff == "" {
				_ = m.statusBar.SetMessage("No diff to edit", components.Info)
				return sbCmd
			}
			return editor.EditDiff(m.ctx, m.runner, rawDiff)
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

		switch msg.Code {
		case 'q', tea.KeyEscape:
			return func() tea.Msg {
				return app.PopModelMsg{MutatedGit: false}
			}

		case tea.KeyEnter:
			return m.unstageSelected()

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
func (m *ResetModel) View() string {
	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	var background string
	switch {
	case m.noMatchFilter:
		background = "No matching changes for the given paths."
	case len(m.files) == 0:
		background = "Nothing to unstage."
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
func (m *ResetModel) selectedPaths() []string {
	paths := m.fileList.CheckedPaths()
	if len(paths) > 0 {
		return paths
	}
	if path := m.fileList.SelectedPath(); path != "" {
		return []string{path}
	}
	return nil
}

// unstageSelected runs git reset HEAD for the selected files and returns a PopModelMsg on success.
// On failure, the model stays visible with the error in the status bar.
func (m *ResetModel) unstageSelected() tea.Cmd {
	paths := m.selectedPaths()
	if len(paths) == 0 {
		return nil
	}
	if err := git.UnstageFiles(m.ctx, m.runner, paths); err != nil {
		return m.statusBar.SetMessage(fmt.Sprintf("Unstage failed: %v", err), components.Error)
	}
	return func() tea.Msg {
		return app.PopModelMsg{MutatedGit: true}
	}
}

// renderSelectedDiff renders the staged diff for the currently focused file.
func (m *ResetModel) renderSelectedDiff() {
	path := m.fileList.SelectedPath()
	if path == "" {
		return
	}

	// Run git diff --cached for this specific file
	raw, err := m.runner.Run(m.ctx, "diff", fmt.Sprintf("-U%d", m.contextLines), "--cached", "--", path)
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
	resetHintsLeft     = "Tab: panel  Enter: unstage  D: diff  ?: help  q: quit"
	resetHintsRight    = "w: wrap  F: maximize  Tab: panel  ?: help  q: quit"
	resetHintsMaximize = "F: restore  ?: help  q: quit"
)

// updateHints sets the status bar hints based on the current focus and maximize state.
func (m *ResetModel) updateHints() {
	switch {
	case m.diffMaximized:
		m.statusBar.SetHints(resetHintsMaximize)
	case m.focusRight:
		m.statusBar.SetHints(resetHintsRight)
	default:
		m.statusBar.SetHints(resetHintsLeft)
	}
}

// resize recalculates component dimensions after a terminal resize.
func (m *ResetModel) resize() {
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
