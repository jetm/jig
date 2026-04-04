package commands

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

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
	addHintsLeft     = "Tab: panel  Enter: stage  c: commit  D: diff  ?: help  q: quit"
	addHintsRight    = "w: wrap  F: maximize  Tab: panel  ?: help  q: quit"
	addHintsMaximize = "F: restore  ?: help  q: quit"
)

// AddModel is the command model for the add TUI (interactive staging).
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type AddModel struct {
	twoPanelModel

	ctx           context.Context
	runner        git.Runner
	renderer      diff.Renderer
	files         []git.StatusFile
	fileList      components.FileList
	branch        string
	contextLines  int
	filterPaths   []string // optional paths to filter the file list
	noMatchFilter bool     // true when filterPaths produced no matching changes
}

// NewAddModel creates an AddModel by listing unstaged files.
// filterPaths optionally restricts the file list to specific paths (expanded from globs).
func NewAddModel(
	ctx context.Context,
	runner git.Runner,
	cfg config.Config,
	renderer diff.Renderer,
	filterPaths ...[]string,
) (*AddModel, error) {
	var paths []string
	if len(filterPaths) > 0 {
		paths = ExpandGlobs(filterPaths[0])
	}
	files, err := git.ListUnstagedFilesFiltered(ctx, runner, paths)
	if err != nil {
		return nil, fmt.Errorf("listing unstaged files: %w", err)
	}
	branchName, _ := git.BranchName(ctx, runner)

	entries := make([]components.FileEntry, len(files))
	for i, f := range files {
		entries[i] = components.FileEntry{Path: f.Path, Status: f.Status}
	}

	noMatch := len(filterPaths) > 0 && len(filterPaths[0]) > 0 && len(files) == 0

	fileList := components.NewFileList(entries, true)

	m := &AddModel{
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
						{Key: "Space", Desc: "toggle selection"},
						{Key: "a", Desc: "select all"},
						{Key: "d", Desc: "deselect all"},
						{Key: "?", Desc: "toggle help"},
					},
				},
				{
					Name: "Actions",
					Bindings: []components.KeyBinding{
						{Key: "Enter", Desc: "stage selected files"},
						{Key: "c", Desc: "stage and commit"},
						{Key: "C", Desc: "stage and commit (title only)"},
						{Key: "w", Desc: "toggle soft-wrap (diff panel)"},
						{Key: "F", Desc: "maximize diff panel"},
						{Key: "/", Desc: "search in diff"},
						{Key: "n/N", Desc: "next/prev match"},
						{Key: "q/Esc", Desc: "quit without staging"},
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

	m.setHints(addHintsLeft, addHintsRight, addHintsMaximize)
	m.status.SetBranch(branchName)
	m.status.SetMode("add")

	if len(files) > 0 && m.showDiff {
		m.renderSelectedDiff()
	}

	return m, nil
}

// Update handles messages.
func (m *AddModel) Update(msg tea.Msg) tea.Cmd {
	sbCmd := m.status.Update(msg)

	switch msg := msg.(type) {
	case CommitDoneMsg:
		if msg.Err != nil {
			_ = m.status.SetMessage("Commit aborted", components.Info)
			m.refreshFiles()
			return sbCmd
		}
		return func() tea.Msg {
			return app.PopModelMsg{MutatedGit: true}
		}

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
			if msg.String() == "D" && m.showDiff && len(m.files) > 0 {
				m.renderSelectedDiff()
			}
			if m.leftUpdated {
				m.leftUpdated = false
				m.fileList.Update(msg)
				m.renderSelectedDiff()
			}
			return sbCmd
		}

		if msg.String() == "e" {
			path := m.fileList.SelectedPath()
			if path == "" {
				return sbCmd
			}
			rawDiff, err := m.runner.Run(m.ctx, "diff", "--", path)
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

		if msg.String() == "c" {
			return m.execCommit(false)
		}

		if msg.String() == "C" {
			return m.execCommit(true)
		}

		switch msg.Code {
		case 'q', tea.KeyEscape:
			return func() tea.Msg {
				return app.PopModelMsg{MutatedGit: false}
			}

		case tea.KeyEnter:
			return m.stageSelected()

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
func (m *AddModel) View() string {
	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	var background string
	switch {
	case m.noMatchFilter:
		background = "No matching changes for the given paths."
	case len(m.files) == 0:
		background = "Nothing to stage."
	default:
		m.left = &m.fileList
		background = m.renderLayout()
	}

	return m.help.View(background, m.width, m.height)
}

// stageSelected runs git add for the selected files and returns a PopModelMsg on success.
// On failure, the model stays visible with the error in the status bar.
func (m *AddModel) stageSelected() tea.Cmd {
	paths := m.fileList.SelectedOrCheckedPaths()
	if len(paths) == 0 {
		return nil
	}
	if err := git.StageFiles(m.ctx, m.runner, paths); err != nil {
		return m.status.SetMessage(fmt.Sprintf("Stage failed: %v", err), components.Error)
	}
	return func() tea.Msg {
		return app.PopModelMsg{MutatedGit: true}
	}
}

// renderSelectedDiff renders the diff for the currently focused file.
func (m *AddModel) renderSelectedDiff() {
	path := m.fileList.SelectedPath()
	if path == "" {
		return
	}

	// For untracked files, use --no-index to produce a diff against /dev/null.
	// git diff --no-index exits 1 when files differ, so use RunAllowExitCode.
	sf := m.findFile(path)
	var raw string
	var err error
	contextArg := fmt.Sprintf("-U%d", m.contextLines)
	if sf != nil && sf.Status == git.Added && !m.isTracked(path) {
		raw, err = m.runner.RunAllowExitCode(m.ctx, 1, "diff", contextArg, "--no-index", "--", "/dev/null", path)
	} else {
		raw, err = m.runner.Run(m.ctx, "diff", contextArg, "--", path)
	}
	if err != nil || raw == "" {
		m.diff.SetContent("(no diff available)")
		return
	}

	rendered, err := m.renderer.Render(raw)
	if err != nil {
		rendered = raw
	}
	m.diff.SetDiffContent(raw, rendered)
}

// findFile returns the StatusFile for the given path, or nil.
func (m *AddModel) findFile(path string) *git.StatusFile {
	for i := range m.files {
		if m.files[i].Path == path {
			return &m.files[i]
		}
	}
	return nil
}

// isTracked returns true if the file path appears in the tracked working-tree diff
// (i.e., is not purely untracked). Used to decide whether to show diff or placeholder.
func (m *AddModel) isTracked(path string) bool {
	for _, f := range m.files {
		if f.Path == path && f.Status != git.Added {
			return true
		}
	}
	return false
}

// execCommit stages selected files and launches the configured commit command.
// If titleOnly is true, appends CommitTitleOnlyFlag (when non-empty).
func (m *AddModel) execCommit(titleOnly bool) tea.Cmd {
	paths := m.fileList.SelectedOrCheckedPaths()
	if len(paths) == 0 {
		return nil
	}
	if err := git.StageFiles(m.ctx, m.runner, paths); err != nil {
		_ = m.status.SetMessage(fmt.Sprintf("Stage failed: %v", err), components.Error)
		return nil
	}
	// Split command string to handle multi-word values like "devtool commit".
	parts := strings.Fields(m.cfg.CommitCmd)
	args := make([]string, len(parts)-1, len(parts))
	copy(args, parts[1:])
	if titleOnly && m.cfg.CommitTitleOnlyFlag != "" {
		args = append(args, m.cfg.CommitTitleOnlyFlag)
	}
	cmd := exec.Command(parts[0], args...) //nolint:gosec // commit command is user-configured
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return CommitDoneMsg{Err: err}
	})
}

// refreshFiles re-queries git state and rebuilds the file list.
// Called after a commit completes to reflect the new index state.
func (m *AddModel) refreshFiles() {
	files, _ := git.ListUnstagedFilesFiltered(m.ctx, m.runner, m.filterPaths)
	m.files = files

	entries := make([]components.FileEntry, len(files))
	for i, f := range files {
		entries[i] = components.FileEntry{Path: f.Path, Status: f.Status}
	}
	m.fileList = components.NewFileList(entries, true)
	m.left = &m.fileList

	if len(files) > 0 && m.showDiff {
		m.renderSelectedDiff()
	}
	m.resize()
}
