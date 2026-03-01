package commands

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

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

const (
	fixupHintsLeft     = "Tab: panel  Enter: fixup  D: diff  ?: help  q: quit"
	fixupHintsRight    = "w: wrap  F: maximize  Tab: panel  ?: help  q: quit"
	fixupHintsMaximize = "F: restore  ?: help  q: quit"
)

// FixupModel is the command model for the fixup TUI.
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type FixupModel struct {
	twoPanelModel

	ctx      context.Context
	runner   git.Runner
	renderer diff.Renderer

	commits      []git.CommitEntry
	commitList   components.ItemList
	branch       string
	selectedIdx  int
	contextLines int
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

	commitList := components.NewCompactItemList(items, 40, 20)

	m := &FixupModel{
		twoPanelModel: newTwoPanelModel(
			&commitList,
			components.NewDiffView(80, 20),
			components.NewStatusBar(120),
			components.NewHelpOverlay([]components.KeyGroup{
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
						{Key: "/", Desc: "search in diff"},
						{Key: "n/N", Desc: "next/prev match"},
						{Key: "q/Esc", Desc: "quit"},
					},
				},
			}),
			cfg,
		),
		ctx:          ctx,
		runner:       runner,
		renderer:     renderer,
		commits:      commits,
		commitList:   commitList,
		branch:       branchName,
		selectedIdx:  0,
		contextLines: cfg.DiffContext,
	}

	m.setHints(fixupHintsLeft, fixupHintsRight, fixupHintsMaximize)
	m.status.SetBranch(branchName)
	m.status.SetMode("fixup")

	// Render the first commit's diff if available and diff panel is visible
	if len(commits) > 0 && m.showDiff {
		m.renderSelectedDiff()
	}

	return m, nil
}

// Update handles messages and returns commands.
func (m *FixupModel) Update(msg tea.Msg) tea.Cmd {
	sbCmd := m.status.Update(msg)

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

		if cmd, handled := m.handleKey(msg); handled {
			if cmd != nil {
				return cmd
			}
			if msg.String() == "D" && m.showDiff && len(m.commits) > 0 {
				m.renderSelectedDiff()
			}
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

		switch msg.Code {
		case 'q', tea.KeyEscape:
			return func() tea.Msg {
				return app.PopModelMsg{MutatedGit: false}
			}

		case tea.KeyEnter:
			return m.confirmFixup()
		}

		if m.focusRight {
			dvCmd := m.diff.Update(msg)
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
		m.left = &m.commitList
		background = m.renderLayout()
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
		return m.status.SetMessage(fmt.Sprintf("Fixup failed: %v", err), components.Error)
	}
	if err := git.AutosquashRebase(m.ctx, m.runner, hash); err != nil {
		return m.status.SetMessage(fmt.Sprintf("Rebase failed: %v", err), components.Error)
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
	raw, err := git.CommitDiff(m.ctx, m.runner, hash, m.contextLines)
	if err != nil {
		m.diff.SetContent(fmt.Sprintf("(could not load diff: %v)", err))
		return
	}
	rendered, err := m.renderer.Render(raw)
	if err != nil {
		rendered = raw
	}
	m.diff.SetContent(rendered)
}
