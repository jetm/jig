package tui_test

import (
	"testing"

	"github.com/jetm/jig/internal/tui"
)

func TestColumnsFromConfig_Ratio60Width100(t *testing.T) {
	left, right := tui.ColumnsFromConfig(100, 60)
	if left != 60 {
		t.Errorf("ColumnsFromConfig(100, 60) left = %d, want 60", left)
	}
	if right != 40 {
		t.Errorf("ColumnsFromConfig(100, 60) right = %d, want 40", right)
	}
}

func TestColumnsFromConfig_SumEqualsTotal(t *testing.T) {
	for _, w := range []int{60, 80, 100, 120, 160, 200} {
		left, right := tui.ColumnsFromConfig(w, 40)
		if left+right != w {
			t.Errorf("ColumnsFromConfig(%d, 40): left+right = %d+%d = %d, want %d", w, left, right, left+right, w)
		}
	}
}

func TestColumnsFromConfig_NarrowFallsToMinimum(t *testing.T) {
	left, right := tui.ColumnsFromConfig(60, 40)
	// 40% of 60 = 24, but minimum is 28
	if left != 28 {
		t.Errorf("ColumnsFromConfig(60, 40) left = %d, want 28 (minimum)", left)
	}
	if right != 32 {
		t.Errorf("ColumnsFromConfig(60, 40) right = %d, want 32", right)
	}
}

func TestIsTerminalTooSmall(t *testing.T) {
	t.Parallel()
	if !tui.IsTerminalTooSmall(30, 5) {
		t.Error("30x5 should be too small")
	}
	if tui.IsTerminalTooSmall(120, 40) {
		t.Error("120x40 should not be too small")
	}
	if !tui.IsTerminalTooSmall(0, 0) {
		t.Error("0x0 should be too small")
	}
}
