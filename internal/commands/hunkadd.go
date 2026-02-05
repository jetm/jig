package commands

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jetm/gti/internal/app"
	"github.com/jetm/gti/internal/config"
	"github.com/jetm/gti/internal/diff"
	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/tui"
	"github.com/jetm/gti/internal/tui/components"
)

// HunkAddModel is the command model for the hunk-add TUI (interactive hunk-level staging).
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type HunkAddModel struct {
	ctx      context.Context
	runner   git.Runner
	renderer diff.Renderer
	cfg      config.Config

	files         []git.FileDiff
	fileList      components.FileList
	diffView      components.DiffView
	statusBar     components.StatusBar
	help          components.HelpOverlay
	branch        string
	width         int
	height        int
	panelRatio    int
	fileIdx       int          // index into files for the current file
	hunks         []git.Hunk   // hunks of the current file
	hunkIdx       int          // index into hunks (current hunk)
	decided       map[int]bool // hunkIdx -> staged (true) or skipped (false); nil means undecided
	allDecided    bool         // true once all hunks in current file are decided
	focusRight    bool
	showDiff      bool
	diffMaximized bool
	filterPaths   []string
	noMatchFilter bool
}

// NewHunkAddModel creates a HunkAddModel by listing files with unstaged changes.
// filterPaths optionally restricts the file list to specific paths.
func NewHunkAddModel(
	ctx context.Context,
	runner git.Runner,
	cfg config.Config,
	renderer diff.Renderer,
	filterPaths ...[]string,
) *HunkAddModel {
	var paths []string
	if len(filterPaths) > 0 {
		paths = expandGlobs(filterPaths[0])
	}
	// Only modified tracked files have patchable hunks.
	diffArgs := []string{"diff"}
	if len(paths) > 0 {
		diffArgs = append(diffArgs, "--")
		diffArgs = append(diffArgs, paths...)
	}
	rawDiff, _ := runner.Run(ctx, diffArgs...)
	branchName, _ := git.BranchName(ctx, runner)

	files := git.ParseFileDiffs(rawDiff)

	entries := make([]components.FileEntry, len(files))
	for i, f := range files {
		entries[i] = components.FileEntry{Path: f.DisplayPath(), Status: f.Status}
	}

	noMatch := len(filterPaths) > 0 && len(filterPaths[0]) > 0 && len(files) == 0

	m := &HunkAddModel{
		ctx:           ctx,
		runner:        runner,
		renderer:      renderer,
		cfg:           cfg,
		files:         files,
		fileList:      components.NewFileList(entries, false),
		filterPaths:   paths,
		noMatchFilter: noMatch,
		diffView:      components.NewDiffView(80, 20),
		statusBar:     components.NewStatusBar(120),
		help: components.NewHelpOverlay([]components.KeyGroup{
			{
				Name: "Navigation",
				Bindings: []components.KeyBinding{
					{Key: "j/k", Desc: "move between files"},
					{Key: "o", Desc: "expand/collapse"},
					{Key: "Tab", Desc: "switch panel"},
					{Key: "D", Desc: "toggle diff"},
					{Key: "n", Desc: "skip hunk"},
					{Key: "?", Desc: "toggle help"},
				},
			},
			{
				Name: "Actions",
				Bindings: []components.KeyBinding{
					{Key: "y", Desc: "stage hunk"},
					{Key: "a", Desc: "stage all remaining hunks"},
					{Key: "s", Desc: "split hunk"},
					{Key: "w", Desc: "toggle soft-wrap (diff panel)"},
					{Key: "F", Desc: "maximize diff panel"},
					{Key: "q/Esc", Desc: "quit"},
				},
			},
		}),
		branch:     branchName,
		decided:    make(map[int]bool),
		panelRatio: cfg.PanelRatio,
	}

	m.showDiff = cfg.ShowDiffPanel
	m.diffView.SetSoftWrap(cfg.SoftWrap)

	m.statusBar.SetHints(m.hintsWithProgress())
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("hunk-add")

	if len(files) > 0 {
		m.loadFileHunks(0)
	}

	return m
}

