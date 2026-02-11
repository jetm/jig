package commands

import (
	"context"
	"fmt"
	"os/exec"
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
// It uses a HunkList component for checkbox-based hunk selection and batch staging.
type HunkAddModel struct {
	ctx      context.Context
	runner   git.Runner
	renderer diff.Renderer
	cfg      config.Config

	files         []git.FileDiff
	hunkList      components.HunkList
	diffView      components.DiffView
	statusBar     components.StatusBar
	help          components.HelpOverlay
	branch        string
	width         int
	height        int
	panelRatio    int
	focusRight    bool
	showDiff      bool
	diffMaximized bool
	filterPaths   []string
	noMatchFilter bool
	// prevFileIdx/prevHunkIdx track cursor position to detect changes and update diff.
	prevFileIdx int
	prevHunkIdx int
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
		paths = ExpandGlobs(filterPaths[0])
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

	// Parse hunks per file for HunkList.
	hunks := make([][]git.Hunk, len(files))
	for i, f := range files {
		hunks[i] = git.ParseHunks(f.RawDiff)
	}

	noMatch := len(filterPaths) > 0 && len(filterPaths[0]) > 0 && len(files) == 0

	m := &HunkAddModel{
		ctx:           ctx,
		runner:        runner,
		renderer:      renderer,
		cfg:           cfg,
		files:         files,
		hunkList:      components.NewHunkList(files, hunks),
		filterPaths:   paths,
		noMatchFilter: noMatch,
		diffView:      components.NewDiffView(80, 20),
		statusBar:     components.NewStatusBar(120),
		help: components.NewHelpOverlay([]components.KeyGroup{
			{
				Name: "Navigation",
				Bindings: []components.KeyBinding{
					{Key: "j/k", Desc: "move between hunks"},
					{Key: "Tab", Desc: "switch panel"},
					{Key: "D", Desc: "toggle diff"},
					{Key: "?", Desc: "toggle help"},
				},
			},
			{
				Name: "Actions",
				Bindings: []components.KeyBinding{
					{Key: "Space", Desc: "toggle hunk staged"},
					{Key: "Enter", Desc: "apply staged hunks"},
					{Key: "c", Desc: "stage and commit"},
					{Key: "C", Desc: "stage and commit (title only)"},
					{Key: "s", Desc: "split hunk"},
					{Key: "e", Desc: "edit hunk in editor"},
					{Key: "w", Desc: "toggle soft-wrap (diff panel)"},
					{Key: "F", Desc: "maximize diff panel"},
					{Key: "q/Esc", Desc: "quit"},
				},
			},
		}),
		branch:     branchName,
		panelRatio: cfg.PanelRatio,
	}

	m.showDiff = cfg.ShowDiffPanel
	m.diffView.SetSoftWrap(cfg.SoftWrap)

	m.statusBar.SetHints(m.hintsForContext())
	m.statusBar.SetBranch(branchName)
	m.statusBar.SetMode("hunk-add")

	if len(files) > 0 {
		m.renderCurrentHunk()
	}

	return m
}

// Update handles messages and returns commands.
func (m *HunkAddModel) Update(msg tea.Msg) tea.Cmd {
	sbCmd := m.statusBar.Update(msg)

	switch msg := msg.(type) {
	case CommitDoneMsg:
		if msg.Err != nil {
			_ = m.statusBar.SetMessage("Commit aborted", components.Info)
			m.refreshHunks()
			return sbCmd
		}
		return func() tea.Msg {
			return app.PopModelMsg{MutatedGit: true}
		}

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
				m.statusBar.SetHints(m.hintsForContext())
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
				m.statusBar.SetHints(m.hintsForContext())
			}
			return sbCmd
		}

		if msg.String() == "w" && m.focusRight {
			m.diffView.SetSoftWrap(!m.diffView.SoftWrap())
			return sbCmd
		}

		if msg.String() == "e" {
			hunk, ok := m.hunkList.CurrentHunk()
			if !ok {
				return sbCmd
			}
			fi := m.hunkList.CurrentFileIdx()
			rawDiff := m.patchHeader(m.files[fi]) + "\n" + hunk.Body
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
			return m.applyStaged()

		case 's':
			m.splitCurrentHunk()
			return sbCmd
		}

		if m.focusRight {
			dvCmd := m.diffView.Update(msg)
			return tea.Batch(sbCmd, dvCmd)
		}

		// Forward navigation (j/k/Space) to the hunk list.
		m.hunkList.Update(msg)
		m.syncDiffPreview()
		return sbCmd
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
		m.statusBar.SetHints(m.hintsForContext())

		if !m.showDiff {
			panelW := m.width - 1
			leftContent := m.renderLeftPanel(panelW, contentHeight)
			leftPanel := tui.StyleFocusBorder.Width(panelW).Height(contentHeight).MaxHeight(contentHeight).Render(leftContent)
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

			m.diffView.SetWidth(rightW)
			m.diffView.SetHeight(contentHeight)

			leftBorder, rightBorder := tui.StyleFocusBorder, tui.StyleDimBorder
			if m.focusRight {
				leftBorder, rightBorder = tui.StyleDimBorder, tui.StyleFocusBorder
			}

			leftContent := m.renderLeftPanel(leftW, contentHeight)
			leftPanel := leftBorder.Width(leftW).Height(contentHeight).MaxHeight(contentHeight).Render(leftContent)
			rightPanel := rightBorder.Width(rightW).Height(contentHeight).MaxHeight(contentHeight).Render(m.diffView.View())

			panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
			background = panels + "\n" + m.statusBar.View()
		}
	}

	return m.help.View(background, m.width, m.height)
}

