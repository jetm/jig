package components

import (
	"strings"
	"testing"

	"github.com/jetm/jig/internal/tui"
)

func newTestStatusBar() *StatusBar {
	sb := NewStatusBar(80)
	return &sb
}

func TestStatusBarSetHintsRendersOnLeft(t *testing.T) {
	sb := newTestStatusBar()
	sb.SetHints("<space> stage  <q> quit")
	view := sb.View()
	if !strings.Contains(view, "<space> stage") {
		t.Error("View() should contain hint text")
	}
}

func TestStatusBarSetBranchRendersOnRight(t *testing.T) {
	sb := newTestStatusBar()
	sb.SetBranch("main")
	view := sb.View()
	if !strings.Contains(view, "main") {
		t.Error("View() should contain branch name")
	}
	if !strings.Contains(view, tui.IconBranch) {
		t.Error("View() should contain branch icon")
	}
}

func TestStatusBarSetModeRendersLabel(t *testing.T) {
	sb := newTestStatusBar()
	sb.SetMode("staged")
	view := sb.View()
	if !strings.Contains(view, "staged") {
		t.Error("View() should contain mode label")
	}
}

func TestStatusBarSuccessMessage(t *testing.T) {
	sb := newTestStatusBar()
	sb.SetMessage("staged driver.c", Success)
	view := sb.View()
	if !strings.Contains(view, tui.IconSuccess) {
		t.Error("success message should contain success icon")
	}
	if !strings.Contains(view, "staged driver.c") {
		t.Error("success message should contain message text")
	}
}

func TestStatusBarErrorMessage(t *testing.T) {
	sb := newTestStatusBar()
	sb.SetMessage("nothing to stage", Error)
	view := sb.View()
	if !strings.Contains(view, tui.IconError) {
		t.Error("error message should contain error icon")
	}
	if !strings.Contains(view, "nothing to stage") {
		t.Error("error message should contain message text")
	}
}

func TestStatusBarMessageClearsOnKeypress(t *testing.T) {
	sb := newTestStatusBar()
	sb.SetMessage("test msg", Success)
	sendKey(sb, 'a')
	view := sb.View()
	if strings.Contains(view, "test msg") {
		t.Error("message should be cleared after keypress")
	}
}

func TestStatusBarInfoMessage(t *testing.T) {
	sb := newTestStatusBar()
	sb.SetMessage("info text", Info)
	view := sb.View()
	if !strings.Contains(view, "info text") {
		t.Error("info message should contain message text")
	}
}

func TestStatusBarClearMessageMsg(t *testing.T) {
	sb := newTestStatusBar()
	sb.SetMessage("will clear", Success)
	sb.Update(ClearMessageMsg{})
	view := sb.View()
	if strings.Contains(view, "will clear") {
		t.Error("message should be cleared by ClearMessageMsg")
	}
}

func TestStatusBarSetWidth(t *testing.T) {
	sb := newTestStatusBar()
	sb.SetWidth(120)
	sb.SetHints("test")
	if sb.View() == "" {
		t.Error("View() should not be empty after SetWidth")
	}
}

func TestStatusBarSetMessageReturnsCmd(t *testing.T) {
	sb := newTestStatusBar()
	cmd := sb.SetMessage("timed", Success)
	if cmd == nil {
		t.Error("SetMessage should return a non-nil tea.Cmd for auto-clear")
	}
}

func TestStatusBarViewWithAllFields(t *testing.T) {
	sb := newTestStatusBar()
	sb.SetHints("<q> quit")
	sb.SetBranch("feature")
	sb.SetMode("unstaged")
	view := sb.View()
	if !strings.Contains(view, "<q> quit") {
		t.Error("should contain hints")
	}
	if !strings.Contains(view, "feature") {
		t.Error("should contain branch")
	}
	if !strings.Contains(view, "unstaged") {
		t.Error("should contain mode")
	}
}
