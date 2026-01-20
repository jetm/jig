package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/tui"
)

// FileList is a flat one-file-per-line list component with cursor navigation,
// optional checkboxes, and viewport scrolling. It implements the same method
// surface as FileTree so commands can swap constructors mechanically.
type FileList struct {
	entries      []FileEntry
	checked      []bool
	cursor       int
	offset       int
	width        int
	height       int
	showCheckbox bool
}

// NewFileList creates a FileList from a slice of file entries.
// showCheckbox enables checkbox rendering for selection-based views (add, checkout, reset).
func NewFileList(entries []FileEntry, showCheckbox bool) FileList {
	checked := make([]bool, len(entries))
	return FileList{
		entries:      entries,
		checked:      checked,
		showCheckbox: showCheckbox,
	}
}

// SetWidth sets the viewport width used when rendering rows.
func (fl *FileList) SetWidth(w int) { fl.width = w }

// SetHeight sets the viewport height (number of visible rows).
func (fl *FileList) SetHeight(h int) { fl.height = h }

// SelectedPath returns the full relative path of the file at the cursor, or
// empty string if the list is empty.
func (fl *FileList) SelectedPath() string {
	if len(fl.entries) == 0 {
		return ""
	}
	return fl.entries[fl.cursor].Path
}

// CheckedPaths returns the full paths of all checked file entries.
func (fl *FileList) CheckedPaths() []string {
	var paths []string
	for i, e := range fl.entries {
		if fl.checked[i] {
			paths = append(paths, e.Path)
		}
	}
	return paths
}

// ToggleChecked flips the checked state of the entry at the current cursor.
func (fl *FileList) ToggleChecked() {
	if len(fl.entries) == 0 {
		return
	}
	fl.checked[fl.cursor] = !fl.checked[fl.cursor]
}

// SetAllChecked sets every entry's checked state to the given value.
func (fl *FileList) SetAllChecked(checked bool) {
	for i := range fl.checked {
		fl.checked[i] = checked
	}
}

// Update handles tea messages. It moves the cursor on j/k/up/down key presses
// and adjusts the scroll offset to keep the cursor visible.
func (fl *FileList) Update(msg tea.Msg) tea.Cmd {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	switch kp.Code {
	case 'j', tea.KeyDown:
		if fl.cursor < len(fl.entries)-1 {
			fl.cursor++
			fl.clampOffset()
		}
	case 'k', tea.KeyUp:
		if fl.cursor > 0 {
			fl.cursor--
			fl.clampOffset()
		}
	}
	return nil
}

// clampOffset adjusts the scroll offset so the cursor row is always visible.
func (fl *FileList) clampOffset() {
	if fl.height <= 0 {
		return
	}
	if fl.cursor < fl.offset {
		fl.offset = fl.cursor
	}
	if fl.cursor >= fl.offset+fl.height {
		fl.offset = fl.cursor - fl.height + 1
	}
}

// View renders the visible slice of file entries as a string.
// The cursor row is highlighted. Each row shows: [checkbox] status-icon path.
func (fl *FileList) View() string {
	if len(fl.entries) == 0 {
		return ""
	}

	// Determine visible slice bounds.
	start := fl.offset
	end := len(fl.entries)
	if fl.height > 0 && start+fl.height < end {
		end = start + fl.height
	}
	if start > len(fl.entries) {
		start = len(fl.entries)
	}
	visible := fl.entries[start:end]

	normalStyle := lipgloss.NewStyle()
	cursorStyle := lipgloss.NewStyle().Background(tui.ColorBgSel)

	var sb strings.Builder
	for i, e := range visible {
		absIdx := start + i
		style := normalStyle
		if absIdx == fl.cursor {
			style = cursorStyle
		}

		var row strings.Builder
		if fl.showCheckbox {
			if fl.checked[absIdx] {
				row.WriteString(tui.IconChecked)
			} else {
				row.WriteString(tui.IconUnchecked)
			}
			row.WriteString(" ")
		}
		row.WriteString(fileListStatusIcon(e.Status))
		row.WriteString(" ")
		row.WriteString(e.Path)

		line := row.String()
		if fl.width > 0 {
			line = style.Width(fl.width).MaxWidth(fl.width).Render(line)
		} else {
			line = style.Render(line)
		}

		sb.WriteString(line)
		if i < len(visible)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// fileListStatusIcon returns the icon for a git file status.
func fileListStatusIcon(s git.FileStatus) string {
	switch s {
	case git.Added:
		return tui.IconAdded
	case git.Deleted:
		return tui.IconDeleted
	case git.Renamed:
		return tui.IconRenamed
	default:
		return tui.IconModified
	}
}
