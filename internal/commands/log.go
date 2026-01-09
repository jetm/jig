package commands

import (
	"context"
	"fmt"

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

	commits     []git.CommitEntry
	commitList  components.ItemList
	diffView    components.DiffView
	statusBar   components.StatusBar
	help        components.HelpOverlay
	branch      string
	selectedIdx int
	width       int
	height      int
	focusRight  bool
}

// NewLogModel creates a LogModel by loading recent commits from git log.
// If ref is non-empty, commits are loaded starting from that revision.
func NewLogModel(
	ctx context.Context,
	runner git.Runner,
	_ config.Config,
	renderer diff.Renderer,
	ref string,
) *LogModel {
	commits, _ := git.RecentCommitsFrom(ctx, runner, defaultLogCommitLimit, ref)
	branchName, _ := git.BranchName(ctx, runner)

	items := make([]list.Item, len(commits))
	for i, c := range commits {
		items[i] = logItem{entry: c}
	}

	m := &LogModel{
		ctx:        ctx,
		runner:     runner,
		renderer:   renderer,
		commits:    commits,
		commitList: components.NewItemList(items, 40, 20),
		diffView:   components.NewDiffView(80, 20),
		statusBar:  components.NewStatusBar(120),
		help: components.NewHelpOverlay([]components.KeyGroup{
			{
				Name: "Navigation",
				Bindings: []components.KeyBinding{
					{Key: "j/k", Desc: "move up/down"},
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
		branch:      branchName,
		selectedIdx: 0,
	}

	m.statusBar.SetHints("j/k: navigate  Tab: panel  ?: help  q: quit")
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("log")

	// Render the first commit's diff if available.
	if len(commits) > 0 {
		m.renderSelectedDiff()
	}

	return m
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

// View renders the two-panel layout.
func (m *LogModel) View() string {
	if m.help.IsVisible() {
		return m.help.View(m.width, m.height)
	}

	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	if len(m.commits) == 0 {
		return "No commits to show."
	}

	leftW, rightW := tui.Columns(m.width)
	contentHeight := m.height - 1 // reserve 1 row for status bar

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

	leftPanel := leftBorder.Width(leftW).Height(contentHeight).Render(m.commitList.View())
	rightPanel := rightBorder.Width(rightW).Height(contentHeight).Render(m.diffView.View())

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	m.statusBar.SetWidth(m.width)
	return panels + "\n" + m.statusBar.View()
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
	raw, err := git.CommitDiff(m.ctx, m.runner, hash)
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

// resize recalculates component dimensions after a terminal resize.
func (m *LogModel) resize() {
	leftW, rightW := tui.Columns(m.width)
	contentHeight := m.height - 1

	leftW--
	rightW--

	m.commitList.SetWidth(leftW)
	m.commitList.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)
	m.statusBar.SetWidth(m.width)
}
