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

// checkoutItem wraps a git.StatusFile for use with ItemList.
type checkoutItem struct {
	sf       git.StatusFile
	selected bool
}

func (c checkoutItem) Title() string {
	icon := tui.IconUnchecked
	if c.selected {
		icon = tui.IconChecked
	}
	return icon + " " + c.sf.Path
}

func (c checkoutItem) Description() string { return statusLabel(c.sf.Status) }
func (c checkoutItem) FilterValue() string { return c.sf.Path }

// CheckoutModel is the command model for the checkout TUI (interactive discard).
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type CheckoutModel struct {
	ctx        context.Context
	runner     git.Runner
	renderer   diff.Renderer
	files      []git.StatusFile
	selected   map[string]bool
	fileList   components.ItemList
	diffView   components.DiffView
	statusBar  components.StatusBar
	help       components.HelpOverlay
	branch     string
	width      int
	height     int
	confirming bool
}

// NewCheckoutModel creates a CheckoutModel by listing modified working-tree files.
func NewCheckoutModel(
	ctx context.Context,
	runner git.Runner,
	_ config.Config,
	renderer diff.Renderer,
) *CheckoutModel {
	files, _ := git.ListModifiedFiles(ctx, runner)
	branchName, _ := git.BranchName(ctx, runner)

	items := checkoutItemsFromFiles(files, nil)

	m := &CheckoutModel{
		ctx:       ctx,
		runner:    runner,
		renderer:  renderer,
		files:     files,
		selected:  make(map[string]bool),
		fileList:  components.NewItemList(items, 40, 20),
		diffView:  components.NewDiffView(80, 20),
		statusBar: components.NewStatusBar(120),
		help: components.NewHelpOverlay([]components.KeyGroup{
			{
				Name: "Navigation",
				Bindings: []components.KeyBinding{
					{Key: "j/k", Desc: "move up/down"},
					{Key: "Space", Desc: "toggle selection"},
					{Key: "a", Desc: "select all"},
					{Key: "d", Desc: "deselect all"},
					{Key: "?", Desc: "toggle help"},
				},
			},
			{
				Name: "Actions",
				Bindings: []components.KeyBinding{
					{Key: "Enter", Desc: "discard changes (with confirmation)"},
					{Key: "q/Esc", Desc: "quit without discarding"},
				},
			},
		}),
		branch: branchName,
	}

	m.statusBar.SetHints("Space: toggle  a: all  d: none  Enter: discard  ?: help  q: quit")
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("checkout")

	if len(files) > 0 {
		m.renderSelectedDiff()
	}

	return m
}

// Update handles messages.
func (m *CheckoutModel) Update(msg tea.Msg) tea.Cmd {
	sbCmd := m.statusBar.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return sbCmd

	case tea.KeyPressMsg:
		// Confirmation mode
		if m.confirming {
			return m.handleConfirmKey(msg, sbCmd)
		}

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
			paths := m.selectedPaths()
			if len(paths) == 0 {
				return sbCmd
			}
			m.confirming = true
			return sbCmd

		case ' ':
			m.toggleSelected()
			m.refreshList()
			return sbCmd

		case 'a':
			m.selectAll()
			m.refreshList()
			return sbCmd

		case 'd':
			m.deselectAll()
			m.refreshList()
			return sbCmd
		}

		listCmd := m.fileList.Update(msg)
		m.renderSelectedDiff()
		return tea.Batch(sbCmd, listCmd)
	}

	return sbCmd
}

// handleConfirmKey processes keys while the confirmation prompt is visible.
func (m *CheckoutModel) handleConfirmKey(msg tea.KeyPressMsg, sbCmd tea.Cmd) tea.Cmd {
	switch {
	case msg.Code == 'y' || msg.Text == "y":
		m.confirming = false
		return m.discardSelected()

	default:
		// Any other key cancels confirmation
		m.confirming = false
		return sbCmd
	}
}