// Update handles messages and returns commands.
func (m *HunkAddModel) Update(msg tea.Msg) tea.Cmd {
	sbCmd := m.statusBar.Update(msg)

	switch msg := msg.(type) {
	case git.EditDiffMsg:
		if msg.Err != nil {
			_ = m.statusBar.SetMessage(fmt.Sprintf("Edit failed: %v", msg.Err), components.Error)
			return sbCmd
		}
		if err := git.ApplyEditedDiff(m.ctx, m.runner, msg.OriginalDiff, msg.EditedPath); err != nil {
			_ = m.statusBar.SetMessage(fmt.Sprintf("Apply failed: %v", err), components.Error)
			return sbCmd
		}
		m.renderCurrentHunk()
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

		if msg.Code == tea.KeyTab {
			if m.showDiff && !m.diffMaximized {
				m.focusRight = !m.focusRight
				m.statusBar.SetHints(m.hintsWithProgress())
			}
			return sbCmd
		}

		if msg.String() == "D" {
			m.showDiff = !m.showDiff
			if m.showDiff && len(m.files) > 0 {
				m.renderCurrentHunk()
			}
			return sbCmd
		}

		if msg.String() == "F" {
			if m.showDiff {
				m.diffMaximized = !m.diffMaximized
				m.statusBar.SetHints(m.hintsWithProgress())
			}
			return sbCmd
		}

		if msg.String() == "w" && m.focusRight {
			m.diffView.SetSoftWrap(!m.diffView.SoftWrap())
			return sbCmd
		}

		if msg.String() == "e" {
			if len(m.hunks) == 0 || m.allDecided || m.hunkIdx >= len(m.hunks) {
				return sbCmd
			}
			rawDiff := m.hunks[m.hunkIdx].Body
			if rawDiff == "" {
				return sbCmd
			}
			return git.EditDiff(m.ctx, m.runner, rawDiff)
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

		case 'y':
			return m.stageCurrentHunk()

		case 'n':
			return m.skipCurrentHunk()

		case 'a':
			return m.stageAllRemaining()

		case 's':
			m.splitCurrentHunk()
			return sbCmd
		}

		if m.focusRight {
			dvCmd := m.diffView.Update(msg)
			return tea.Batch(sbCmd, dvCmd)
		}

		// Forward navigation (j/k) to the file tree
		treeCmd := m.fileList.Update(msg)
		m.syncFileSelection()
		return tea.Batch(sbCmd, treeCmd)
	}

	return sbCmd
}

// View renders the two-panel layout with the help overlay composited on top
// when visible.
func (m *HunkAddModel) View() string {
	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	var background string
	switch {
	case m.noMatchFilter:
		background = "No matching changes for the given paths."
	case len(m.files) == 0:
		background = "No unstaged changes to stage."
	default:
		contentHeight := m.height - 1
		m.statusBar.SetWidth(m.width)
		m.statusBar.SetHints(m.hintsWithProgress())

		if !m.showDiff {
			panelW := m.width - 1
			m.fileList.SetWidth(panelW)
			m.fileList.SetHeight(contentHeight)
			leftPanel := tui.StyleFocusBorder.Width(panelW).Height(contentHeight).MaxHeight(contentHeight).Render(m.fileList.View())
			background = leftPanel + "\n" + m.statusBar.View()
		} else if m.diffMaximized {
			rightW := m.width - 1
			m.diffView.SetWidth(rightW)
			m.diffView.SetHeight(contentHeight)
			rightPanel := tui.StyleFocusBorder.Width(rightW).Height(contentHeight).MaxHeight(contentHeight).Render(m.diffView.View())
			background = rightPanel + "\n" + m.statusBar.View()
		} else {
			leftW, rightW := tui.ColumnsFromConfig(m.width, m.panelRatio)

			leftW--
			rightW--

			m.fileList.SetWidth(leftW)
			m.fileList.SetHeight(contentHeight)
			m.diffView.SetWidth(rightW)
			m.diffView.SetHeight(contentHeight)

			leftBorder, rightBorder := tui.StyleFocusBorder, tui.StyleDimBorder
			if m.focusRight {
				leftBorder, rightBorder = tui.StyleDimBorder, tui.StyleFocusBorder
			}

			leftPanel := leftBorder.Width(leftW).Height(contentHeight).MaxHeight(contentHeight).Render(m.fileList.View())
			rightPanel := rightBorder.Width(rightW).Height(contentHeight).MaxHeight(contentHeight).Render(m.diffView.View())

			panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
			background = panels + "\n" + m.statusBar.View()
		}
	}

	return m.help.View(background, m.width, m.height)
}

