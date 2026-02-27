package tui_test

import (
	"github.com/jetm/jig/internal/tui"
	"github.com/jetm/jig/internal/tui/components"
)

// Compile-time interface satisfaction checks.
var (
	_ tui.LeftPanel = (*components.FileList)(nil)
	_ tui.LeftPanel = (*components.ItemList)(nil)
	_ tui.LeftPanel = (*components.HunkList)(nil)
)
