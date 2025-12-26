package tui

import (
	"regexp"
	"strings"
	"testing"
)

func TestPaletteColorsAreValidHex(t *testing.T) {
	hex := regexp.MustCompile(`^#[0-9a-f]{6}$`)
	colors := map[string]string{
		"HexBg":       HexBg,
		"HexBgAlt":    HexBgAlt,
		"HexBgSel":    HexBgSel,
		"HexBgFloat":  HexBgFloat,
		"HexFg":       HexFg,
		"HexFgSubtle": HexFgSubtle,
		"HexFgEmph":   HexFgEmph,
		"HexRed":      HexRed,
		"HexOrange":   HexOrange,
		"HexYellow":   HexYellow,
		"HexGreen":    HexGreen,
		"HexCyan":     HexCyan,
		"HexBlue":     HexBlue,
		"HexPurple":   HexPurple,
	}
	if len(colors) != 14 {
		t.Fatalf("expected 14 palette hex constants, got %d", len(colors))
	}
	for name, h := range colors {
		if !hex.MatchString(h) {
			t.Errorf("%s = %q, want #rrggbb", name, h)
		}
	}
}

func TestIconConstantsAreNonEmpty(t *testing.T) {
	icons := map[string]string{
		"IconModified":  IconModified,
		"IconAdded":     IconAdded,
		"IconDeleted":   IconDeleted,
		"IconRenamed":   IconRenamed,
		"IconUntracked": IconUntracked,
		"IconBranch":    IconBranch,
		"IconCommit":    IconCommit,
		"IconChecked":   IconChecked,
		"IconUnchecked": IconUnchecked,
		"IconWarning":   IconWarning,
		"IconError":     IconError,
		"IconSuccess":   IconSuccess,
		"IconDiff":      IconDiff,
		"IconFilter":    IconFilter,
		"IconPick":      IconPick,
		"IconReword":    IconReword,
		"IconEdit":      IconEdit,
		"IconSquash":    IconSquash,
		"IconFixup":     IconFixup,
		"IconDrop":      IconDrop,
	}
	for name, icon := range icons {
		if icon == "" {
			t.Errorf("%s is empty", name)
		}
	}
}

func TestBoldStyleRendersBold(t *testing.T) {
	out := StyleBold.Render("test")
	if !strings.Contains(out, "\033[1m") {
		t.Error("bold style missing bold ANSI attribute")
	}
}

func TestItalicStyleRendersItalic(t *testing.T) {
	out := StyleItalic.Render("test")
	if !strings.Contains(out, "\033[3m") {
		t.Error("italic style missing italic ANSI attribute")
	}
}

func TestStrikethroughStyleRendersStrikethrough(t *testing.T) {
	out := StyleStrikethrough.Render("test")
	if !strings.Contains(out, "\033[9m") {
		t.Error("strikethrough style missing strikethrough ANSI attribute")
	}
}
