package commands

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/testhelper"
	"github.com/jetm/jig/internal/tui/components"
)

// fakePanel implements tui.LeftPanel for testing.
type fakePanel struct {
	width, height int
	content       string
}

func (f *fakePanel) View() string           { return f.content }
func (f *fakePanel) SetWidth(w int)         { f.width = w }
func (f *fakePanel) SetHeight(h int)        { f.height = h }
func (f *fakePanel) Update(tea.Msg) tea.Cmd { return nil }

// spyPanel records all messages it receives via Update.
type spyPanel struct {
	fakePanel
	messages []tea.Msg
}

func (s *spyPanel) Update(msg tea.Msg) tea.Cmd {
	s.messages = append(s.messages, msg)
	return nil
}

func newSpyTwoPanel() (*twoPanelModel, *spyPanel) {
	cfg := config.NewDefault()
	spy := &spyPanel{fakePanel: fakePanel{content: "left"}}
	diff := components.NewDiffView(80, 20, true)
	status := components.NewStatusBar(120)
	help := components.NewHelpOverlay(nil)
	tp := newTwoPanelModel(spy, diff, status, help, cfg)
	tp.width = 120
	tp.height = 40
	tp.setHints("left hints", "right hints", "maximize hints")
	return &tp, spy
}

func newTestTwoPanel() *twoPanelModel {
	cfg := config.NewDefault()
	left := &fakePanel{content: "left"}
	diff := components.NewDiffView(80, 20, true)
	status := components.NewStatusBar(120)
	help := components.NewHelpOverlay(nil)
	tp := newTwoPanelModel(left, diff, status, help, cfg)
	tp.width = 120
	tp.height = 40
	tp.setHints("left hints", "right hints", "maximize hints")
	return &tp
}

