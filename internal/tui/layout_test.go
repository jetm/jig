package tui

import "testing"

func TestColumnsAt60(t *testing.T) {
	left, right := Columns(60)
	if left < 28 {
		t.Errorf("left = %d, want >= 28", left)
	}
	if left+right != 60 {
		t.Errorf("left+right = %d, want 60", left+right)
	}
}

func TestColumnsAt80(t *testing.T) {
	left, right := Columns(80)
	if left < 28 {
		t.Errorf("left = %d, want >= 28", left)
	}
	if left+right != 80 {
		t.Errorf("left+right = %d, want 80", left+right)
	}
}

func TestColumnsAt120(t *testing.T) {
	left, right := Columns(120)
	if left != 36 {
		t.Errorf("left = %d, want 36", left)
	}
	if right != 84 {
		t.Errorf("right = %d, want 84", right)
	}
}

func TestColumnsAt200(t *testing.T) {
	left, right := Columns(200)
	if left != 60 {
		t.Errorf("left = %d, want 60", left)
	}
	if right != 140 {
		t.Errorf("right = %d, want 140", right)
	}
}

func TestIsTerminalTooSmallAdequate(t *testing.T) {
	if IsTerminalTooSmall(80, 24) {
		t.Error("80x24 should be adequate")
	}
}

func TestIsTerminalTooSmallTooNarrow(t *testing.T) {
	if !IsTerminalTooSmall(50, 24) {
		t.Error("50x24 should be too small")
	}
}

func TestIsTerminalTooSmallTooShort(t *testing.T) {
	if !IsTerminalTooSmall(80, 8) {
		t.Error("80x8 should be too small")
	}
}
