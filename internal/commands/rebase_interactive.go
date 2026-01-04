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

// actionIcon returns the Nerd Font icon for a rebase action.
func actionIcon(a git.RebaseAction) string {
	switch a {
	case git.ActionPick:
		return tui.IconPick
	case git.ActionReword:
		return tui.IconReword
	case git.ActionEdit:
		return tui.IconEdit
	case git.ActionSquash:
		return tui.IconSquash
	case git.ActionFixup:
		return tui.IconFixup
	case git.ActionDrop:
		return tui.IconDrop
	default:
		return tui.IconPick
	}
}

// rebaseItem wraps a git.RebaseTodoEntry for use with ItemList.
type rebaseItem struct {
	entry git.RebaseTodoEntry
}

func (r rebaseItem) Title() string {
	return fmt.Sprintf("%s %-6s  %s  %s",
		actionIcon(r.entry.Action),
		string(r.entry.Action),
		r.entry.Hash,
		r.entry.Subject,
	)
}

func (r rebaseItem) Description() string {
	return fmt.Sprintf("action: %s  hash: %s", r.entry.Action, r.entry.Hash)
}

func (r rebaseItem) FilterValue() string {
	return r.entry.Hash + " " + r.entry.Subject
}

// RebaseInteractiveModel is the TUI model for interactive rebase.
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type RebaseInteractiveModel struct {
	ctx      context.Context
	runner   git.Runner
	renderer diff.Renderer

	base        string
	entries     []git.RebaseTodoEntry
	commitList  components.ItemList
	diffView    components.DiffView
	statusBar   components.StatusBar
	help        components.HelpOverlay
	branch      string
	selectedIdx int
	width       int
	height      int
}

// NewRebaseInteractiveModel creates a RebaseInteractiveModel.
// base is the revision to rebase from (e.g. "HEAD~5").
func NewRebaseInteractiveModel(
	ctx context.Context,
	runner git.Runner,
	_ config.Config,
	renderer diff.Renderer,
	base string,
) *RebaseInteractiveModel {
	if base == "" {
		base = "HEAD~10"
	}

	entries, _ := git.CommitsForRebase(ctx, runner, base)
	branchName, _ := git.BranchName(ctx, runner)

	items := make([]list.Item, len(entries))
	for i, e := range entries {
		items[i] = rebaseItem{entry: e}
	}

	m := &RebaseInteractiveModel{
		ctx:        ctx,
		runner:     runner,
		renderer:   renderer,
		base:       base,
		entries:    entries,
		commitList: components.NewItemList(items, 40, 20),
		diffView:   components.NewDiffView(80, 20),
		statusBar:  components.NewStatusBar(120),
		help: components.NewHelpOverlay([]components.KeyGroup{
			{
				Name: "Navigation",
				Bindings: []components.KeyBinding{
					{Key: "j/k or ↑/↓", Desc: "move cursor"},
					{Key: "Ctrl+Up / K", Desc: "move commit up"},
					{Key: "Ctrl+Down / J", Desc: "move commit down"},
					{Key: "?", Desc: "toggle help"},
				},
			},
			{
				Name: "Actions",
				Bindings: []components.KeyBinding{
					{Key: "Space", Desc: "cycle action"},
					{Key: "p", Desc: "pick"},
					{Key: "r", Desc: "reword"},
					{Key: "e", Desc: "edit"},
					{Key: "s", Desc: "squash"},
					{Key: "f", Desc: "fixup"},
					{Key: "d", Desc: "drop"},
					{Key: "Enter", Desc: "confirm & execute rebase"},
					{Key: "q/Esc", Desc: "abort"},
				},
			},
		}),
		branch:      branchName,
		selectedIdx: 0,
	}

	m.statusBar.SetHints("Space: cycle action  p/r/e/s/f/d: set action  K/J: reorder  Enter: confirm  ?: help  q: quit")
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("rebase-interactive")

	if len(entries) > 0 {
		m.renderSelectedDiff()
	}

	return m
}

// Update handles messages and returns commands.
func (m *RebaseInteractiveModel) Update(msg tea.Msg) tea.Cmd {
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

		switch msg.Code {
		case 'q', tea.KeyEscape:
			return func() tea.Msg {
				return app.PopModelMsg{MutatedGit: false}
			}

		case tea.KeyEnter:
			return m.confirmRebase()

		case ' ':
			m.cycleAction()
			m.refreshList()
			return sbCmd

		case 'p':
			m.setAction(git.ActionPick)
			m.refreshList()
			return sbCmd

		case 'r':
			m.setAction(git.ActionReword)
			m.refreshList()
			return sbCmd

		case 'e':
			m.setAction(git.ActionEdit)
			m.refreshList()
			return sbCmd

		case 's':
			m.setAction(git.ActionSquash)
			m.refreshList()
			return sbCmd

		case 'f':
			m.setAction(git.ActionFixup)
			m.refreshList()
			return sbCmd

		case 'd':
			m.setAction(git.ActionDrop)
			m.refreshList()
			return sbCmd

		case 'K':
			m.moveUp()
			return sbCmd

		case 'J':
			m.moveDown()
			return sbCmd

		case tea.KeyUp:
			if msg.Mod.Contains(tea.ModCtrl) {
				m.moveUp()
				return sbCmd
			}

		case tea.KeyDown:
			if msg.Mod.Contains(tea.ModCtrl) {
				m.moveDown()
				return sbCmd
			}
		}

		// Forward navigation to list
		listCmd := m.commitList.Update(msg)
		m.checkSelectionChange()
		return tea.Batch(sbCmd, listCmd)
	}

	return sbCmd
}

