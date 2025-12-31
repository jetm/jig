package commands

import (
	"context"
	"fmt"
	"strings"

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

// hunkAddFileItem wraps a git.FileDiff for use with ItemList in hunk-add mode.
type hunkAddFileItem struct {
	fd git.FileDiff
}

func (h hunkAddFileItem) Title() string       { return h.fd.DisplayPath() }
func (h hunkAddFileItem) Description() string { return statusLabel(h.fd.Status) }
func (h hunkAddFileItem) FilterValue() string { return h.fd.DisplayPath() }

// HunkAddModel is the command model for the hunk-add TUI (interactive hunk-level staging).
// It follows the child component pattern: Update returns tea.Cmd, View returns string.
type HunkAddModel struct {
	ctx      context.Context
	runner   git.Runner
	renderer diff.Renderer

	files      []git.FileDiff
	fileList   components.ItemList
	diffView   components.DiffView
	statusBar  components.StatusBar
	help       components.HelpOverlay
	branch     string
	width      int
	height     int
	fileIdx    int          // index into files for the current file
	hunks      []git.Hunk   // hunks of the current file
	hunkIdx    int          // index into hunks (current hunk)
	decided    map[int]bool // hunkIdx -> staged (true) or skipped (false); nil means undecided
	allDecided bool         // true once all hunks in current file are decided
}

// NewHunkAddModel creates a HunkAddModel by listing files with unstaged changes.
func NewHunkAddModel(
	ctx context.Context,
	runner git.Runner,
	_ config.Config,
	renderer diff.Renderer,
) *HunkAddModel {
	// Only modified tracked files have patchable hunks.
	rawDiff, _ := runner.Run(ctx, "diff")
	branchName, _ := git.BranchName(ctx, runner)

	files := git.ParseFileDiffs(rawDiff)

	items := make([]list.Item, len(files))
	for i, f := range files {
		items[i] = hunkAddFileItem{fd: f}
	}

	m := &HunkAddModel{
		ctx:       ctx,
		runner:    runner,
		renderer:  renderer,
		files:     files,
		fileList:  components.NewItemList(items, 40, 20),
		diffView:  components.NewDiffView(80, 20),
		statusBar: components.NewStatusBar(120),
		help: components.NewHelpOverlay([]components.KeyGroup{
			{
				Name: "Navigation",
				Bindings: []components.KeyBinding{
					{Key: "j/k", Desc: "move between files"},
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
					{Key: "q/Esc", Desc: "quit"},
				},
			},
		}),
		branch:  branchName,
		decided: make(map[int]bool),
	}

	m.statusBar.SetHints("y: stage  n: skip  a: all  s: split  ?: help  q: quit")
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

		// Forward navigation (j/k) to the file list
		listCmd := m.fileList.Update(msg)
		m.syncFileSelection()
		return tea.Batch(sbCmd, listCmd)
	}

	return sbCmd
}

// View renders the two-panel layout.
func (m *HunkAddModel) View() string {
	if m.help.IsVisible() {
		return m.help.View(m.width, m.height)
	}

	if tui.IsTerminalTooSmall(m.width, m.height) {
		return "Terminal too small. Please resize to at least 60x10."
	}

	if len(m.files) == 0 {
		return "No unstaged changes to stage."
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
	m.statusBar.SetHints(m.hintsWithProgress())
	return panels + "\n" + m.statusBar.View()
}

// hintsWithProgress returns the status bar hint string including hunk progress.
func (m *HunkAddModel) hintsWithProgress() string {
	if len(m.hunks) == 0 {
		return "y: stage  n: skip  a: all  s: split  ?: help  q: quit"
	}
	progress := fmt.Sprintf("Hunk %d/%d", m.hunkIdx+1, len(m.hunks))
	return fmt.Sprintf("%s  y: stage  n: skip  a: all  s: split  ?: help  q: quit", progress)
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
			// Sync the list selection
			m.selectListItem(next)
			return nil
		}
	}
	// No more files — pop; MutatedGit reflects whether anything was staged
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

// selectListItem moves the file list cursor to the given index.
func (m *HunkAddModel) selectListItem(idx int) {
	for range idx {
		m.fileList.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	}
}

// syncFileSelection detects if the list cursor moved to a different file and loads its hunks.
func (m *HunkAddModel) syncFileSelection() {
	sel := m.fileList.SelectedItem()
	if sel == nil {
		return
	}
	item, ok := sel.(hunkAddFileItem)
	if !ok {
		return
	}
	for i, f := range m.files {
		if f.DisplayPath() == item.fd.DisplayPath() && i != m.fileIdx {
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
	leftW, rightW := tui.Columns(m.width)
	contentHeight := m.height - 1

	m.fileList.SetWidth(leftW)
	m.fileList.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)
	m.statusBar.SetWidth(m.width)
}
