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

// HunkAddModel is the command model for the hunk-add TUI (interactive hunk-level staging).
// It uses a HunkList component for checkbox-based hunk selection and batch staging.
// Supports two phases: hunk list (default) and line-level editing (via Enter on a hunk).
type HunkAddModel struct {
	twoPanelModel

	ctx      context.Context
	runner   git.Runner
	renderer diff.Renderer

	files         []git.FileDiff
	hunkList      components.HunkList
	hunkView      *components.HunkView
	inLineEdit    bool
	lineEditFile  int // file index of the hunk being line-edited
	lineEditHunk  int // hunk index within file being line-edited
	branch        string
	contextLines  int
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
) (*HunkAddModel, error) {
	var paths []string
	if len(filterPaths) > 0 {
		paths = ExpandGlobs(filterPaths[0])
	}
	// Only modified tracked files have patchable hunks.
	diffArgs := []string{"diff", fmt.Sprintf("-U%d", cfg.DiffContext)}
	if len(paths) > 0 {
		diffArgs = append(diffArgs, "--")
		diffArgs = append(diffArgs, paths...)
	}
	rawDiff, err := runner.Run(ctx, diffArgs...)
	if err != nil {
		return nil, fmt.Errorf("running git diff: %w", err)
	}
	branchName, _ := git.BranchName(ctx, runner)

	files := git.ParseFileDiffs(rawDiff)

	// Parse hunks per file for HunkList.
	hunks := make([][]git.Hunk, len(files))
	for i, f := range files {
		hunks[i] = git.ParseHunks(f.RawDiff)
	}

	noMatch := len(filterPaths) > 0 && len(filterPaths[0]) > 0 && len(files) == 0

	hl := components.NewHunkList(files, hunks)

	m := &HunkAddModel{
		twoPanelModel: newTwoPanelModel(
			&hl,
			components.NewDiffView(80, 20, cfg.ShowLineNumbers && !isDeltaRenderer(renderer)),
			components.NewStatusBar(120),
			components.NewHelpOverlay([]components.KeyGroup{
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
					Name: "Hunk Actions",
					Bindings: []components.KeyBinding{
						{Key: "Space", Desc: "toggle hunk staged"},
						{Key: "Enter", Desc: "edit lines in hunk"},
						{Key: "w", Desc: "apply staged hunks"},
						{Key: "c", Desc: "stage and commit"},
						{Key: "C", Desc: "stage and commit (title only)"},
						{Key: "s", Desc: "split hunk"},
						{Key: "e", Desc: "edit hunk in editor"},
						{Key: "F", Desc: "maximize diff panel"},
						{Key: "/", Desc: "search in diff"},
						{Key: "n/N", Desc: "next/prev match"},
						{Key: "q/Esc", Desc: "quit"},
					},
				},
				{
					Name: "Line Edit",
					Bindings: []components.KeyBinding{
						{Key: "j/k", Desc: "move between lines"},
						{Key: "Space", Desc: "toggle line selection"},
						{Key: "u", Desc: "undo last toggle"},
						{Key: "Esc", Desc: "back to hunk list"},
					},
				},
			}),
			cfg,
		),
		ctx:           ctx,
		runner:        runner,
		renderer:      renderer,
		files:         files,
		hunkList:      hl,
		contextLines:  cfg.DiffContext,
		filterPaths:   paths,
		noMatchFilter: noMatch,
		branch:        branchName,
	}

	m.status.SetHints(m.hintsForContext())
	m.status.SetBranch(branchName)
	m.status.SetMode("hunk-add")

	if len(files) > 0 {
		m.renderCurrentHunk()
	}

	return m, nil
}

