package commands

import (
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/tui"
	"github.com/jetm/jig/internal/tui/components"
)

// twoPanelModel encapsulates the shared layout state and behavior for all
// two-panel command models. Commands embed this struct and delegate universal
// key handling, resize, and View rendering to it.
type twoPanelModel struct {
	left   tui.LeftPanel
	diff   components.DiffView
	status components.StatusBar
	help   components.HelpOverlay
	cfg    config.Config

	width         int
	height        int
	panelRatio    int
	focusRight    bool
	showDiff      bool
	diffMaximized bool

	hintsLeft     string
	hintsRight    string
	hintsMaximize string

	searchMode  bool
	searchInput textinput.Model

	// leftUpdated is set by handleKey when a maximize-mode intercept
	// forwarded a key to the left panel. Callers should check this flag
	// and trigger their diff re-render, then clear it.
	leftUpdated bool
}

// newTwoPanelModel creates a twoPanelModel with the given configuration.
func newTwoPanelModel(
	left tui.LeftPanel,
	diff components.DiffView,
	status components.StatusBar,
	help components.HelpOverlay,
	cfg config.Config,
) twoPanelModel {
	tp := twoPanelModel{
		left:       left,
		diff:       diff,
		status:     status,
		help:       help,
		cfg:        cfg,
		panelRatio: cfg.PanelRatio,
		showDiff:   cfg.ShowDiffPanel,
	}
	tp.diff.SetSoftWrap(cfg.SoftWrap)
	return tp
}

// setHints configures the three hint strings used by the status bar.
// twoPanelModel calls updateHints internally when focus/maximize state changes.
func (tp *twoPanelModel) setHints(left, right, maximize string) {
	tp.hintsLeft = left
	tp.hintsRight = right
	tp.hintsMaximize = maximize
	tp.updateHints()
}

// updateHints sets the status bar hints based on the current focus and maximize state.
func (tp *twoPanelModel) updateHints() {
	switch {
	case tp.diffMaximized:
		tp.status.SetHints(tp.hintsMaximize)
	case tp.focusRight:
		tp.status.SetHints(tp.hintsRight)
	default:
		tp.status.SetHints(tp.hintsLeft)
	}
}

// handleKey processes universal two-panel keys: Tab, D, F, w, [, ], /, n, N, Esc.
// Returns the command to execute and whether the key was consumed.
// Commands must call this before processing their own keys.
func (tp *twoPanelModel) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	// When search input is active, route keys to the text input.
	if tp.searchMode && tp.searchInput.Focused() {
		switch msg.Code {
		case tea.KeyEnter:
			query := tp.searchInput.Value()
			tp.searchInput.Blur()
			if query == "" {
				tp.searchMode = false
				return nil, true
			}
			tp.diff.Search(query)
			if tp.diff.MatchCount() == 0 {
				return tp.status.SetMessage("No matches", components.Error), true
			}
			return tp.status.SetMessage(
				fmt.Sprintf("%d matches", tp.diff.MatchCount()), components.Success,
			), true
		case tea.KeyEscape:
			tp.searchMode = false
			tp.searchInput.Blur()
			return nil, true
		default:
			tp.searchInput, _ = tp.searchInput.Update(msg)
			return nil, true
		}
	}

	if msg.Code == tea.KeyTab {
		if tp.showDiff && !tp.diffMaximized {
			tp.focusRight = !tp.focusRight
			tp.updateHints()
		}
		return nil, true
	}

	// When maximized, intercept j/k/Space so they reach the left panel
	// instead of scrolling the viewport. We set leftUpdated so the caller
	// knows to forward the message to its left panel and re-render the
	// diff. We don't call tp.left.Update() here because some callers
	// reassign tp.left in View(), so the pointer may be stale.
	if tp.diffMaximized {
		if s := msg.String(); s == "j" || s == "k" || s == "space" {
			tp.leftUpdated = true
			return nil, true
		}
	}

	switch msg.String() {
	case "/":
		if tp.focusRight {
			tp.searchMode = true
			tp.searchInput = textinput.New()
			tp.searchInput.Prompt = "/"
			tp.searchInput.Focus()
			return nil, true
		}
		return nil, false

	case "n":
		if tp.diff.HasSearch() {
			tp.diff.SearchNext()
			return nil, true
		}
		return nil, false

	case "N":
		if tp.diff.HasSearch() {
			tp.diff.SearchPrev()
			return nil, true
		}
		return nil, false

	case "D":
		tp.showDiff = !tp.showDiff
		return nil, true

	case "F":
		if tp.showDiff {
			tp.diffMaximized = !tp.diffMaximized
			if tp.diffMaximized {
				tp.focusRight = true
			}
			tp.updateHints()
		}
		return nil, true
	case "w":
		if tp.focusRight {
			tp.diff.SetSoftWrap(!tp.diff.SoftWrap())
			return nil, true
		}
		return nil, false

	case "[":
		if tp.panelRatio > 20 {
			tp.panelRatio -= 5
			if tp.panelRatio < 20 {
				tp.panelRatio = 20
			}
			tp.cfg.PanelRatio = tp.panelRatio
			if err := config.Save(tp.cfg); err != nil {
				return tp.status.SetMessage(fmt.Sprintf("Config save failed: %v", err), components.Error), true
			}
			tp.resize()
		}
		return nil, true

	case "]":
		if tp.panelRatio < 80 {
			tp.panelRatio += 5
			if tp.panelRatio > 80 {
				tp.panelRatio = 80
			}
			tp.cfg.PanelRatio = tp.panelRatio
			if err := config.Save(tp.cfg); err != nil {
				return tp.status.SetMessage(fmt.Sprintf("Config save failed: %v", err), components.Error), true
			}
			tp.resize()
		}
		return nil, true
	}

	// Esc clears search if active, otherwise not consumed.
	if msg.Code == tea.KeyEscape && tp.diff.HasSearch() {
		tp.searchMode = false
		tp.diff.ClearSearch()
		return nil, true
	}

	return nil, false
}