// hintsWithProgress returns the status bar hint string including hunk progress.
// When the right panel has focus, it shows scroll hints instead of action hints.
func (m *HunkAddModel) hintsWithProgress() string {
	if m.diffMaximized {
		if len(m.hunks) == 0 {
			return "F: restore  ?: help  q: quit"
		}
		progress := fmt.Sprintf("Hunk %d/%d", m.hunkIdx+1, len(m.hunks))
		return fmt.Sprintf("%s  F: restore  ?: help  q: quit", progress)
	}
	if m.focusRight {
		if len(m.hunks) == 0 {
			return "h/l: scroll  Tab: panel  ?: help  q: quit"
		}
		progress := fmt.Sprintf("Hunk %d/%d", m.hunkIdx+1, len(m.hunks))
		return fmt.Sprintf("%s  h/l: scroll  Tab: panel  ?: help  q: quit", progress)
	}
	if len(m.hunks) == 0 {
		return "y: stage  n: skip  Tab: panel  ?: help  q: quit"
	}
	progress := fmt.Sprintf("Hunk %d/%d", m.hunkIdx+1, len(m.hunks))
	return fmt.Sprintf("%s  y: stage  n: skip  Tab: panel  ?: help  q: quit", progress)
}

// loadFileHunks loads the hunks for the file at fileIdx and renders the first hunk.
func (m *HunkAddModel) loadFileHunks(idx int) {
	if idx >= len(m.files) {
		return
	}
	m.fileIdx = idx
	m.hunks = git.ParseHunks(m.files[idx].RawDiff)
	m.hunkIdx = 0
	m.decided = make(map[int]bool)
	m.allDecided = false
	m.renderCurrentHunk()
}

// renderCurrentHunk updates the right-panel diff view with the current hunk body.
func (m *HunkAddModel) renderCurrentHunk() {
	if len(m.hunks) == 0 {
		m.diffView.SetContent("(no hunks)")
		return
	}
	if m.allDecided {
		m.diffView.SetContent("All hunks decided for this file.")
		return
	}
	if m.hunkIdx >= len(m.hunks) {
		m.diffView.SetContent("(no more hunks)")
		return
	}

	raw := m.hunks[m.hunkIdx].Body
	rendered, err := m.renderer.Render(raw)
	if err != nil {
		rendered = raw
	}
	m.diffView.SetContent(rendered)
}

// stageCurrentHunk stages the current hunk via git apply --cached.
func (m *HunkAddModel) stageCurrentHunk() tea.Cmd {
	if len(m.hunks) == 0 || m.allDecided {
		return nil
	}
	if m.fileIdx >= len(m.files) {
		return nil
	}

	hunk := m.hunks[m.hunkIdx]
	header := m.patchHeader(m.files[m.fileIdx])

	err := git.StageHunk(m.ctx, m.runner, header, hunk.Body)
	if err != nil {
		errCmd := m.statusBar.SetMessage(fmt.Sprintf("Stage failed: %v", err), components.Error)
		_ = errCmd
		return nil
	}

	m.decided[m.hunkIdx] = true
	return m.advanceHunk()
}

// skipCurrentHunk marks the current hunk as skipped and advances.
func (m *HunkAddModel) skipCurrentHunk() tea.Cmd {
	if len(m.hunks) == 0 || m.allDecided {
		return nil
	}
	m.decided[m.hunkIdx] = false
	return m.advanceHunk()
}

// stageAllRemaining stages all undecided hunks from the current hunk onwards.
func (m *HunkAddModel) stageAllRemaining() tea.Cmd {
	if len(m.hunks) == 0 || m.allDecided || m.fileIdx >= len(m.files) {
		return nil
	}

	header := m.patchHeader(m.files[m.fileIdx])
	var lastErr error

	for i := m.hunkIdx; i < len(m.hunks); i++ {
		if _, already := m.decided[i]; already {
			continue
		}
		err := git.StageHunk(m.ctx, m.runner, header, m.hunks[i].Body)
		if err != nil {
			lastErr = err
			continue
		}
		m.decided[i] = true
	}

	if lastErr != nil {
		errCmd := m.statusBar.SetMessage(fmt.Sprintf("Stage failed: %v", lastErr), components.Error)
		_ = errCmd
	}

	m.allDecided = true
	m.renderCurrentHunk()
	return m.tryNextFile()
}

// splitCurrentHunk attempts to split the current hunk into sub-hunks.
// If the hunk cannot be split further (only one context-block), it shows a message.
func (m *HunkAddModel) splitCurrentHunk() {
	if len(m.hunks) == 0 || m.allDecided || m.hunkIdx >= len(m.hunks) {
		return
	}

	sub := splitHunk(m.hunks[m.hunkIdx])
	if len(sub) <= 1 {
		msgCmd := m.statusBar.SetMessage("Cannot split hunk further", components.Info)
		_ = msgCmd
		return
	}

	// Replace current hunk with sub-hunks
	before := m.hunks[:m.hunkIdx]
	after := m.hunks[m.hunkIdx+1:]
	m.hunks = make([]git.Hunk, 0, len(before)+len(sub)+len(after))
	m.hunks = append(m.hunks, before...)
	m.hunks = append(m.hunks, sub...)
	m.hunks = append(m.hunks, after...)

	m.renderCurrentHunk()
}