// Update handles messages and returns commands.
// Note: hunkadd has custom 'w' behavior (apply staged hunks when not focusRight),
// so it handles 'w' before delegating to twoPanelModel.handleKey.
func (m *HunkAddModel) Update(msg tea.Msg) tea.Cmd {
	sbCmd := m.status.Update(msg)

	switch msg := msg.(type) {
	case CommitDoneMsg:
		if msg.Err != nil {
			_ = m.status.SetMessage("Commit aborted", components.Info)
			m.refreshHunks()
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
		m.renderCurrentHunk()
		return sbCmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return sbCmd

	case tea.KeyPressMsg:
		// Line-edit phase: delegate to HunkView
		if m.inLineEdit {
			return m.updateLineEdit(msg, sbCmd)
		}

		if m.help.HandleKey(msg) {
			return sbCmd
		}

		// Handle 'w' before twoPanelModel - hunkadd uses 'w' for apply (not soft-wrap)
		if msg.String() == "w" {
			if !m.focusRight {
				return m.applyStaged()
			}
			// When focusRight, fall through to twoPanelModel for soft-wrap toggle
		}

		if cmd, handled := m.handleKey(msg); handled {
			if cmd != nil {
				return cmd
			}
			if msg.String() == "D" && m.showDiff && len(m.files) > 0 {
				m.renderCurrentHunk()
			}
			if m.leftUpdated {
				m.leftUpdated = false
				m.hunkList.Update(msg)
				m.syncDiffPreview()
			}
			m.status.SetHints(m.hintsForContext())
			return sbCmd
		}

		if msg.String() == "e" {
			hunk, ok := m.hunkList.CurrentHunk()
			if !ok {
				return sbCmd
			}
			fi := m.hunkList.CurrentFileIdx()
			rawDiff := patchHeader(m.files[fi]) + "\n" + hunk.Body()
			return editor.EditDiff(m.ctx, m.runner, rawDiff)
		}

		if msg.String() == "{" {
			if m.contextLines > 0 {
				m.contextLines--
				m.refreshHunks()
			}
			return sbCmd
		}

		if msg.String() == "}" {
			if m.contextLines < 20 {
				m.contextLines++
				m.refreshHunks()
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
		case 'q':
			return func() tea.Msg {
				return app.PopModelMsg{MutatedGit: false}
			}

		case tea.KeyEscape:
			return func() tea.Msg {
				return app.PopModelMsg{MutatedGit: false}
			}

		case tea.KeyEnter:
			m.enterLineEdit()
			return sbCmd

		case 's':
			m.splitCurrentHunk()
			return sbCmd
		}

		if m.focusRight {
			dvCmd := m.diff.Update(msg)
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
		m.status.SetHints(m.hintsForContext())
		// Use custom left panel rendering for line-edit mode
		m.left = &hunkAddLeftPanel{m: m}
		background = m.renderLayout()
	}

	return m.help.View(background, m.width, m.height)
}

// hunkAddLeftPanel wraps the HunkAddModel's left panel logic to satisfy tui.LeftPanel.
// It renders either the HunkView (in line-edit mode) or the HunkList.
type hunkAddLeftPanel struct {
	m *HunkAddModel
}

func (p *hunkAddLeftPanel) View() string {
	if p.m.inLineEdit && p.m.hunkView != nil {
		return p.m.hunkView.View()
	}
	return p.m.hunkList.View()
}

func (p *hunkAddLeftPanel) SetWidth(w int) {
	if p.m.inLineEdit && p.m.hunkView != nil {
		p.m.hunkView.SetWidth(w)
	}
	p.m.hunkList.SetWidth(w)
}

func (p *hunkAddLeftPanel) SetHeight(h int) {
	if p.m.inLineEdit && p.m.hunkView != nil {
		p.m.hunkView.SetHeight(h)
	}
	p.m.hunkList.SetHeight(h)
}

func (p *hunkAddLeftPanel) Update(msg tea.Msg) tea.Cmd {
	return p.m.hunkList.Update(msg)
}

// hintsForContext returns status bar hints based on current context.
func (m *HunkAddModel) hintsForContext() string {
	if m.inLineEdit {
		return "j/k: navigate  Space: toggle  u: undo  Esc: back  ?: help"
	}
	if m.diffMaximized {
		return "F: restore  ?: help  q: quit"
	}
	if m.focusRight {
		return "h/l: scroll  Tab: panel  ?: help  q: quit"
	}
	return "Space: toggle  Enter: lines  w: apply  c: commit  ?: help  q: quit"
}

// enterLineEdit transitions from hunk list to line-level editing for the current hunk.
func (m *HunkAddModel) enterLineEdit() {
	hunk, ok := m.hunkList.CurrentHunk()
	if !ok {
		return
	}
	hv := components.NewHunkView(hunk)
	m.hunkView = &hv
	m.inLineEdit = true
	m.lineEditFile = m.hunkList.CurrentFileIdx()
	m.lineEditHunk = m.hunkList.CurrentHunkIdx()
	m.status.SetHints(m.hintsForContext())
}

// exitLineEdit returns from line-level editing to the hunk list,
// preserving line selections in the hunk.
func (m *HunkAddModel) exitLineEdit() {
	if m.hunkView == nil {
		m.inLineEdit = false
		return
	}
	// Write back the modified hunk with line selections to the hunk list.
	modifiedHunk := m.hunkView.Hunk()
	fileHunks := m.hunkList.FileHunks(m.lineEditFile)
	if m.lineEditHunk < len(fileHunks) {
		fileHunks[m.lineEditHunk] = modifiedHunk
		m.hunkList.ReplaceHunks(m.lineEditFile, fileHunks)
		// Restore cursor position to the same hunk.
		m.hunkList.ScrollToFile(m.lineEditFile)
	}
	m.hunkView = nil
	m.inLineEdit = false
	m.status.SetHints(m.hintsForContext())
	m.renderCurrentHunk()
}

// updateLineEdit handles key messages during line-level editing phase.
func (m *HunkAddModel) updateLineEdit(msg tea.KeyPressMsg, sbCmd tea.Cmd) tea.Cmd {
	if m.help.HandleKey(msg) {
		return sbCmd
	}

	switch msg.Code {
	case tea.KeyEscape:
		m.exitLineEdit()
		return sbCmd
	case 'q':
		m.exitLineEdit()
		return sbCmd
	}

	if m.hunkView != nil {
		m.hunkView.Update(msg)
	}
	return sbCmd
}

func (m *HunkAddModel) renderCurrentHunk() {
	renderHunkPreview(&m.hunkList, &m.diff, m.renderer, &m.prevFileIdx, &m.prevHunkIdx)
}

func (m *HunkAddModel) syncDiffPreview() {
	syncHunkPreview(&m.hunkList, &m.diff, m.renderer, &m.prevFileIdx, &m.prevHunkIdx)
}

func (m *HunkAddModel) applyStaged() tea.Cmd {
	return applyHunks(m.ctx, m.runner, &m.hunkList, m.files, &m.status, git.StageHunk, "Stage")
}

// splitCurrentHunk attempts to split the hunk under the cursor into sub-hunks.
func (m *HunkAddModel) splitCurrentHunk() {
	hunk, ok := m.hunkList.CurrentHunk()
	if !ok {
		return
	}

	sub := splitHunk(hunk, m.contextLines)
	if len(sub) <= 1 {
		_ = m.status.SetMessage("Cannot split hunk further", components.Info)
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
// contextLines controls the surrounding context padding for each sub-hunk.
// Returns the original slice (len==1) if it cannot be split further.
func splitHunk(h git.Hunk, contextLines int) []git.Hunk {
	lines := strings.Split(h.Body(), "\n")
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

	// Build sub-hunks: each keeps surrounding context (up to contextLines) around its changes.
	var result []git.Hunk
	header := lines[0]

	for _, seg := range segments {
		ctxBefore := max(1, seg.start-contextLines)
		ctxAfter := min(len(lines), seg.end+contextLines)

		subLines := []string{header}
		subLines = append(subLines, lines[ctxBefore:ctxAfter]...)

		// Parse the sub-hunk lines into Line structs
		var subParsedLines []git.Line
		for _, sl := range subLines[1:] { // skip header
			subParsedLines = append(subParsedLines, git.ParseLine(sl))
		}
		result = append(result, git.Hunk{
			Header: header,
			Lines:  subParsedLines,
		})
	}

	return result
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
		patch := patchHeader(m.files[sh.FileIdx]) + "\n" + sh.Hunk.Body() + "\n"
		if err := git.StageHunk(m.ctx, m.runner, patch); err != nil {
			lastErr = err
			continue
		}
		applied++
	}

	if applied == 0 {
		if lastErr != nil {
			_ = m.status.SetMessage(fmt.Sprintf("Stage failed: %v", lastErr), components.Error)
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
	diffArgs := []string{"diff", fmt.Sprintf("-U%d", m.contextLines)}
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
	m.left = &m.hunkList
	m.prevFileIdx = 0
	m.prevHunkIdx = 0

	if len(files) > 0 {
		m.renderCurrentHunk()
	}
	m.resize()
}