// resize recalculates component dimensions based on the current width, height,
// showDiff, diffMaximized, and panelRatio state.
func (tp *twoPanelModel) resize() {
	contentHeight := tp.height - 1
	tp.status.SetWidth(tp.width)

	if !tp.showDiff {
		panelW := tp.width - 1
		tp.left.SetWidth(panelW)
		tp.left.SetHeight(contentHeight)
		return
	}

	if tp.diffMaximized {
		rightW := tp.width - 1
		tp.diff.SetWidth(rightW)
		tp.diff.SetHeight(contentHeight)
		return
	}

	leftW, rightW := tui.ColumnsFromConfig(tp.width, tp.panelRatio)
	leftW--
	rightW--

	tp.left.SetWidth(leftW)
	tp.left.SetHeight(contentHeight)
	tp.diff.SetWidth(rightW)
	tp.diff.SetHeight(contentHeight)
}

// bottomBar returns the search input view when in search mode, or the status bar otherwise.
func (tp *twoPanelModel) bottomBar() string {
	if tp.searchMode && tp.searchInput.Focused() {
		return tp.searchInput.View()
	}
	return tp.status.View()
}

// renderLayout produces the two-panel layout string respecting showDiff,
// diffMaximized, and focusRight state. The status bar is included at the bottom.
func (tp *twoPanelModel) renderLayout() string {
	contentHeight := tp.height - 1
	tp.status.SetWidth(tp.width)

	switch {
	case !tp.showDiff:
		panelW := tp.width - 1
		tp.left.SetWidth(panelW)
		tp.left.SetHeight(contentHeight)
		leftPanel := tui.StyleFocusBorder.Width(panelW).Height(contentHeight).MaxHeight(contentHeight).Render(tp.left.View())
		return leftPanel + "\n" + tp.bottomBar()

	case tp.diffMaximized:
		rightW := tp.width - 1
		tp.diff.SetWidth(rightW)
		tp.diff.SetHeight(contentHeight)
		rightPanel := tui.StyleFocusBorder.Width(rightW).Height(contentHeight).MaxHeight(contentHeight).Render(tp.diff.View())
		return rightPanel + "\n" + tp.bottomBar()

	default:
		leftW, rightW := tui.ColumnsFromConfig(tp.width, tp.panelRatio)
		leftW--
		rightW--

		tp.left.SetWidth(leftW)
		tp.left.SetHeight(contentHeight)
		tp.diff.SetWidth(rightW)
		tp.diff.SetHeight(contentHeight)

		leftBorder, rightBorder := tui.StyleFocusBorder, tui.StyleDimBorder
		if tp.focusRight {
			leftBorder, rightBorder = tui.StyleDimBorder, tui.StyleFocusBorder
		}

		leftPanel := leftBorder.Width(leftW).Height(contentHeight).MaxHeight(contentHeight).Render(tp.left.View())
		rightPanel := rightBorder.Width(rightW).Height(contentHeight).MaxHeight(contentHeight).Render(tp.diff.View())

		panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
		return panels + "\n" + tp.bottomBar()
	}
}

// isDeltaRenderer reports whether r is a *diff.DeltaRenderer.
func isDeltaRenderer(r diff.Renderer) bool {
	_, ok := r.(*diff.DeltaRenderer)
	return ok
}