func TestTwoPanelModel_HandleKey_Tab_TogglesFocus(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true

	assert.False(t, tp.focusRight, "should start focused left")

	_, handled := tp.handleKey(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.True(t, handled)
	assert.True(t, tp.focusRight, "Tab should toggle to right")

	_, handled = tp.handleKey(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.True(t, handled)
	assert.False(t, tp.focusRight, "Tab should toggle back to left")
}

func TestTwoPanelModel_HandleKey_Tab_NoopWhenDiffHidden(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = false

	_, handled := tp.handleKey(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.True(t, handled)
	assert.False(t, tp.focusRight, "Tab should be consumed but not toggle when diff hidden")
}

func TestTwoPanelModel_HandleKey_Tab_NoopWhenMaximized(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true
	tp.diffMaximized = true

	_, handled := tp.handleKey(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.True(t, handled)
	assert.False(t, tp.focusRight, "Tab should not toggle when maximized")
}

func TestTwoPanelModel_HandleKey_D_TogglesDiff(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true

	_, handled := tp.handleKey(tea.KeyPressMsg{Code: 'D', Text: "D"})
	assert.True(t, handled)
	assert.False(t, tp.showDiff, "D should toggle diff off")

	_, handled = tp.handleKey(tea.KeyPressMsg{Code: 'D', Text: "D"})
	assert.True(t, handled)
	assert.True(t, tp.showDiff, "D should toggle diff on")
}

func TestTwoPanelModel_HandleKey_F_TogglesMaximize(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true

	_, handled := tp.handleKey(tea.KeyPressMsg{Code: 'F', Text: "F"})
	assert.True(t, handled)
	assert.True(t, tp.diffMaximized, "F should maximize diff")

	_, handled = tp.handleKey(tea.KeyPressMsg{Code: 'F', Text: "F"})
	assert.True(t, handled)
	assert.False(t, tp.diffMaximized, "F should restore diff")
}

func TestTwoPanelModel_HandleKey_F_NoopWhenDiffHidden(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = false

	_, handled := tp.handleKey(tea.KeyPressMsg{Code: 'F', Text: "F"})
	assert.True(t, handled)
	assert.False(t, tp.diffMaximized, "F should not maximize when diff hidden")
}

func TestTwoPanelModel_HandleKey_W_TogglesSoftWrap(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.focusRight = true

	initialWrap := tp.diff.SoftWrap()
	_, handled := tp.handleKey(tea.KeyPressMsg{Code: 'w', Text: "w"})
	assert.True(t, handled)
	assert.NotEqual(t, initialWrap, tp.diff.SoftWrap(), "w should toggle soft wrap")
}

func TestTwoPanelModel_HandleKey_W_NotHandledWhenFocusLeft(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.focusRight = false

	_, handled := tp.handleKey(tea.KeyPressMsg{Code: 'w', Text: "w"})
	assert.False(t, handled, "w should not be consumed when focused left")
}

func TestTwoPanelModel_HandleKey_BracketLeft_DecreasesRatio(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.panelRatio = 40

	_, handled := tp.handleKey(tea.KeyPressMsg{Code: '[', Text: "["})
	assert.True(t, handled)
	assert.Equal(t, 35, tp.panelRatio)
}

func TestTwoPanelModel_HandleKey_BracketLeft_ClampsAt20(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.panelRatio = 22

	_, handled := tp.handleKey(tea.KeyPressMsg{Code: '[', Text: "["})
	assert.True(t, handled)
	assert.Equal(t, 20, tp.panelRatio)
}

func TestTwoPanelModel_HandleKey_BracketRight_IncreasesRatio(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.panelRatio = 40

	_, handled := tp.handleKey(tea.KeyPressMsg{Code: ']', Text: "]"})
	assert.True(t, handled)
	assert.Equal(t, 45, tp.panelRatio)
}

func TestTwoPanelModel_HandleKey_BracketRight_ClampsAt80(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.panelRatio = 78

	_, handled := tp.handleKey(tea.KeyPressMsg{Code: ']', Text: "]"})
	assert.True(t, handled)
	assert.Equal(t, 80, tp.panelRatio)
}

func TestTwoPanelModel_HandleKey_UnhandledKey(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()

	_, handled := tp.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, handled, "Enter should not be consumed")

	_, handled = tp.handleKey(tea.KeyPressMsg{Code: ' ', Text: " "})
	assert.False(t, handled, "Space should not be consumed")
}

func TestTwoPanelModel_Resize_DiffHidden(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = false
	tp.resize()

	left := tp.left.(*fakePanel)
	assert.Equal(t, tp.width-1, left.width)
	assert.Equal(t, tp.height-1, left.height)
}

func TestTwoPanelModel_Resize_DiffMaximized(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true
	tp.diffMaximized = true
	tp.resize()

	// DiffView dimensions are set internally; we just verify no panic.
}

func TestTwoPanelModel_Resize_SplitPanels(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true
	tp.diffMaximized = false
	tp.resize()

	left := tp.left.(*fakePanel)
	assert.Positive(t, left.width)
	assert.Equal(t, tp.height-1, left.height)
}

func TestTwoPanelModel_RenderLayout_DiffHidden(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = false

	output := tp.renderLayout()
	require.NotEmpty(t, output, "renderLayout should produce output")
}

func TestTwoPanelModel_RenderLayout_DiffMaximized(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true
	tp.diffMaximized = true

	output := tp.renderLayout()
	require.NotEmpty(t, output, "renderLayout should produce output")
}

func TestTwoPanelModel_RenderLayout_SplitPanels(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true
	tp.diffMaximized = false

	output := tp.renderLayout()
	require.NotEmpty(t, output, "renderLayout should produce output")
}

func TestTwoPanelModel_SetHints_UpdatesStatusBar(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()

	// Default: focused left
	tp.focusRight = false
	tp.diffMaximized = false
	tp.setHints("LEFT", "RIGHT", "MAX")
	// After setHints, updateHints is called => status bar gets "LEFT"

	tp.focusRight = true
	tp.updateHints()
	// Now status bar should have "RIGHT"

	tp.diffMaximized = true
	tp.updateHints()
	// Now status bar should have "MAX"

	// We can't directly read status bar hints, but we verify no panic.
}

func TestHunkAddLeftPanel_InterfaceMethods(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{singleHunkDiff, "main"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)

	p := &hunkAddLeftPanel{m: m}
	p.SetWidth(80)
	p.SetHeight(20)
	_ = p.View()
	_ = p.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})

	// In line-edit mode
	m.enterLineEdit()
	p.SetWidth(80)
	p.SetHeight(20)
	_ = p.View()
}

func TestTwoPanelModel_HandleKey_ConfigSaveError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent/no/write")
	tp := newTestTwoPanel()
	tp.panelRatio = 40

	cmd, handled := tp.handleKey(tea.KeyPressMsg{Code: ']', Text: "]"})
	assert.True(t, handled)
	assert.NotNil(t, cmd, "should return error cmd when config save fails")
}

