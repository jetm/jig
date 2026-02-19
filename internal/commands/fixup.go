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
	focusRight    bool
	showDiff      bool
	diffMaximized bool
}

// NewFixupModel creates a FixupModel by loading recent commits from git log.
// It returns an error if preconditions are not met (nothing staged, rebase in
// progress) or if git commands fail during initialization.
func NewFixupModel(
	ctx context.Context,
	runner git.Runner,
	cfg config.Config,
	renderer diff.Renderer,
) (*FixupModel, error) {
	if !git.HasStagedChanges(ctx, runner) {
		return nil, fmt.Errorf("nothing staged")
	}
	if git.IsRebaseInProgress(ctx, runner) {
		return nil, fmt.Errorf("rebase in progress")
	}

	commits, err := git.RecentCommits(ctx, runner, defaultFixupCommitLimit)
	if err != nil {
		return nil, fmt.Errorf("loading commits: %w", err)
	}
	branchName, err := git.BranchName(ctx, runner)
	if err != nil {
		return nil, fmt.Errorf("getting branch: %w", err)
	}

	items := make([]list.Item, len(commits))
	for i, c := range commits {
		items[i] = fixupItem{entry: c}
	}

	m := &FixupModel{
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
					{Key: "Enter", Desc: "create fixup commit for selected"},
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
		branch:      branchName,
		selectedIdx: 0,
		panelRatio:  cfg.PanelRatio,
	}

	m.showDiff = cfg.ShowDiffPanel
	m.diffView.SetSoftWrap(cfg.SoftWrap)

	m.updateHints()
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("fixup")

	// Render the first commit's diff if available and diff panel is visible
	if len(commits) > 0 && m.showDiff {
		m.renderSelectedDiff()
	}

	return m, nil
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

// View renders the two-panel layout with the help overlay composited on top
// when visible.
func (m *FixupModel) View() string {
	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	var background string
	if len(m.commits) == 0 {
		background = "No commits to fixup."
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

// confirmFixup creates a fixup! commit for the currently selected commit,
// then runs an autosquash rebase to squash it into the target.
func (m *FixupModel) confirmFixup() tea.Cmd {
	if len(m.commits) == 0 || m.selectedIdx >= len(m.commits) {
		return nil
	}
	hash := m.commits[m.selectedIdx].Hash
	if err := git.CreateFixupCommit(m.ctx, m.runner, hash); err != nil {
		return m.statusBar.SetMessage(fmt.Sprintf("Fixup failed: %v", err), components.Error)
	}
	if err := git.AutosquashRebase(m.ctx, m.runner, hash); err != nil {
		return m.statusBar.SetMessage(fmt.Sprintf("Rebase failed: %v", err), components.Error)
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

const (
	fixupHintsLeft     = "Tab: panel  Enter: fixup  D: diff  ?: help  q: quit"
	fixupHintsRight    = "w: wrap  F: maximize  Tab: panel  ?: help  q: quit"
	fixupHintsMaximize = "F: restore  ?: help  q: quit"
)

// updateHints sets the status bar hints based on the current focus and maximize state.
func (m *FixupModel) updateHints() {
	switch {
	case m.diffMaximized:
		m.statusBar.SetHints(fixupHintsMaximize)
	case m.focusRight:
		m.statusBar.SetHints(fixupHintsRight)
	default:
		m.statusBar.SetHints(fixupHintsLeft)
	}
}

// resize recalculates component dimensions after a terminal resize.
func (m *FixupModel) resize() {
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
