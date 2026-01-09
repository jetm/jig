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

const defaultFixupCommitLimit = 20

// fixupItem wraps a git.CommitEntry for use with ItemList.
type fixupItem struct {
	entry git.CommitEntry
}

func (f fixupItem) Title() string {
	return tui.IconFixup + " " + f.entry.Hash + "  " + f.entry.Subject
}

func (f fixupItem) Description() string {
	return f.entry.Author + "  " + f.entry.Date
}

func (f fixupItem) FilterValue() string {
	return f.entry.Hash + " " + f.entry.Subject
}

// FixupModel is the command model for the fixup TUI.
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type FixupModel struct {
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

// NewFixupModel creates a FixupModel by loading recent commits from git log.
func NewFixupModel(
	ctx context.Context,
	runner git.Runner,
	_ config.Config,
	renderer diff.Renderer,
) *FixupModel {
	commits, _ := git.RecentCommits(ctx, runner, defaultFixupCommitLimit)
	branchName, _ := git.BranchName(ctx, runner)

	items := make([]list.Item, len(commits))
	for i, c := range commits {
		items[i] = fixupItem{entry: c}
	}

	m := &FixupModel{
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
					{Key: "Enter", Desc: "create fixup commit for selected"},
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

	m.statusBar.SetHints("j/k: navigate  Tab: panel  Enter: fixup  ?: help  q: quit")
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("fixup")

	// Render the first commit's diff if available
	if len(commits) > 0 {
		m.renderSelectedDiff()
	}

	return m
}

// Update handles messages and returns commands.
func (m *FixupModel) Update(msg tea.Msg) tea.Cmd {
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
			return m.confirmFixup()
		}

		if m.focusRight {
			dvCmd := m.diffView.Update(msg)
			return tea.Batch(sbCmd, dvCmd)
		}

		// Forward navigation to commit list
		listCmd := m.commitList.Update(msg)
		m.checkSelectionChange()
		return tea.Batch(sbCmd, listCmd)
	}

	return sbCmd
}

// View renders the two-panel layout.
func (m *FixupModel) View() string {
	if m.help.IsVisible() {
		return m.help.View(m.width, m.height)
	}

	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	if len(m.commits) == 0 {
		return "No commits to fixup."
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

// confirmFixup creates a fixup! commit for the currently selected commit.
func (m *FixupModel) confirmFixup() tea.Cmd {
	if len(m.commits) == 0 || m.selectedIdx >= len(m.commits) {
		return nil
	}
	hash := m.commits[m.selectedIdx].Hash
	err := git.CreateFixupCommit(m.ctx, m.runner, hash)
	if err != nil {
		errCmd := m.statusBar.SetMessage(fmt.Sprintf("Fixup failed: %v", err), components.Error)
		return errCmd
	}
	return func() tea.Msg {
		return app.PopModelMsg{MutatedGit: true}
	}
}

// checkSelectionChange detects if the list cursor moved and re-renders the diff.
func (m *FixupModel) checkSelectionChange() {
	sel := m.commitList.SelectedItem()
	if sel == nil {
		return
	}
	fi, ok := sel.(fixupItem)
	if !ok {
		return
	}
	for i, c := range m.commits {
		if c.Hash == fi.entry.Hash && i != m.selectedIdx {
			m.selectedIdx = i
			m.renderSelectedDiff()
			return
		}
	}
}

// renderSelectedDiff fetches and renders the selected commit's diff in the right panel.
func (m *FixupModel) renderSelectedDiff() {
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
func (m *FixupModel) resize() {
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
