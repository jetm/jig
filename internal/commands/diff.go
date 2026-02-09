// Package commands provides TUI command models for gti subcommands.
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

// DiffModel is the command model for the diff TUI.
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type DiffModel struct {
	ctx           context.Context
	runner        git.Runner
	files         []git.FileDiff
	fileTree      components.FileTree
	diffView      components.DiffView
	statusBar     components.StatusBar
	help          components.HelpOverlay
	renderer      diff.Renderer
	cfg           config.Config
	branch        string
	selectedPath  string
	width         int
	height        int
	panelRatio    int
	focusRight    bool
	showDiff      bool
	diffMaximized bool
	filterPaths   []string
	noMatchFilter bool
}

// NewDiffModel creates a DiffModel by running git diff and parsing the output.
// filterPaths optionally restricts the file list to specific paths.
func NewDiffModel(
	ctx context.Context,
	runner git.Runner,
	cfg config.Config,
	renderer diff.Renderer,
	revision string,
	staged bool,
	filterPaths ...[]string,
) *DiffModel {
	var paths []string
	if len(filterPaths) > 0 {
		paths = ExpandGlobs(filterPaths[0])
	}
	args := git.DiffArgs(revision, staged)
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	rawDiff, _ := runner.Run(ctx, args...)
	branchName, _ := git.BranchName(ctx, runner)

	files := git.ParseFileDiffs(rawDiff)

	entries := make([]components.FileEntry, len(files))
	for i, f := range files {
		entries[i] = components.FileEntry{Path: f.DisplayPath(), Status: f.Status}
	}

	noMatch := len(filterPaths) > 0 && len(filterPaths[0]) > 0 && len(files) == 0

	m := &DiffModel{
		ctx:           ctx,
		runner:        runner,
		files:         files,
		fileTree:      components.NewFileTree(entries, false),
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
					{Key: "?", Desc: "toggle help"},
				},
			},
			{
				Name: "Actions",
				Bindings: []components.KeyBinding{
					{Key: "w", Desc: "toggle soft-wrap (diff panel)"},
					{Key: "F", Desc: "maximize diff panel"},
					{Key: "q/Esc", Desc: "quit"},
				},
			},
		}),
		renderer:   renderer,
		cfg:        cfg,
		branch:     branchName,
		panelRatio: cfg.PanelRatio,
	}

	m.showDiff = cfg.ShowDiffPanel
	m.diffView.SetSoftWrap(cfg.SoftWrap)

	m.updateHints()
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("diff")

	// Render first file if available and diff panel is visible
	if len(files) > 0 && m.showDiff {
		m.checkSelectionChange()
	}

	return m
}

// Update handles messages and returns commands.
func (m *DiffModel) Update(msg tea.Msg) tea.Cmd {
	// Status bar always processes messages
	sbCmd := m.statusBar.Update(msg)

	switch msg := msg.(type) {
	case git.EditDiffMsg:
		if msg.Err != nil {
			_ = m.statusBar.SetMessage(fmt.Sprintf("Edit failed: %v", msg.Err), components.Error)
			return sbCmd
		}
		// diff is read-only but we still attempt apply and refresh.
		if err := git.ApplyEditedDiff(m.ctx, m.runner, msg.OriginalDiff, msg.EditedPath); err != nil {
			_ = m.statusBar.SetMessage(fmt.Sprintf("Apply failed: %v", err), components.Error)
			return sbCmd
		}
		_ = m.statusBar.SetMessage("Patch applied", components.Info)
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
				m.checkSelectionChange()
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
			// Edit the selected file's diff
			for _, f := range m.files {
				if f.DisplayPath() == m.selectedPath && f.RawDiff != "" {
					return git.EditDiff(m.ctx, m.runner, f.RawDiff)
				}
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

// View renders the two-panel layout with the help overlay composited on top
// when visible.
func (m *DiffModel) View() string {
	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	var background string
	switch {
	case m.noMatchFilter:
		background = "No matching changes for the given paths."
	case len(m.files) == 0:
		background = "No changes to display."
	default:
		contentHeight := m.height - 1 // reserve 1 row for status bar
		m.statusBar.SetWidth(m.width)

		if !m.showDiff {
			panelW := m.width - 1
			m.fileTree.SetWidth(panelW)
			m.fileTree.SetHeight(contentHeight)
			leftPanel := tui.StyleFocusBorder.Width(panelW).Height(contentHeight).MaxHeight(contentHeight).Render(m.fileTree.View())
			background = leftPanel + "\n" + m.statusBar.View()
		} else if m.diffMaximized {
			rightW := m.width - 1
			m.diffView.SetWidth(rightW)
			m.diffView.SetHeight(contentHeight)
			rightPanel := tui.StyleFocusBorder.Width(rightW).Height(contentHeight).MaxHeight(contentHeight).Render(m.diffView.View())
			background = rightPanel + "\n" + m.statusBar.View()
		} else {
			leftW, rightW := tui.ColumnsFromConfig(m.width, m.panelRatio)

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

			leftPanel := leftBorder.Width(leftW).Height(contentHeight).MaxHeight(contentHeight).Render(m.fileTree.View())
			rightPanel := rightBorder.Width(rightW).Height(contentHeight).MaxHeight(contentHeight).Render(m.diffView.View())

			panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
			background = panels + "\n" + m.statusBar.View()
		}
	}

	return m.help.View(background, m.width, m.height)
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

const (
	diffHintsLeft     = "j/k: navigate  Tab: panel  D: diff  ?: help  q: quit"
	diffHintsRight    = "w: wrap  F: maximize  Tab: panel  ?: help  q: quit"
	diffHintsMaximize = "F: restore  ?: help  q: quit"
)

// updateHints sets the status bar hints based on the current focus and maximize state.
func (m *DiffModel) updateHints() {
	switch {
	case m.diffMaximized:
		m.statusBar.SetHints(diffHintsMaximize)
	case m.focusRight:
		m.statusBar.SetHints(diffHintsRight)
	default:
		m.statusBar.SetHints(diffHintsLeft)
	}
}

// resize recalculates component dimensions after a terminal resize.
func (m *DiffModel) resize() {
	contentHeight := m.height - 1
	m.statusBar.SetWidth(m.width)

	if !m.showDiff {
		panelW := m.width - 1
		m.fileTree.SetWidth(panelW)
		m.fileTree.SetHeight(contentHeight)
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

	m.fileTree.SetWidth(leftW)
	m.fileTree.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)
}