func TestTwoPanelModel_HandleKey_BracketLeft_ConfigSaveError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent/no/write")
	tp := newTestTwoPanel()
	tp.panelRatio = 40

	cmd, handled := tp.handleKey(tea.KeyPressMsg{Code: '[', Text: "["})
	assert.True(t, handled)
	assert.NotNil(t, cmd, "should return error cmd when config save fails")
}

func TestCheckoutModel_ConfirmationInView_Maximized(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40
	m.diffMaximized = true
	m.showDiff = true

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := m.View()
	if !strings.Contains(view, "Discard changes") {
		t.Errorf("View() in maximized mode should show confirmation prompt, got: %q", view)
	}
}

func TestCheckoutModel_ConfirmationInView_NoDiff(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40
	m.showDiff = false

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := m.View()
	if !strings.Contains(view, "Discard changes") {
		t.Errorf("View() with diff hidden should show confirmation prompt, got: %q", view)
	}
}

func TestHunkCheckoutModel_ConfirmationInView_Maximized(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{singleHunkDiff, "main"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkCheckoutModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40
	m.diffMaximized = true
	m.showDiff = true

	// Toggle hunk selection and trigger confirm
	m.hunkList.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := m.View()
	if !strings.Contains(view, "Discard") {
		t.Errorf("View() in maximized mode should show confirmation, got: %q", view)
	}
}

func TestHunkCheckoutModel_ConfirmationInView_NoDiff(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{singleHunkDiff, "main"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkCheckoutModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40
	m.showDiff = false

	m.hunkList.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := m.View()
	if !strings.Contains(view, "Discard") {
		t.Errorf("View() with diff hidden should show confirmation, got: %q", view)
	}
}

func TestHunkResetModel_RenderCurrentHunk_NoHunks(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", "main"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkResetModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)

	m.width = 120
	m.height = 40
	m.renderCurrentHunk()
	// With no hunks, diff should show placeholder text
}

func TestHunkAddModel_RenderCurrentHunk_NoHunks(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", "main"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)

	m.width = 120
	m.height = 40
	m.renderCurrentHunk()
}

func TestHunkCheckoutModel_RenderCurrentHunk_NoHunks(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", "main"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkCheckoutModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)

	m.width = 120
	m.height = 40
	m.renderCurrentHunk()
}

func TestHunkResetModel_SyncDiffPreview_SamePosition(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{singleHunkDiff, "main"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkResetModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)

	m.width = 120
	m.height = 40
	// syncDiffPreview should not re-render when position unchanged
	m.syncDiffPreview()
}

func TestHunkCheckoutModel_SyncDiffPreview_SamePosition(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{singleHunkDiff, "main"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkCheckoutModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)

	m.width = 120
	m.height = 40
	m.syncDiffPreview()
}

func TestTwoPanelModel_HandleKey_Slash_EntersSearchMode(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true
	tp.focusRight = true

	_, consumed := tp.handleKey(tea.KeyPressMsg{Code: '/'})
	assert.True(t, consumed)
	assert.True(t, tp.searchMode)
}

func TestTwoPanelModel_HandleKey_Slash_IgnoredWhenNotFocusRight(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true
	tp.focusRight = false

	_, consumed := tp.handleKey(tea.KeyPressMsg{Code: '/'})
	assert.False(t, consumed)
	assert.False(t, tp.searchMode)
}

func TestTwoPanelModel_HandleKey_Esc_ClearsSearch(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true
	tp.focusRight = true
	tp.diff.SetContent("foo bar foo")
	tp.diff.Search("foo")
	tp.searchMode = true

	_, consumed := tp.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.True(t, consumed)
	assert.False(t, tp.searchMode)
	assert.False(t, tp.diff.HasSearch())
}

func TestTwoPanelModel_HandleKey_N_NavigatesSearch(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true
	tp.focusRight = true
	tp.diff.SetContent("foo bar foo baz foo")
	tp.diff.Search("foo")

	_, consumed := tp.handleKey(tea.KeyPressMsg{Code: 'n'})
	assert.True(t, consumed)

	_, consumed = tp.handleKey(tea.KeyPressMsg{Code: 'N'})
	assert.True(t, consumed)
}

func TestTwoPanelModel_HandleKey_Slash_SearchInputEnterSubmits(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true
	tp.focusRight = true
	tp.diff.SetContent("foo bar foo baz")

	// Enter search mode
	tp.handleKey(tea.KeyPressMsg{Code: '/'})
	require.True(t, tp.searchMode)
	require.True(t, tp.searchInput.Focused())

	// Type search query
	tp.handleKey(tea.KeyPressMsg{Code: 'f', Text: "f"})
	tp.handleKey(tea.KeyPressMsg{Code: 'o', Text: "o"})
	tp.handleKey(tea.KeyPressMsg{Code: 'o', Text: "o"})

	// Submit search
	tp.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, tp.diff.HasSearch())
	assert.Equal(t, 2, tp.diff.MatchCount())
	assert.False(t, tp.searchInput.Focused())
}

func TestTwoPanelModel_HandleKey_Slash_EmptySearchCancels(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true
	tp.focusRight = true

	// Enter search mode
	tp.handleKey(tea.KeyPressMsg{Code: '/'})
	require.True(t, tp.searchMode)

	// Submit empty search
	tp.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, tp.searchMode)
}