// View renders the two-panel layout.
func (m *RebaseInteractiveModel) View() string {
	if m.help.IsVisible() {
		return m.help.View(m.width, m.height)
	}

	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	if len(m.entries) == 0 {
		return "No commits to rebase. Specify a valid base revision."
	}

	leftW, rightW := tui.Columns(m.width)
	contentHeight := m.height - 1 // reserve 1 row for status bar

	m.commitList.SetWidth(leftW)
	m.commitList.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)

	leftPanel := lipgloss.NewStyle().Width(leftW).Height(contentHeight).Render(m.commitList.View())
	rightPanel := lipgloss.NewStyle().Width(rightW).Height(contentHeight).Render(m.diffView.View())

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	m.statusBar.SetWidth(m.width)
	return panels + "\n" + m.statusBar.View()
}

// confirmRebase executes the interactive rebase with the current todo.
func (m *RebaseInteractiveModel) confirmRebase() tea.Cmd {
	if len(m.entries) == 0 {
		return nil
	}
	err := git.ExecuteRebaseInteractive(m.ctx, m.runner, m.base, m.entries)
	if err != nil {
		return m.statusBar.SetMessage(fmt.Sprintf("Rebase failed: %v", err), components.Error)
	}
	return func() tea.Msg {
		return app.PopModelMsg{MutatedGit: true}
	}
}

// cycleAction cycles the selected entry's action to the next in the sequence.
func (m *RebaseInteractiveModel) cycleAction() {
	if m.selectedIdx >= len(m.entries) {
		return
	}
	m.entries[m.selectedIdx].Action = git.NextAction(m.entries[m.selectedIdx].Action)
}

// setAction sets the selected entry's action to a specific value.
func (m *RebaseInteractiveModel) setAction(a git.RebaseAction) {
	if m.selectedIdx >= len(m.entries) {
		return
	}
	m.entries[m.selectedIdx].Action = a
}

// moveUp swaps the selected entry with the one above it (reorders commits upward).
func (m *RebaseInteractiveModel) moveUp() {
	if m.selectedIdx <= 0 || len(m.entries) < 2 {
		return
	}
	m.entries[m.selectedIdx], m.entries[m.selectedIdx-1] = m.entries[m.selectedIdx-1], m.entries[m.selectedIdx]
	m.selectedIdx--
	m.refreshList()
	m.renderSelectedDiff()
}

// moveDown swaps the selected entry with the one below it (reorders commits downward).
func (m *RebaseInteractiveModel) moveDown() {
	if m.selectedIdx >= len(m.entries)-1 || len(m.entries) < 2 {
		return
	}
	m.entries[m.selectedIdx], m.entries[m.selectedIdx+1] = m.entries[m.selectedIdx+1], m.entries[m.selectedIdx]
	m.selectedIdx++
	m.refreshList()
	m.renderSelectedDiff()
}

// refreshList rebuilds the list items from entries and keeps the cursor at selectedIdx.
func (m *RebaseInteractiveModel) refreshList() {
	items := make([]list.Item, len(m.entries))
	for i, e := range m.entries {
		items[i] = rebaseItem{entry: e}
	}
	_ = m.commitList.SetItems(items)
	m.commitList.Select(m.selectedIdx)
}

// checkSelectionChange detects if the list cursor moved and re-renders the diff.
func (m *RebaseInteractiveModel) checkSelectionChange() {
	sel := m.commitList.SelectedItem()
	if sel == nil {
		return
	}
	ri, ok := sel.(rebaseItem)
	if !ok {
		return
	}
	for i, e := range m.entries {
		if e.Hash == ri.entry.Hash && i != m.selectedIdx {
			m.selectedIdx = i
			m.renderSelectedDiff()
			return
		}
	}
}

// renderSelectedDiff fetches and renders the selected commit's diff.
func (m *RebaseInteractiveModel) renderSelectedDiff() {
	if m.selectedIdx >= len(m.entries) {
		return
	}
	hash := m.entries[m.selectedIdx].Hash
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
func (m *RebaseInteractiveModel) resize() {
	leftW, rightW := tui.Columns(m.width)
	contentHeight := m.height - 1

	m.commitList.SetWidth(leftW)
	m.commitList.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)
	m.statusBar.SetWidth(m.width)
}
