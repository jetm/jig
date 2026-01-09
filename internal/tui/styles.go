package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// OneDark hex values.
const (
	HexBg       = "#282c34"
	HexBgAlt    = "#2c313c"
	HexBgSel    = "#3e4451"
	HexBgFloat  = "#21252b"
	HexFg       = "#abb2bf"
	HexFgSubtle = "#5c6370"
	HexFgEmph   = "#ffffff"
	HexRed      = "#e06c75"
	HexOrange   = "#d19a66"
	HexYellow   = "#e5c07b"
	HexGreen    = "#98c379"
	HexCyan     = "#56b6c2"
	HexBlue     = "#61afef"
	HexPurple   = "#c678dd"
)

// OneDark color palette as color.Color values for lipgloss styles.
var (
	ColorBg       = lipgloss.Color(HexBg)
	ColorBgAlt    = lipgloss.Color(HexBgAlt)
	ColorBgSel    = lipgloss.Color(HexBgSel)
	ColorBgFloat  = lipgloss.Color(HexBgFloat)
	ColorFg       = lipgloss.Color(HexFg)
	ColorFgSubtle = lipgloss.Color(HexFgSubtle)
	ColorFgEmph   = lipgloss.Color(HexFgEmph)
	ColorRed      = lipgloss.Color(HexRed)
	ColorOrange   = lipgloss.Color(HexOrange)
	ColorYellow   = lipgloss.Color(HexYellow)
	ColorGreen    = lipgloss.Color(HexGreen)
	ColorCyan     = lipgloss.Color(HexCyan)
	ColorBlue     = lipgloss.Color(HexBlue)
	ColorPurple   = lipgloss.Color(HexPurple)
)

// Nerd Font icons.
const (
	IconModified  = "\uf040" // nf-fa-pencil
	IconAdded     = "\uf067" // nf-fa-plus
	IconDeleted   = "\uf068" // nf-fa-minus
	IconRenamed   = "\uf553" // nf-mdi-rename_box
	IconUntracked = "\uf128" // nf-fa-question
	IconBranch    = "\ue725" // nf-dev-git_branch
	IconCommit    = "\uf417" // nf-oct-git_commit
	IconChecked   = "\uf046" // nf-fa-check_square_o
	IconUnchecked = "\uf096" // nf-fa-square_o
	IconWarning   = "\uf071" // nf-fa-exclamation_triangle
	IconError     = "\uf00d" // nf-fa-times
	IconSuccess   = "\uf00c" // nf-fa-check
	IconDiff      = "\uf440" // nf-oct-diff
	IconFilter    = "\uf0b0" // nf-fa-filter
	IconPick      = "\uf00c" // nf-fa-check
	IconReword    = "\uf040" // nf-fa-pencil
	IconEdit      = "\uf044" // nf-fa-pencil_square_o
	IconSquash    = "\uf066" // nf-fa-compress
	IconFixup     = "\uf0e2" // nf-fa-undo
	IconDrop      = "\uf1f8" // nf-fa-trash
)

// Pre-built typography styles.
var (
	StyleBold          = lipgloss.NewStyle().Bold(true)
	StyleItalic        = lipgloss.NewStyle().Italic(true)
	StyleStrikethrough = lipgloss.NewStyle().Strikethrough(true)
)

// Focus indicator styles for two-panel layouts.
var (
	StyleFocusBorder = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderLeft(true).
				BorderForeground(ColorBlue)

	StyleDimBorder = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderForeground(ColorBgAlt)
)

// compile-time check that color vars are color.Color
var _ color.Color = ColorBg