func TestTwoPanelModel_HandleKey_Slash_EscCancelsInput(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true
	tp.focusRight = true

	// Enter search mode
	tp.handleKey(tea.KeyPressMsg{Code: '/'})
	require.True(t, tp.searchMode)

	// Escape while input is focused
	tp.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, tp.searchMode)
}

func TestTwoPanelModel_HandleKey_NoMatchesShowsMessage(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true
	tp.focusRight = true
	tp.diff.SetContent("foo bar baz")

	// Enter search mode
	tp.handleKey(tea.KeyPressMsg{Code: '/'})

	// Type non-matching query
	tp.handleKey(tea.KeyPressMsg{Code: 'x', Text: "x"})
	tp.handleKey(tea.KeyPressMsg{Code: 'y', Text: "y"})
	tp.handleKey(tea.KeyPressMsg{Code: 'z', Text: "z"})

	// Submit
	cmd, consumed := tp.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, consumed)
	assert.NotNil(t, cmd, "should return a status message command for no matches")
	assert.Equal(t, 0, tp.diff.MatchCount())
}

func TestTwoPanelModel_BottomBar_ShowsSearchInput(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true
	tp.focusRight = true
	tp.width = 120
	tp.height = 40

	// Not in search mode - shows status bar
	bar := tp.bottomBar()
	assert.NotEmpty(t, bar)

	// Enter search mode
	tp.handleKey(tea.KeyPressMsg{Code: '/'})
	bar = tp.bottomBar()
	assert.Contains(t, bar, "/", "search input should show the / prompt")
}

func TestTwoPanelModel_RenderLayout_SearchMode(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true
	tp.focusRight = true
	tp.width = 120
	tp.height = 40

	// Enter search mode and render
	tp.handleKey(tea.KeyPressMsg{Code: '/'})
	layout := tp.renderLayout()
	assert.Contains(t, layout, "/", "layout should contain search prompt when in search mode")
}