// splitHunk tries to break a hunk into smaller pieces at context-line boundaries.
// Returns the original slice (len==1) if it cannot be split further.
func splitHunk(h git.Hunk) []git.Hunk {
	lines := strings.Split(h.Body, "\n")
	if len(lines) <= 2 {
		return []git.Hunk{h}
	}

	// Find the change lines (+/-); if there are multiple groups separated by
	// context lines, we can split there.
	type segment struct {
		start int
		end   int
	}

	var segments []segment
	inChange := false
	segStart := 0

	for i, line := range lines {
		if i == 0 {
			// Skip @@ header line
			continue
		}
		isChange := strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-")
		if isChange && !inChange {
			inChange = true
			segStart = i
		} else if !isChange && inChange {
			inChange = false
			segments = append(segments, segment{segStart, i})
		}
	}
	if inChange {
		segments = append(segments, segment{segStart, len(lines)})
	}

	if len(segments) <= 1 {
		return []git.Hunk{h}
	}

	// Build sub-hunks: each keeps surrounding context (up to 3 lines) around its changes.
	var result []git.Hunk
	header := lines[0]

	for _, seg := range segments {
		ctxBefore := max(1, seg.start-3)
		ctxAfter := min(len(lines), seg.end+3)

		subLines := []string{header}
		subLines = append(subLines, lines[ctxBefore:ctxAfter]...)

		body := strings.Join(subLines, "\n")
		result = append(result, git.Hunk{
			Header: header,
			Body:   body,
		})
	}

	return result
}

// advanceHunk moves to the next undecided hunk, or marks all decided.
// Returns a tea.Cmd if we should move to the next file.
func (m *HunkAddModel) advanceHunk() tea.Cmd {
	for i := m.hunkIdx + 1; i < len(m.hunks); i++ {
		if _, already := m.decided[i]; !already {
			m.hunkIdx = i
			m.renderCurrentHunk()
			return nil
		}
	}

	// All hunks decided
	m.allDecided = true
	m.renderCurrentHunk()
	return m.tryNextFile()
}

// tryNextFile moves to the next file with hunks, or pops the model if done.
// mutated indicates whether any hunk was actually staged during this session.
func (m *HunkAddModel) tryNextFile() tea.Cmd {
	for next := m.fileIdx + 1; next < len(m.files); next++ {
		hunks := git.ParseHunks(m.files[next].RawDiff)
		if len(hunks) > 0 {
			m.loadFileHunks(next)
			return nil
		}
	}
	// No more files - pop; MutatedGit reflects whether anything was staged
	mutated := m.anyStagedDecision()
	return func() tea.Msg {
		return app.PopModelMsg{MutatedGit: mutated}
	}
}

// anyStagedDecision reports whether any hunk across all files was staged (decided=true).
func (m *HunkAddModel) anyStagedDecision() bool {
	for _, staged := range m.decided {
		if staged {
			return true
		}
	}
	return false
}

// syncFileSelection detects if the tree cursor moved to a different file and loads its hunks.
func (m *HunkAddModel) syncFileSelection() {
	path := m.fileList.SelectedPath()
	if path == "" {
		return
	}
	for i, f := range m.files {
		if f.DisplayPath() == path && i != m.fileIdx {
			m.loadFileHunks(i)
			return
		}
	}
}

// patchHeader builds a minimal unified diff header for git apply.
func (m *HunkAddModel) patchHeader(fd git.FileDiff) string {
	oldPath := "a/" + fd.OldPath
	newPath := "b/" + fd.NewPath
	return fmt.Sprintf("diff --git %s %s\n--- %s\n+++ %s", oldPath, newPath, oldPath, newPath)
}

// resize recalculates component dimensions after a terminal resize.
func (m *HunkAddModel) resize() {
	contentHeight := m.height - 1
	m.statusBar.SetWidth(m.width)

	if !m.showDiff {
		panelW := m.width - 1
		m.fileList.SetWidth(panelW)
		m.fileList.SetHeight(contentHeight)
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

	m.fileList.SetWidth(leftW)
	m.fileList.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)
}