// renderLeftPanel renders the split left panel: top 60% hunk list, bottom 40% file summary.
func (m *HunkAddModel) renderLeftPanel(width, height int) string {
	separatorStyle := lipgloss.NewStyle().Foreground(tui.ColorFgSubtle)
	separator := separatorStyle.Width(width).Render(strings.Repeat("─", width))
	sepHeight := 1

	topHeight := height * 60 / 100
	bottomHeight := height - topHeight - sepHeight
	if bottomHeight < 1 {
		bottomHeight = 1
		topHeight = height - sepHeight - bottomHeight
	}

	m.hunkList.SetWidth(width)
	m.hunkList.SetHeight(topHeight)

	top := m.hunkList.View()
	bottom := m.hunkList.FileSummary()

	// Pad bottom to fill available height.
	bottomLines := strings.Count(bottom, "\n") + 1
	if bottom == "" {
		bottomLines = 0
	}
	for bottomLines < bottomHeight {
		bottom += "\n"
		bottomLines++
	}

	return top + "\n" + separator + "\n" + bottom
}

// hintsForContext returns status bar hints based on current context.
func (m *HunkAddModel) hintsForContext() string {
	if m.diffMaximized {
		return "F: restore  ?: help  q: quit"
	}
	if m.focusRight {
		return "h/l: scroll  Tab: panel  ?: help  q: quit"
	}
	return "Space: toggle  Enter: apply  c: commit  Tab: panel  ?: help  q: quit"
}

// renderCurrentHunk updates the right-panel diff view with the hunk under the cursor.
func (m *HunkAddModel) renderCurrentHunk() {
	hunk, ok := m.hunkList.CurrentHunk()
	if !ok {
		m.diffView.SetContent("(no hunks)")
		return
	}

	raw := hunk.Body
	rendered, err := m.renderer.Render(raw)
	if err != nil {
		rendered = raw
	}
	m.diffView.SetContent(rendered)
	m.prevFileIdx = m.hunkList.CurrentFileIdx()
	m.prevHunkIdx = m.hunkList.CurrentHunkIdx()
}

// syncDiffPreview updates the diff view if the cursor moved to a different hunk.
func (m *HunkAddModel) syncDiffPreview() {
	fi := m.hunkList.CurrentFileIdx()
	hi := m.hunkList.CurrentHunkIdx()
	if fi != m.prevFileIdx || hi != m.prevHunkIdx {
		m.renderCurrentHunk()
	}
}

// applyStaged stages all checked hunks via git apply --cached.
func (m *HunkAddModel) applyStaged() tea.Cmd {
	staged := m.hunkList.StagedHunks()
	if len(staged) == 0 {
		return nil
	}

	var lastErr error
	applied := 0
	for _, sh := range staged {
		if sh.FileIdx >= len(m.files) {
			continue
		}
		header := m.patchHeader(m.files[sh.FileIdx])
		err := git.StageHunk(m.ctx, m.runner, header, sh.Hunk.Body)
		if err != nil {
			lastErr = err
			continue
		}
		applied++
	}

	if lastErr != nil {
		_ = m.statusBar.SetMessage(fmt.Sprintf("Stage failed: %v", lastErr), components.Error)
		if applied == 0 {
			return nil
		}
	}

	return func() tea.Msg {
		return app.PopModelMsg{MutatedGit: true}
	}
}

