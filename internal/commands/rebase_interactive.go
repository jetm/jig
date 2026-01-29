package commands

import (
	"context"
	"fmt"
	"os"

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

// AbortEditorMsg signals that the editor mode was aborted, requesting non-zero exit.
// The app model handles this by setting Aborted=true and quitting.
type AbortEditorMsg = app.AbortMsg

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

	base         string
	todoFilePath string // non-empty = editor mode (invoked as $GIT_SEQUENCE_EDITOR)
	entries      []git.RebaseTodoEntry
	commitList   components.ItemList
	diffView     components.DiffView
	statusBar    components.StatusBar
	help         components.HelpOverlay
	branch       string
	selectedIdx  int
	width        int
	height       int
	focusRight   bool
	showDiff     bool
}

// NewRebaseInteractiveModel creates a RebaseInteractiveModel.
// base is the revision to rebase from (e.g. "HEAD~5") for standalone mode.
// todoFilePath is the path to the git rebase todo file for editor mode.
// When todoFilePath is non-empty, the model operates in editor mode.
func NewRebaseInteractiveModel(
	ctx context.Context,
	runner git.Runner,
	cfg config.Config,
	renderer diff.Renderer,
	base string,
	todoFilePath string,
) *RebaseInteractiveModel {
	var entries []git.RebaseTodoEntry

	if todoFilePath != "" {
		// Editor mode: parse the todo file
		raw, err := os.ReadFile(todoFilePath)
		if err == nil {
			entries = git.ParseNativeTodo(string(raw))
		}
	} else {
		// Standalone mode: fetch commits via git log
		if base == "" {
			base = "HEAD~10"
		}
		entries, _ = git.CommitsForRebase(ctx, runner, base)
	}
	branchName, _ := git.BranchName(ctx, runner)

	items := make([]list.Item, len(entries))
	for i, e := range entries {
		items[i] = rebaseItem{entry: e}
	}

	m := &RebaseInteractiveModel{
		ctx:          ctx,
		runner:       runner,
		renderer:     renderer,
		base:         base,
		todoFilePath: todoFilePath,
		entries:      entries,
		commitList:   components.NewCompactItemList(items, 40, 20),
		diffView:     components.NewDiffView(80, 20),
		statusBar:    components.NewStatusBar(120),
		help: components.NewHelpOverlay([]components.KeyGroup{
			{
				Name: "Navigation",
				Bindings: []components.KeyBinding{
					{Key: "j/k or ↑/↓", Desc: "move cursor"},
					{Key: "Tab", Desc: "switch panel"},
					{Key: "Ctrl+Up / K", Desc: "move commit up"},
					{Key: "Ctrl+Down / J", Desc: "move commit down"},
					{Key: "D", Desc: "toggle diff"},
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

	// Editor mode always starts with diff hidden; standalone reads from config.
	if todoFilePath == "" {
		m.showDiff = cfg.ShowDiffPanel
		if m.showDiff && len(entries) > 0 {
			m.renderSelectedDiff()
		}
	}

	m.updateHints()
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("rebase-interactive")

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
		if msg.String() == "?" {
			m.help.Toggle()
			return sbCmd
		}

		if m.help.IsVisible() {
			return sbCmd
		}

		switch msg.String() {
		case "tab":
			if m.showDiff {
				m.focusRight = !m.focusRight
				m.updateHints()
			}
			return sbCmd

		case "q", "esc":
			if m.todoFilePath != "" {
				return func() tea.Msg { return AbortEditorMsg{} }
			}
			return func() tea.Msg {
				return app.PopModelMsg{MutatedGit: false}
			}

		case "enter":
			return m.confirmRebase()

		case "space":
			m.cycleAction()
			m.refreshList()
			return sbCmd

		case "p":
			m.setAction(git.ActionPick)
			m.refreshList()
			return sbCmd

		case "r":
			m.setAction(git.ActionReword)
			m.refreshList()
			return sbCmd

		case "e":
			m.setAction(git.ActionEdit)
			m.refreshList()
			return sbCmd

		case "s":
			m.setAction(git.ActionSquash)
			m.refreshList()
			return sbCmd

		case "f":
			m.setAction(git.ActionFixup)
			m.refreshList()
			return sbCmd

		case "d":
			m.setAction(git.ActionDrop)
			m.refreshList()
			return sbCmd

		case "D":
			m.showDiff = !m.showDiff
			if m.showDiff && len(m.entries) > 0 {
				m.renderSelectedDiff()
			}
			return sbCmd

		case "K":
			m.moveUp()
			return sbCmd

		case "J":
			m.moveDown()
			return sbCmd

		case "ctrl+up":
			m.moveUp()
			return sbCmd

		case "ctrl+down":
			m.moveDown()
			return sbCmd
		}

		if m.focusRight {
			dvCmd := m.diffView.Update(msg)
			return tea.Batch(sbCmd, dvCmd)
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

	contentHeight := m.height - 1 // reserve 1 row for status bar
	m.statusBar.SetWidth(m.width)

	if !m.showDiff {
		// Single-panel mode: left panel fills the full terminal width
		panelW := m.width - 1
		m.commitList.SetWidth(panelW)
		m.commitList.SetHeight(contentHeight)
		leftPanel := tui.StyleFocusBorder.Width(panelW).Height(contentHeight).MaxHeight(contentHeight).Render(m.commitList.View())
		return leftPanel + "\n" + m.statusBar.View()
	}

	leftW, rightW := tui.ColumnsWide(m.width)

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

	return panels + "\n" + m.statusBar.View()
}

// confirmRebase executes the interactive rebase with the current todo.
// In editor mode, it writes the todo back to the file and exits cleanly.
// In standalone mode, it calls ExecuteRebaseInteractive.
func (m *RebaseInteractiveModel) confirmRebase() tea.Cmd {
	if len(m.entries) == 0 {
		return nil
	}

	if m.todoFilePath != "" {
		// Editor mode: write modified todo back to file
		todo := git.FormatTodo(m.entries)
		if err := os.WriteFile(m.todoFilePath, []byte(todo), 0o644); err != nil {
			return m.statusBar.SetMessage(fmt.Sprintf("Write todo file: %v", err), components.Error)
		}
		return func() tea.Msg {
			return app.PopModelMsg{MutatedGit: false}
		}
	}

	// Standalone mode: execute rebase
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
	if m.showDiff {
		m.renderSelectedDiff()
	}
}

// moveDown swaps the selected entry with the one below it (reorders commits downward).
func (m *RebaseInteractiveModel) moveDown() {
	if m.selectedIdx >= len(m.entries)-1 || len(m.entries) < 2 {
		return
	}
	m.entries[m.selectedIdx], m.entries[m.selectedIdx+1] = m.entries[m.selectedIdx+1], m.entries[m.selectedIdx]
	m.selectedIdx++
	m.refreshList()
	if m.showDiff {
		m.renderSelectedDiff()
	}
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
			if m.showDiff {
				m.renderSelectedDiff()
			}
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

const (
	rebaseHintsLeft  = "Space: cycle action  p/r/e/s/f/d: set action  K/J: reorder  Tab: panel  D: diff  Enter: confirm  ?: help  q: quit"
	rebaseHintsRight = "h/l: scroll  Tab: panel  D: diff  ?: help  q: quit"
)

// updateHints sets the status bar hints based on the current focus.
func (m *RebaseInteractiveModel) updateHints() {
	if m.focusRight {
		m.statusBar.SetHints(rebaseHintsRight)
	} else {
		m.statusBar.SetHints(rebaseHintsLeft)
	}
}

// resize recalculates component dimensions after a terminal resize.
func (m *RebaseInteractiveModel) resize() {
	contentHeight := m.height - 1
	m.statusBar.SetWidth(m.width)

	if !m.showDiff {
		panelW := m.width - 1
		m.commitList.SetWidth(panelW)
		m.commitList.SetHeight(contentHeight)
		return
	}

	leftW, rightW := tui.ColumnsWide(m.width)

	leftW--
	rightW--

	m.commitList.SetWidth(leftW)
	m.commitList.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)
}
