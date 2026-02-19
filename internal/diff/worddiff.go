package diff

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// similarityThreshold is the minimum fraction of shared content (0.0-1.0)
// required to apply word-level emphasis to a line pair. Below this, lines
// render as plain removed/added without inline highlighting.
const similarityThreshold = 0.30

// WordDiffRenderer applies character-level diff emphasis to paired -/+ line
// blocks in unified diff output. It uses Myers diff via diffmatchpatch to
// identify changed spans and renders them with bold/reverse styling.
//
// Lines that are too dissimilar (below similarityThreshold) are left as
// plain colored diff lines without inline emphasis.
type WordDiffRenderer struct {
	removedStyle  lipgloss.Style
	addedStyle    lipgloss.Style
	emphasisStyle lipgloss.Style
}

// NewWordDiffRenderer creates a WordDiffRenderer with OneDark-inspired styles.
func NewWordDiffRenderer() *WordDiffRenderer {
	return &WordDiffRenderer{
		removedStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#e06c75")),
		addedStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#98c379")),
		emphasisStyle: lipgloss.NewStyle().Bold(true).Reverse(true),
	}
}

// Render processes a unified diff string and applies word-level emphasis to
// paired -/+ line blocks.
func (w *WordDiffRenderer) Render(rawDiff string) (string, error) {
	if rawDiff == "" {
		return "", nil
	}

	lines := strings.Split(rawDiff, "\n")
	var result []string
	i := 0

	for i < len(lines) {
		line := lines[i]

		// Collect a block of consecutive - lines.
		if isDiffRemoved(line) {
			removed := collectBlock(lines, &i, isDiffRemoved)
			added := collectBlock(lines, &i, isDiffAdded)

			rendered := w.renderPairedBlocks(removed, added)
			result = append(result, rendered...)

			continue
		}

		// A + block without preceding - block: render as plain added.
		if isDiffAdded(line) {
			added := collectBlock(lines, &i, isDiffAdded)
			for _, a := range added {
				result = append(result, w.addedStyle.Render(a))
			}

			continue
		}

		// Non-diff lines (context, headers, etc.) pass through unchanged.
		result = append(result, line)
		i++
	}

	return strings.Join(result, "\n"), nil
}

// renderPairedBlocks pairs removed and added lines positionally and applies
// character-level diff emphasis to each pair.
func (w *WordDiffRenderer) renderPairedBlocks(removed, added []string) []string {
	var result []string
	minLen := min(len(removed), len(added))

	for j := range minLen {
		rContent := stripDiffPrefix(removed[j])
		aContent := stripDiffPrefix(added[j])

		if similarity(rContent, aContent) < similarityThreshold {
			result = append(result, w.removedStyle.Render(removed[j]))
			result = append(result, w.addedStyle.Render(added[j]))

			continue
		}

		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(rContent, aContent, false)
		diffs = dmp.DiffCleanupSemanticLossless(diffs)

		result = append(result, w.renderLine("-", diffs, w.removedStyle, true))
		result = append(result, w.renderLine("+", diffs, w.addedStyle, false))
	}

	// Unpaired remainder: render as plain colored lines.
	for j := minLen; j < len(removed); j++ {
		result = append(result, w.removedStyle.Render(removed[j]))
	}

	for j := minLen; j < len(added); j++ {
		result = append(result, w.addedStyle.Render(added[j]))
	}

	return result
}

// renderLine builds a styled line from character-level diffs.
// When forRemoved is true, it renders the "old" side (DiffEqual + DiffDelete).
// When false, it renders the "new" side (DiffEqual + DiffInsert).
func (w *WordDiffRenderer) renderLine(prefix string, diffs []diffmatchpatch.Diff, baseStyle lipgloss.Style, forRemoved bool) string {
	var b strings.Builder
	b.WriteString(baseStyle.Render(prefix))

	matchOp := diffmatchpatch.DiffInsert
	if forRemoved {
		matchOp = diffmatchpatch.DiffDelete
	}

	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			b.WriteString(baseStyle.Render(d.Text))
		case matchOp:
			b.WriteString(w.emphasisStyle.Render(d.Text))
			// Skip the opposite operation (it belongs to the other line).
		}
	}

	return b.String()
}

// isDiffRemoved returns true if the line starts with "-" but not "---".
func isDiffRemoved(line string) bool {
	return strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---")
}

// isDiffAdded returns true if the line starts with "+" but not "+++".
func isDiffAdded(line string) bool {
	return strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++")
}

// collectBlock gathers consecutive lines matching pred, advancing *i.
func collectBlock(lines []string, i *int, pred func(string) bool) []string {
	var block []string

	for *i < len(lines) && pred(lines[*i]) {
		block = append(block, lines[*i])
		*i++
	}

	return block
}

// stripDiffPrefix removes the leading -/+ character from a diff line.
func stripDiffPrefix(line string) string {
	if len(line) > 0 && (line[0] == '-' || line[0] == '+') {
		return line[1:]
	}

	return line
}

// similarity computes the fraction of shared characters between two strings
// using the longer string's length as the denominator. Returns 0.0 for empty
// inputs, 1.0 for identical strings.
func similarity(a, b string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0.0
	}

	maxLen := max(len(a), len(b))

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(a, b, false)

	common := 0
	for _, d := range diffs {
		if d.Type == diffmatchpatch.DiffEqual {
			common += len(d.Text)
		}
	}

	return float64(common) / float64(maxLen)
}