// View renders the two-panel layout.
func (m *CheckoutModel) View() string {
	if m.help.IsVisible() {
		return m.help.View(m.width, m.height)
	}

	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	if len(m.files) == 0 {
		return "Nothing to discard."
	}

	leftW, rightW := tui.Columns(m.width)
	contentHeight := m.height - 1

	m.fileList.SetWidth(leftW)
	m.fileList.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)

	leftPanel := lipgloss.NewStyle().Width(leftW).Height(contentHeight).Render(m.fileList.View())
	rightPanel := lipgloss.NewStyle().Width(rightW).Height(contentHeight).Render(m.diffView.View())

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	m.statusBar.SetWidth(m.width)

	if m.confirming {
		paths := m.selectedPaths()
		prompt := fmt.Sprintf("Discard changes to %d file(s)? [y/N] ", len(paths))
		promptStyle := lipgloss.NewStyle().
			Foreground(tui.ColorYellow).
			Bold(true)
		return panels + "\n" + promptStyle.Render(prompt)
	}

	return panels + "\n" + m.statusBar.View()
}

// toggleSelected toggles selection state of the currently focused item.
func (m *CheckoutModel) toggleSelected() {
	sel := m.fileList.SelectedItem()
	if sel == nil {
		return
	}
	item, ok := sel.(checkoutItem)
	if !ok {
		return
	}
	m.selected[item.sf.Path] = !m.selected[item.sf.Path]
}

// selectAll marks all files as selected.
func (m *CheckoutModel) selectAll() {
	for _, f := range m.files {
		m.selected[f.Path] = true
	}
}

// deselectAll clears all selections.
func (m *CheckoutModel) deselectAll() {
	m.selected = make(map[string]bool)
}

// selectedPaths returns paths of selected files or the focused file if none selected.
func (m *CheckoutModel) selectedPaths() []string {
	var paths []string
	for _, f := range m.files {
		if m.selected[f.Path] {
			paths = append(paths, f.Path)
		}
	}
	if len(paths) > 0 {
		return paths
	}
	sel := m.fileList.SelectedItem()
	if sel == nil {
		return nil
	}
	item, ok := sel.(checkoutItem)
	if !ok {
		return nil
	}
	return []string{item.sf.Path}
}

// discardSelected runs git checkout -- for the selected files.
func (m *CheckoutModel) discardSelected() tea.Cmd {
	paths := m.selectedPaths()
	if len(paths) == 0 {
		return nil
	}
	err := git.DiscardFiles(m.ctx, m.runner, paths)
	mutated := err == nil
	if err != nil {
		msgCmd := m.statusBar.SetMessage(fmt.Sprintf("Discard failed: %v", err), components.Error)
		_ = msgCmd
	}
	return func() tea.Msg {
		return app.PopModelMsg{MutatedGit: mutated}
	}
}

// refreshList rebuilds the list items to reflect current selection state.
func (m *CheckoutModel) refreshList() {
	items := checkoutItemsFromFiles(m.files, m.selected)
	_ = m.fileList.SetItems(items)
}

// renderSelectedDiff renders the diff for the currently focused file.
func (m *CheckoutModel) renderSelectedDiff() {
	sel := m.fileList.SelectedItem()
	if sel == nil {
		return
	}
	item, ok := sel.(checkoutItem)
	if !ok {
		return
	}

	raw, err := m.runner.Run(m.ctx, "diff", "--", item.sf.Path)
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

// resize recalculates component dimensions after a terminal resize.
func (m *CheckoutModel) resize() {
	leftW, rightW := tui.Columns(m.width)
	contentHeight := m.height - 1

	m.fileList.SetWidth(leftW)
	m.fileList.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)
	m.statusBar.SetWidth(m.width)
}

// checkoutItemsFromFiles converts StatusFile slice to list.Item slice.
func checkoutItemsFromFiles(files []git.StatusFile, selected map[string]bool) []list.Item {
	items := make([]list.Item, len(files))
	for i, f := range files {
		items[i] = checkoutItem{sf: f, selected: selected[f.Path]}
	}
	return items
}
