// Package commands provides TUI command models for jig subcommands.
package commands

import (
	"context"
	"fmt"
	"regexp"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/jig/internal/app"
	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/editor"
	"github.com/jetm/jig/internal/git"
	"github.com/jetm/jig/internal/tui"
	"github.com/jetm/jig/internal/tui/components"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

const (
	diffHintsLeft     = "j/k: navigate  Tab: panel  D: diff  ?: help  q: quit"
	diffHintsRight    = "w: wrap  F: maximize  Tab: panel  ?: help  q: quit"
	diffHintsMaximize = "j/k: files  F: restore  ?: help  q: quit"
)

// DiffModel is the command model for the diff TUI.
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type DiffModel struct {
	twoPanelModel

	ctx           context.Context
	runner        git.Runner
	files         []git.FileDiff
	fileList      components.FileList
	renderer      diff.Renderer
	branch        string
	selectedPath  string
	contextLines  int
	revision      string
	staged        bool
	pagerMode     bool
	filterPaths   []string
	noMatchFilter bool
}

// NewDiffModel creates a DiffModel by running git diff and parsing the output.
// When rawInput is non-empty, it is used as the diff content instead of running
// git diff (pager mode). filterPaths optionally restricts the file list to specific paths.
func NewDiffModel(
	ctx context.Context,
	runner git.Runner,
	cfg config.Config,
	renderer diff.Renderer,
	revision string,
	staged bool,
	rawInput string,
	filterPaths ...[]string,
) (*DiffModel, error) {
	var paths []string
	if len(filterPaths) > 0 {
		paths = ExpandGlobs(filterPaths[0])
	}

	var rawDiff string
	if rawInput != "" {
		rawDiff = ansiEscape.ReplaceAllString(rawInput, "")
	} else {
		args := git.DiffArgs(revision, staged, cfg.DiffContext)
		if len(paths) > 0 {
			args = append(args, "--")
			args = append(args, paths...)
		}
		var err error
		rawDiff, err = runner.Run(ctx, args...)
		if err != nil {
			return nil, fmt.Errorf("running git diff: %w", err)
		}
	}
	branchName, _ := git.BranchName(ctx, runner)

	files := git.ParseFileDiffs(rawDiff)

	entries := make([]components.FileEntry, len(files))
	for i, f := range files {
		entries[i] = components.FileEntry{Path: f.DisplayPath(), Status: f.Status}
	}

	noMatch := len(filterPaths) > 0 && len(filterPaths[0]) > 0 && len(files) == 0

	fileList := components.NewFileList(entries, false)

	m := &DiffModel{
		twoPanelModel: newTwoPanelModel(
			&fileList,
			components.NewDiffView(80, 20, cfg.ShowLineNumbers && !isDeltaRenderer(renderer)),
			components.NewStatusBar(120),
			components.NewHelpOverlay([]components.KeyGroup{
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
						{Key: "/", Desc: "search in diff"},
						{Key: "n/N", Desc: "next/prev match"},
						{Key: "q/Esc", Desc: "quit"},
					},
				},
			}),
			cfg,
		),
		ctx:           ctx,
		runner:        runner,
		files:         files,
		fileList:      fileList,
		filterPaths:   paths,
		noMatchFilter: noMatch,
		renderer:      renderer,
		branch:        branchName,
		contextLines:  cfg.DiffContext,
		revision:      revision,
		staged:        staged,
		pagerMode:     rawInput != "",
	}

	m.setHints(diffHintsLeft, diffHintsRight, diffHintsMaximize)
	m.status.SetBranch(branchName)
	if rawInput != "" {
		m.status.SetMode("diff (pager)")
	} else {
		m.status.SetMode("diff")
	}

	// Render first file if available and diff panel is visible
	if len(files) > 0 && m.showDiff {
		m.checkSelectionChange()
	}

	return m, nil
}

// Update handles messages and returns commands.
func (m *DiffModel) Update(msg tea.Msg) tea.Cmd {
	// Status bar always processes messages
	sbCmd := m.status.Update(msg)

	switch msg := msg.(type) {
	case editor.EditDiffMsg:
		if msg.Err != nil {
			_ = m.status.SetMessage(fmt.Sprintf("Edit failed: %v", msg.Err), components.Error)
			return sbCmd
		}
		// diff is read-only but we still attempt apply and refresh.
		if err := editor.ApplyEditedDiff(m.ctx, m.runner, msg.OriginalDiff, msg.EditedPath); err != nil {
			_ = m.status.SetMessage(fmt.Sprintf("Apply failed: %v", err), components.Error)
			return sbCmd
		}
		_ = m.status.SetMessage("Patch applied", components.Info)
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

		if cmd, handled := m.handleKey(msg); handled {
			if cmd != nil {
				return cmd
			}
			if msg.String() == "D" && m.showDiff && len(m.files) > 0 {
				m.checkSelectionChange()
			}
			return sbCmd
		}

		if msg.String() == "e" {
			// Edit the selected file's diff
			for _, f := range m.files {
				if f.DisplayPath() == m.selectedPath && f.RawDiff != "" {
					return editor.EditDiff(m.ctx, m.runner, f.RawDiff)
				}
			}
			return sbCmd
		}

		if msg.String() == "{" && !m.pagerMode {
			if m.contextLines > 0 {
				m.contextLines--
				m.refreshDiff()
			}
			return sbCmd
		}

		if msg.String() == "}" && !m.pagerMode {
			if m.contextLines < 20 {
				m.contextLines++
				m.refreshDiff()
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
			dvCmd := m.diff.Update(msg)
			return tea.Batch(sbCmd, dvCmd)
		}

		// Forward to file list
		listCmd := m.fileList.Update(msg)

		// Check if selection changed
		m.checkSelectionChange()

		return tea.Batch(sbCmd, listCmd)
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
		m.left = &m.fileList
		background = m.renderLayout()
	}

	return m.help.View(background, m.width, m.height)
}

// refreshDiff re-runs git diff with the current contextLines and rebuilds the file list.
func (m *DiffModel) refreshDiff() {
	args := git.DiffArgs(m.revision, m.staged, m.contextLines)
	if len(m.filterPaths) > 0 {
		args = append(args, "--")
		args = append(args, m.filterPaths...)
	}
	rawDiff, _ := m.runner.Run(m.ctx, args...)
	m.files = git.ParseFileDiffs(rawDiff)

	entries := make([]components.FileEntry, len(m.files))
	for i, f := range m.files {
		entries[i] = components.FileEntry{Path: f.DisplayPath(), Status: f.Status}
	}
	m.fileList = components.NewFileList(entries, false)
	m.left = &m.fileList
	m.selectedPath = ""
	if len(m.files) > 0 {
		m.checkSelectionChange()
	}
	m.resize()
}

// checkSelectionChange detects if the selected file changed and re-renders the diff.
func (m *DiffModel) checkSelectionChange() {
	path := m.fileList.SelectedPath()
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
			m.diff.SetDiffContent(f.RawDiff, rendered)
			return
		}
	}
}
