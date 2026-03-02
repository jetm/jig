package commands

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/jig/internal/app"
	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/editor"
	"github.com/jetm/jig/internal/git"
	"github.com/jetm/jig/internal/tui"
	"github.com/jetm/jig/internal/tui/components"
)

const (
	resetHintsLeft     = "Tab: panel  Enter: unstage  D: diff  ?: help  q: quit"
	resetHintsRight    = "w: wrap  F: maximize  Tab: panel  ?: help  q: quit"
	resetHintsMaximize = "F: restore  ?: help  q: quit"
)

// ResetModel is the command model for the reset TUI (interactive unstaging).
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type ResetModel struct {
	twoPanelModel

	ctx           context.Context
	runner        git.Runner
	renderer      diff.Renderer
	files         []git.StatusFile
	fileList      components.FileList
	branch        string
	contextLines  int
	filterPaths   []string
	noMatchFilter bool
}

// NewResetModel creates a ResetModel by listing staged files.
// filterPaths optionally restricts the file list to specific paths.
func NewResetModel(
	ctx context.Context,
	runner git.Runner,
	cfg config.Config,
	renderer diff.Renderer,
	filterPaths ...[]string,
) (*ResetModel, error) {
	var paths []string
	if len(filterPaths) > 0 {
		paths = ExpandGlobs(filterPaths[0])
	}
	files, err := git.ListStagedFilesFiltered(ctx, runner, paths)
	if err != nil {
		return nil, fmt.Errorf("listing staged files: %w", err)
	}
	branchName, _ := git.BranchName(ctx, runner)

	entries := make([]components.FileEntry, len(files))
	for i, f := range files {
		entries[i] = components.FileEntry{Path: f.Path, Status: f.Status}
	}

	noMatch := len(filterPaths) > 0 && len(filterPaths[0]) > 0 && len(files) == 0

	fileList := components.NewFileList(entries, true)

	m := &ResetModel{
		twoPanelModel: newTwoPanelModel(
			&fileList,
			components.NewDiffView(80, 20),
			components.NewStatusBar(120),
			components.NewHelpOverlay([]components.KeyGroup{
				{
					Name: "Navigation",
					Bindings: []components.KeyBinding{
						{Key: "j/k", Desc: "move up/down"},
						{Key: "o", Desc: "expand/collapse"},
						{Key: "Tab", Desc: "switch panel"},
						{Key: "D", Desc: "toggle diff"},
						{Key: "Space", Desc: "toggle selection"},
						{Key: "a", Desc: "select all"},
						{Key: "d", Desc: "deselect all"},
						{Key: "?", Desc: "toggle help"},
					},
				},
				{
					Name: "Actions",
					Bindings: []components.KeyBinding{
						{Key: "Enter", Desc: "unstage selected files"},
						{Key: "w", Desc: "toggle soft-wrap (diff panel)"},
						{Key: "F", Desc: "maximize diff panel"},
						{Key: "/", Desc: "search in diff"},
						{Key: "n/N", Desc: "next/prev match"},
						{Key: "q/Esc", Desc: "quit without unstaging"},
					},
				},
			}),
			cfg,
		),
		ctx:           ctx,
		runner:        runner,
		renderer:      renderer,
		files:         files,
		fileList:      fileList,
		filterPaths:   paths,
		noMatchFilter: noMatch,
		branch:        branchName,
		contextLines:  cfg.DiffContext,
	}

	m.setHints(resetHintsLeft, resetHintsRight, resetHintsMaximize)
	m.status.SetBranch(branchName)
	m.status.SetMode("reset")

	if len(files) > 0 && m.showDiff {
		m.renderSelectedDiff()
	}

	return m, nil
}

// Update handles messages.
func (m *ResetModel) Update(msg tea.Msg) tea.Cmd {
	sbCmd := m.status.Update(msg)

	switch msg := msg.(type) {
	case editor.EditDiffMsg:
		if msg.Err != nil {
			_ = m.status.SetMessage(fmt.Sprintf("Edit failed: %v", msg.Err), components.Error)
			return sbCmd
		}
		if err := editor.ApplyEditedDiff(m.ctx, m.runner, msg.OriginalDiff, msg.EditedPath); err != nil {
			_ = m.status.SetMessage(fmt.Sprintf("Apply failed: %v", err), components.Error)
			return sbCmd
		}
		m.renderSelectedDiff()
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
			// D key needs to trigger diff render
			if msg.String() == "D" && m.showDiff && len(m.files) > 0 {
				m.renderSelectedDiff()
			}
			return sbCmd
		}

		if msg.String() == "e" {
			path := m.fileList.SelectedPath()
			if path == "" {
				return sbCmd
			}
			rawDiff, err := m.runner.Run(m.ctx, "diff", "--cached", "--", path)
			if err != nil || rawDiff == "" {
				_ = m.status.SetMessage("No diff to edit", components.Info)
				return sbCmd
			}
			return editor.EditDiff(m.ctx, m.runner, rawDiff)
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
			return m.unstageSelected()

		case ' ':
			m.fileList.ToggleChecked()
			return sbCmd

		case 'a':
			m.fileList.SetAllChecked(true)
			return sbCmd

		case 'd':
			m.fileList.SetAllChecked(false)
			return sbCmd
		}

		if m.focusRight {
			dvCmd := m.diff.Update(msg)
			return tea.Batch(sbCmd, dvCmd)
		}

		treeCmd := m.fileList.Update(msg)
		m.renderSelectedDiff()
		return tea.Batch(sbCmd, treeCmd)
	}

	return sbCmd
}

// View renders the two-panel layout with the help overlay composited on top
// when visible.
func (m *ResetModel) View() string {
	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	var background string
	switch {
	case m.noMatchFilter:
		background = "No matching changes for the given paths."
	case len(m.files) == 0:
		background = "Nothing to unstage."
	default:
		m.left = &m.fileList
		background = m.renderLayout()
	}

	return m.help.View(background, m.width, m.height)
}

// unstageSelected runs git reset HEAD for the selected files and returns a PopModelMsg on success.
// On failure, the model stays visible with the error in the status bar.
func (m *ResetModel) unstageSelected() tea.Cmd {
	paths := m.fileList.SelectedOrCheckedPaths()
	if len(paths) == 0 {
		return nil
	}
	if err := git.UnstageFiles(m.ctx, m.runner, paths); err != nil {
		return m.status.SetMessage(fmt.Sprintf("Unstage failed: %v", err), components.Error)
	}
	return func() tea.Msg {
		return app.PopModelMsg{MutatedGit: true}
	}
}

// renderSelectedDiff renders the staged diff for the currently focused file.
func (m *ResetModel) renderSelectedDiff() {
	path := m.fileList.SelectedPath()
	if path == "" {
		return
	}

	// Run git diff --cached for this specific file
	raw, err := m.runner.Run(m.ctx, "diff", fmt.Sprintf("-U%d", m.contextLines), "--cached", "--", path)
	if err != nil || raw == "" {
		m.diff.SetContent("(no diff available)")
		return
	}

	rendered, err := m.renderer.Render(raw)
	if err != nil {
		rendered = raw
	}
	m.diff.SetContent(rendered)
}