// splitCurrentHunk attempts to split the hunk under the cursor into sub-hunks.
func (m *HunkAddModel) splitCurrentHunk() {
	hunk, ok := m.hunkList.CurrentHunk()
	if !ok {
		return
	}

	sub := splitHunk(hunk)
	if len(sub) <= 1 {
		_ = m.statusBar.SetMessage("Cannot split hunk further", components.Info)
		return
	}

	fi := m.hunkList.CurrentFileIdx()
	hi := m.hunkList.CurrentHunkIdx()
	fileHunks := m.hunkList.FileHunks(fi)

	// Replace the single hunk with sub-hunks in the file's hunk slice.
	before := fileHunks[:hi]
	after := fileHunks[hi+1:]
	newHunks := make([]git.Hunk, 0, len(before)+len(sub)+len(after))
	newHunks = append(newHunks, before...)
	newHunks = append(newHunks, sub...)
	newHunks = append(newHunks, after...)

	m.hunkList.ReplaceHunks(fi, newHunks)
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

// patchHeader builds a minimal unified diff header for git apply.
func (m *HunkAddModel) patchHeader(fd git.FileDiff) string {
	oldPath := "a/" + fd.OldPath
	newPath := "b/" + fd.NewPath
	return fmt.Sprintf("diff --git %s %s\n--- %s\n+++ %s", oldPath, newPath, oldPath, newPath)
}

// execCommit stages checked hunks and launches devtool commit as a subprocess.
// If titleOnly is true, passes -t for a title-only commit message.
// Returns nil if staging fails (error shown in status bar).
func (m *HunkAddModel) execCommit(titleOnly bool) tea.Cmd {
	staged := m.hunkList.StagedHunks()
	if len(staged) == 0 {
		return nil
	}

	var lastErr error
	applied := 0
	for _, sh := range staged {
		if sh.FileIdx >= len(m.files) {
			continue
		}
		header := m.patchHeader(m.files[sh.FileIdx])
		if err := git.StageHunk(m.ctx, m.runner, header, sh.Hunk.Body); err != nil {
			lastErr = err
			continue
		}
		applied++
	}

	if applied == 0 {
		if lastErr != nil {
			_ = m.statusBar.SetMessage(fmt.Sprintf("Stage failed: %v", lastErr), components.Error)
		}
		return nil
	}

	args := []string{"commit"}
	if titleOnly {
		args = append(args, "-t")
	}
	cmd := exec.Command("devtool", args...) //nolint:gosec // devtool is a trusted user tool
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return CommitDoneMsg{Err: err}
	})
}

// refreshHunks re-queries git state and rebuilds the hunk list.
// Called after a commit completes to reflect the new index state.
func (m *HunkAddModel) refreshHunks() {
	diffArgs := []string{"diff"}
	if len(m.filterPaths) > 0 {
		diffArgs = append(diffArgs, "--")
		diffArgs = append(diffArgs, m.filterPaths...)
	}
	rawDiff, _ := m.runner.Run(m.ctx, diffArgs...)
	files := git.ParseFileDiffs(rawDiff)

	hunks := make([][]git.Hunk, len(files))
	for i, f := range files {
		hunks[i] = git.ParseHunks(f.RawDiff)
	}

	m.files = files
	m.hunkList = components.NewHunkList(files, hunks)
	m.prevFileIdx = 0
	m.prevHunkIdx = 0

	if len(files) > 0 {
		m.renderCurrentHunk()
	}
	m.resize()
}

// resize recalculates component dimensions after a terminal resize.
func (m *HunkAddModel) resize() {
	contentHeight := m.height - 1
	m.statusBar.SetWidth(m.width)

	if !m.showDiff {
		panelW := m.width - 1
		m.hunkList.SetWidth(panelW)
		m.hunkList.SetHeight(contentHeight)
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

	m.hunkList.SetWidth(leftW)
	m.hunkList.SetHeight(contentHeight)
	m.diffView.SetWidth(rightW)
	m.diffView.SetHeight(contentHeight)
}
