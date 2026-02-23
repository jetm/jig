package commands

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jetm/jig/internal/app"
	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/git"
	"github.com/jetm/jig/internal/tui"
	"github.com/jetm/jig/internal/tui/components"
)

const defaultLogCommitLimit = 50

// logItem wraps a git.CommitEntry for use with ItemList.
type logItem struct {
	entry git.CommitEntry
}

func (l logItem) Title() string {
	return tui.IconCommit + " " + l.entry.Hash + "  " + l.entry.Subject
}

func (l logItem) Description() string {
	return l.entry.Author + "  " + l.entry.Date
}

func (l logItem) FilterValue() string {
	return l.entry.Hash + " " + l.entry.Subject
}

// LogModel is the command model for the log TUI.
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type LogModel struct {
	ctx      context.Context
	runner   git.Runner
	renderer diff.Renderer
	cfg      config.Config

	commits       []git.CommitEntry
	commitList    components.ItemList
	diffView      components.DiffView
	statusBar     components.StatusBar
	help          components.HelpOverlay
	branch        string
	selectedIdx   int
	width         int
	height        int
	panelRatio    int
	contextLines  int
	focusRight    bool
	showDiff      bool
	diffMaximized bool
}

// NewLogModel creates a LogModel by loading recent commits from git log.
// If ref is non-empty, commits are loaded starting from that revision.
func NewLogModel(
	ctx context.Context,
	runner git.Runner,
	cfg config.Config,
	renderer diff.Renderer,
	ref string,
) (*LogModel, error) {
	commits, err := git.RecentCommitsFrom(ctx, runner, defaultLogCommitLimit, ref)
	if err != nil {
		return nil, fmt.Errorf("loading commits: %w", err)
	}
	branchName, _ := git.BranchName(ctx, runner)

	items := make([]list.Item, len(commits))
	for i, c := range commits {
		items[i] = logItem{entry: c}
	}

	m := &LogModel{
		ctx:        ctx,
		runner:     runner,
		renderer:   renderer,
		cfg:        cfg,
		commits:    commits,
		commitList: components.NewCompactItemList(items, 40, 20),
		diffView:   components.NewDiffView(80, 20),
		statusBar:  components.NewStatusBar(120),
		help: components.NewHelpOverlay([]components.KeyGroup{
			{
				Name: "Navigation",
				Bindings: []components.KeyBinding{
					{Key: "j/k", Desc: "move up/down"},
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
		branch:       branchName,
		selectedIdx:  0,
		panelRatio:   cfg.PanelRatio,
		contextLines: cfg.DiffContext,
	}

	m.showDiff = cfg.ShowDiffPanel
	m.diffView.SetSoftWrap(cfg.SoftWrap)

	m.updateHints()
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("log")

	// Render the first commit's diff if available and diff panel is visible.
	if len(commits) > 0 && m.showDiff {
		m.renderSelectedDiff()
	}

	return m, nil
}

// Update handles messages and returns commands.
func (m *LogModel) Update(msg tea.Msg) tea.Cmd {
	sbCmd := m.statusBar.Update(msg)

	switch msg := msg.(type) {
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
			if m.showDiff && len(m.commits) > 0 {
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
		}

		if m.focusRight {
			dvCmd := m.diffView.Update(msg)
			return tea.Batch(sbCmd, dvCmd)
		}

		// Forward navigation to commit list.
		listCmd := m.commitList.Update(msg)
		m.checkSelectionChange()
		return tea.Batch(sbCmd, listCmd)
	}

	return sbCmd
}

// View renders the two-panel layout with the help overlay composited on top
// when visible.
func (m *LogModel) View() string {
	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	var background string
	if len(m.commits) == 0 {
		background = "No commits to show."
	} else {
		contentHeight := m.height - 1 // reserve 1 row for status bar
		m.statusBar.SetWidth(m.width)

		switch {
		case !m.showDiff:
			panelW := m.width - 1
			m.commitList.SetWidth(panelW)
			m.commitList.SetHeight(contentHeight)
			leftPanel := tui.StyleFocusBorder.Width(panelW).Height(contentHeight).MaxHeight(contentHeight).Render(m.commitList.View())
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

			m.commitList.SetWidth(leftW)
			m.commitList.SetHeight(contentHeight)
			m.diffView.SetWidth(rightW)
			m.diffView.SetHeight(contentHeight)

			leftBorder, rightBorder := tui.StyleFocusBorder, tui.StyleDimBorder
			if m.focusRight {
				leftBorder, rightBorder = tui.StyleDimBorder, tui.StyleFocusBorder
			}

			leftPanel := leftBorder.Width(leftW).Height(contentHeight).MaxHeight(contentHeight).Render(m.commitList.View())
			rightPanel := rightBorder.Width(rightW).Height(contentHeight).MaxHeight(contentHeight).Render(m.diffView.View())

			panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
			background = panels + "\n" + m.statusBar.View()
		}
	}

	return m.help.View(background, m.width, m.height)
}

// checkSelectionChange detects if the list cursor moved and re-renders the diff.
func (m *LogModel) checkSelectionChange() {
	sel := m.commitList.SelectedItem()
	if sel == nil {
		return
	}
	li, ok := sel.(logItem)
	if !ok {
		return
	}
	for i, c := range m.commits {
		if c.Hash == li.entry.Hash && i != m.selectedIdx {
			m.selectedIdx = i
			m.renderSelectedDiff()
			return
		}
	}
}

// renderSelectedDiff fetches and renders the selected commit's diff in the right panel.
func (m *LogModel) renderSelectedDiff() {
	if m.selectedIdx >= len(m.commits) {
		return
	}
	hash := m.commits[m.selectedIdx].Hash
	raw, err := git.CommitDiff(m.ctx, m.runner, hash, m.contextLines)
	if err != nil {
		m.diffView.SetContent(fmt.Sprintf("(could not load diff: %v)", err))
		return
	}
	rendered, err := m.renderer.Render(raw)
	if err != nil {
		rendered = raw
	}
	m.diffView.SetContent(rendered)
}

const (
	logHintsLeft     = "j/k: navigate  Tab: panel  D: diff  ?: help  q: quit"
	logHintsRight    = "w: wrap  F: maximize  Tab: panel  ?: help  q: quit"
	logHintsMaximize = "F: restore  ?: help  q: quit"
)

// updateHints sets the status bar hints based on the current focus and maximize state.
func (m *LogModel) updateHints() {
	switch {
	case m.diffMaximized:
		m.statusBar.SetHints(logHintsMaximize)
	case m.focusRight:
		m.statusBar.SetHints(logHintsRight)
	default:
		m.statusBar.SetHints(logHintsLeft)
	}
}

// resize recalculates component dimensions after a terminal resize.
func (m *LogModel) resize() {
	contentHeight := m.height - 1
	m.statusBar.SetWidth(m.width)

	if !m.showDiff {
		panelW := m.width - 1
		m.commitList.SetWidth(panelW)
		m.commitList.SetHeight(contentHeight)
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

	m.commitList.SetWidth(leftW)
	m.commitList.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)
}
