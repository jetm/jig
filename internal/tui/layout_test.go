package tui_test

import (
	"testing"

	"github.com/jetm/gti/internal/tui"
)

func TestColumns_Standard80(t *testing.T) {
	left, right := tui.Columns(80)
	// 40% of 80 = 32
	if left != 32 {
		t.Errorf("Columns(80) left = %d, want 32", left)
	}
	if right != 48 {
		t.Errorf("Columns(80) right = %d, want 48", right)
	}
}

func TestColumns_Wide120(t *testing.T) {
	left, right := tui.Columns(120)
	// 40% of 120 = 48
	if left != 48 {
		t.Errorf("Columns(120) left = %d, want 48", left)
	}
	if right != 72 {
		t.Errorf("Columns(120) right = %d, want 72", right)
	}
}

func TestColumns_VeryWide200(t *testing.T) {
	left, right := tui.Columns(200)
	// 40% of 200 = 80
	if left != 80 {
		t.Errorf("Columns(200) left = %d, want 80", left)
	}
	if right != 120 {
		t.Errorf("Columns(200) right = %d, want 120", right)
	}
}

func TestColumns_NarrowFallsToMinimum(t *testing.T) {
	left, right := tui.Columns(60)
	// 40% of 60 = 24, but minimum is 28
	if left != 28 {
		t.Errorf("Columns(60) left = %d, want 28 (minimum)", left)
	}
	if right != 32 {
		t.Errorf("Columns(60) right = %d, want 32", right)
	}
}

func TestColumns_SumEqualsTotal(t *testing.T) {
	for _, w := range []int{60, 80, 100, 120, 160, 200} {
		left, right := tui.Columns(w)
		if left+right != w {
			t.Errorf("Columns(%d): left+right = %d+%d = %d, want %d", w, left, right, left+right, w)
		}
	}
}

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

func TestColumns_DelegatesCorrectly(t *testing.T) {
	// Columns should produce same result as ColumnsFromConfig with ratio=40
	for _, w := range []int{80, 100, 120, 200} {
		wantLeft, wantRight := tui.ColumnsFromConfig(w, 40)
		gotLeft, gotRight := tui.Columns(w)
		if gotLeft != wantLeft || gotRight != wantRight {
			t.Errorf("Columns(%d) = (%d, %d), want ColumnsFromConfig(%d, 40) = (%d, %d)",
				w, gotLeft, gotRight, w, wantLeft, wantRight)
		}
	}
}

func TestColumnsWide_DelegatesCorrectly(t *testing.T) {
	// ColumnsWide should produce same result as ColumnsFromConfig with ratio=45
	for _, w := range []int{80, 100, 120, 200} {
		wantLeft, wantRight := tui.ColumnsFromConfig(w, 45)
		gotLeft, gotRight := tui.ColumnsWide(w)
		if gotLeft != wantLeft || gotRight != wantRight {
			t.Errorf("ColumnsWide(%d) = (%d, %d), want ColumnsFromConfig(%d, 45) = (%d, %d)",
				w, gotLeft, gotRight, w, wantLeft, wantRight)
		}
	}
}