func TestTwoPanelModel_Maximize_SetsFocusRight(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true

	assert.False(t, tp.focusRight, "should start focused left")

	tp.handleKey(tea.KeyPressMsg{Code: 'F', Text: "F"})

	assert.True(t, tp.diffMaximized, "F should maximize diff")
	assert.True(t, tp.focusRight, "entering maximize should set focusRight=true")
}

func TestTwoPanelModel_Maximize_JK_RoutedToFileList(t *testing.T) {
	t.Parallel()
	tp, spy := newSpyTwoPanel()
	tp.showDiff = true

	// Enter maximize mode.
	tp.handleKey(tea.KeyPressMsg{Code: 'F', Text: "F"})
	require.True(t, tp.diffMaximized)

	// Send j — should be intercepted and forwarded to the left panel.
	_, handledJ := tp.handleKey(tea.KeyPressMsg{Code: 'j', Text: "j"})
	assert.True(t, handledJ, "j should be handled in maximize mode")

	// Send k — same behavior.
	_, handledK := tp.handleKey(tea.KeyPressMsg{Code: 'k', Text: "k"})
	assert.True(t, handledK, "k should be handled in maximize mode")

	// The spy panel should have received both key messages.
	require.Len(t, spy.messages, 2, "left panel should receive j and k messages")
	assert.Equal(t, tea.KeyPressMsg{Code: 'j', Text: "j"}, spy.messages[0])
	assert.Equal(t, tea.KeyPressMsg{Code: 'k', Text: "k"}, spy.messages[1])
}

func TestTwoPanelModel_Maximize_ArrowKeys_NotIntercepted(t *testing.T) {
	t.Parallel()
	tp, spy := newSpyTwoPanel()
	tp.showDiff = true

	// Enter maximize mode.
	tp.handleKey(tea.KeyPressMsg{Code: 'F', Text: "F"})
	require.True(t, tp.diffMaximized)

	// Arrow keys should NOT be handled by handleKey — they pass through
	// to the parent, which routes them to the viewport via focusRight.
	_, handledUp := tp.handleKey(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.False(t, handledUp, "Up arrow should not be handled by handleKey in maximize mode")

	_, handledDown := tp.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.False(t, handledDown, "Down arrow should not be handled by handleKey in maximize mode")

	// Left panel should NOT have received arrow keys.
	assert.Empty(t, spy.messages, "arrow keys should not reach the left panel in maximize mode")
}

func TestTwoPanelModel_Maximize_Exit_PreservesFocusRight(t *testing.T) {
	t.Parallel()
	tp := newTestTwoPanel()
	tp.showDiff = true

	// Enter maximize.
	tp.handleKey(tea.KeyPressMsg{Code: 'F', Text: "F"})
	require.True(t, tp.diffMaximized)
	require.True(t, tp.focusRight, "maximize should set focusRight")

	// Exit maximize.
	tp.handleKey(tea.KeyPressMsg{Code: 'F', Text: "F"})
	assert.False(t, tp.diffMaximized, "F should exit maximize")
	assert.True(t, tp.focusRight, "exiting maximize should preserve focusRight=true")
}

func TestTwoPanelModel_Maximize_PageKeys_NotIntercepted(t *testing.T) {
	t.Parallel()
	tp, spy := newSpyTwoPanel()
	tp.showDiff = true

	// Enter maximize mode.
	tp.handleKey(tea.KeyPressMsg{Code: 'F', Text: "F"})
	require.True(t, tp.diffMaximized)

	// PgUp/PgDn should NOT be handled — they pass through to the viewport.
	_, handledPgUp := tp.handleKey(tea.KeyPressMsg{Code: tea.KeyPgUp})
	assert.False(t, handledPgUp, "PgUp should not be handled by handleKey in maximize mode")

	_, handledPgDn := tp.handleKey(tea.KeyPressMsg{Code: tea.KeyPgDown})
	assert.False(t, handledPgDn, "PgDn should not be handled by handleKey in maximize mode")

	// Left panel should NOT have received page keys.
	assert.Empty(t, spy.messages, "page keys should not reach the left panel in maximize mode")
}
