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

const (
	logHintsLeft     = "j/k: navigate  Tab: panel  D: diff  ?: help  q: quit"
	logHintsRight    = "w: wrap  F: maximize  Tab: panel  ?: help  q: quit"
	logHintsMaximize = "F: restore  ?: help  q: quit"
)

// LogModel is the command model for the log TUI.
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type LogModel struct {
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

	commitList := components.NewCompactItemList(items, 40, 20)

	m := &LogModel{
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

	m.setHints(logHintsLeft, logHintsRight, logHintsMaximize)
	m.status.SetBranch(branchName)
	m.status.SetMode("log")

	// Render the first commit's diff if available and diff panel is visible.
	if len(commits) > 0 && m.showDiff {
		m.renderSelectedDiff()
	}

	return m, nil
}

// Update handles messages and returns commands.
func (m *LogModel) Update(msg tea.Msg) tea.Cmd {
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
		}

		if m.focusRight {
			dvCmd := m.diff.Update(msg)
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
		m.left = &m.commitList
		background = m.renderLayout()
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
		m.diff.SetContent(fmt.Sprintf("(could not load diff: %v)", err))
		return
	}
	rendered, err := m.renderer.Render(raw)
	if err != nil {
		rendered = raw
	}
	m.diff.SetContent(rendered)
}
