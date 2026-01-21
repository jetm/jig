package tui_test

import (
	"testing"

	"github.com/jetm/gti/internal/tui"
)

func TestColumnsWide_Standard120(t *testing.T) {
	left, right := tui.ColumnsWide(120)
	if left != 54 {
		t.Errorf("ColumnsWide(120) left = %d, want 54", left)
	}
	if right != 66 {
		t.Errorf("ColumnsWide(120) right = %d, want 66", right)
	}
}

func TestColumnsWide_NarrowFallsToMinimum(t *testing.T) {
	left, right := tui.ColumnsWide(60)
	if left != 28 {
		t.Errorf("ColumnsWide(60) left = %d, want 28 (minimum)", left)
	}
	if right != 32 {
		t.Errorf("ColumnsWide(60) right = %d, want 32", right)
	}
}

func TestColumnsWide_SumEqualsTotal(t *testing.T) {
	for _, w := range []int{60, 80, 100, 120, 160, 200} {
		left, right := tui.ColumnsWide(w)
		if left+right != w {
			t.Errorf("ColumnsWide(%d): left+right = %d+%d = %d, want %d", w, left, right, left+right, w)
		}
	}
}

func TestColumnsWide_Wide200(t *testing.T) {
	left, right := tui.ColumnsWide(200)
	// 45% of 200 = 90
	if left != 90 {
		t.Errorf("ColumnsWide(200) left = %d, want 90", left)
	}
	if right != 110 {
		t.Errorf("ColumnsWide(200) right = %d, want 110", right)
	}
}
