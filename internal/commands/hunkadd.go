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
	allHunks      [][]git.Hunk // per-file parsed hunk slices
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
}

// NewHunkAddModel creates a HunkAddModel by listing files with unstaged changes.
func NewHunkAddModel(
	ctx context.Context,
	runner git.Runner,
	cfg config.Config,
	renderer diff.Renderer,
) *HunkAddModel {
	rawDiff, _ := runner.Run(ctx, "diff")
	branchName, _ := git.BranchName(ctx, runner)

	files := git.ParseFileDiffs(rawDiff)

	allHunks := make([][]git.Hunk, len(files))
	for i, f := range files {
		allHunks[i] = git.ParseHunks(f.RawDiff)
	}

	m := &HunkAddModel{
		ctx:       ctx,
		runner:    runner,
		renderer:  renderer,
		cfg:       cfg,
		files:     files,
		allHunks:  allHunks,
		hunkList:  components.NewHunkList(files, allHunks),
		diffView:  components.NewDiffView(80, 20),
		statusBar: components.NewStatusBar(120),
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
					{Key: "s", Desc: "split hunk"},
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

	m.statusBar.SetHints(m.hintsWithProgress())
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

		case tea.KeyEnter:
			return m.applyAllStaged()

		case tea.KeySpace:
			if !m.focusRight {
				m.hunkList.Update(msg)
				m.renderCurrentHunk()
				m.statusBar.SetHints(m.hintsWithProgress())
			}
			return sbCmd

		case 's':
			m.splitCurrentHunk()
			return sbCmd
		}

		if m.focusRight {
			dvCmd := m.diffView.Update(msg)
			return tea.Batch(sbCmd, dvCmd)
		}

		// Forward navigation (j/k) to the hunk list
		m.hunkList.Update(msg)
		m.renderCurrentHunk()
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
	if len(m.files) == 0 {
		background = "No unstaged changes to stage."
	} else {
		contentHeight := m.height - 1
		m.statusBar.SetWidth(m.width)
		m.statusBar.SetHints(m.hintsWithProgress())

		if !m.showDiff {
			panelW := m.width - 1
			m.hunkList.SetWidth(panelW)
			m.hunkList.SetHeight(contentHeight)
			leftPanel := tui.StyleFocusBorder.Width(panelW).Height(contentHeight).MaxHeight(contentHeight).Render(m.hunkList.View())
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

			m.hunkList.SetWidth(leftW)
			m.hunkList.SetHeight(contentHeight)
			m.diffView.SetWidth(rightW)
			m.diffView.SetHeight(contentHeight)

			leftBorder, rightBorder := tui.StyleFocusBorder, tui.StyleDimBorder
			if m.focusRight {
				leftBorder, rightBorder = tui.StyleDimBorder, tui.StyleFocusBorder
			}

			leftPanel := leftBorder.Width(leftW).Height(contentHeight).MaxHeight(contentHeight).Render(m.hunkList.View())
			rightPanel := rightBorder.Width(rightW).Height(contentHeight).MaxHeight(contentHeight).Render(m.diffView.View())

			panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
			background = panels + "\n" + m.statusBar.View()
		}
	}

	return m.help.View(background, m.width, m.height)
}

// hintsWithProgress returns the status bar hint string.
func (m *HunkAddModel) hintsWithProgress() string {
	if m.diffMaximized {
		return "F: restore  ?: help  q: quit"
	}
	if m.focusRight {
		return "h/l: scroll  Tab: panel  ?: help  q: quit"
	}
	return "Space: toggle  Enter: apply  s: split  ?: help  q: quit"
}

// applyAllStaged applies all staged hunks via git apply --cached and pops.
func (m *HunkAddModel) applyAllStaged() tea.Cmd {
	staged := m.hunkList.StagedHunks()
	if len(staged) == 0 {
		errCmd := m.statusBar.SetMessage("No hunks staged", components.Info)
		_ = errCmd
		return nil
	}

	anySucceeded := false
	var lastErr error

	for _, sh := range staged {
		if sh.FileIdx >= len(m.files) {
			continue
		}
		header := m.patchHeader(m.files[sh.FileIdx])
		err := git.StageHunk(m.ctx, m.runner, header, sh.Hunk.Body)
		if err != nil {
			lastErr = err
		} else {
			anySucceeded = true
		}
	}

	if lastErr != nil {
		errCmd := m.statusBar.SetMessage(fmt.Sprintf("Stage failed: %v", lastErr), components.Error)
		_ = errCmd
	}

	mutated := anySucceeded
	return func() tea.Msg {
		return app.PopModelMsg{MutatedGit: mutated}
	}
}

// renderCurrentHunk updates the right-panel diff view with the hunk under the cursor.
func (m *HunkAddModel) renderCurrentHunk() {
	fi := m.hunkList.CurrentFileIdx()
	hi := m.hunkList.CurrentHunkIdx()

	if fi >= len(m.allHunks) || len(m.allHunks[fi]) == 0 {
		m.diffView.SetContent("(no hunks)")
		return
	}
	if hi >= len(m.allHunks[fi]) {
		m.diffView.SetContent("(no more hunks)")
		return
	}

	raw := m.allHunks[fi][hi].Body
	rendered, err := m.renderer.Render(raw)
	if err != nil {
		rendered = raw
	}
	m.diffView.SetContent(rendered)
}

// splitCurrentHunk splits the hunk under the cursor and rebuilds the hunk list.
func (m *HunkAddModel) splitCurrentHunk() {
	fi := m.hunkList.CurrentFileIdx()
	hi := m.hunkList.CurrentHunkIdx()

	if fi >= len(m.allHunks) || len(m.allHunks[fi]) == 0 {
		return
	}
	if hi >= len(m.allHunks[fi]) {
		return
	}

	sub := splitHunk(m.allHunks[fi][hi])
	if len(sub) <= 1 {
		msgCmd := m.statusBar.SetMessage("Cannot split hunk further", components.Info)
		_ = msgCmd
		return
	}

	before := m.allHunks[fi][:hi]
	after := m.allHunks[fi][hi+1:]
	newHunks := make([]git.Hunk, 0, len(before)+len(sub)+len(after))
	newHunks = append(newHunks, before...)
	newHunks = append(newHunks, sub...)
	newHunks = append(newHunks, after...)
	m.allHunks[fi] = newHunks

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
