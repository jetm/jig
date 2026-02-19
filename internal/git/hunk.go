package git

import (
	"fmt"
	"strconv"
	"strings"
)

// Line represents a single line within a diff hunk.
type Line struct {
	Op       rune   // '+', '-', or ' ' (context)
	Content  string // line content without the leading op character
	Selected bool   // whether this line is selected for staging
}

// HunkRange represents a line range from a unified diff header.
type HunkRange struct {
	Start int
	Count int
}

// Hunk represents a single diff hunk within a file diff.
type Hunk struct {
	// Header is the "@@ ... @@" line.
	Header string
	// Lines holds the parsed line-level representation of the hunk body.
	Lines []Line
}

// Body reconstructs the full hunk text (header + body lines) from Lines.
// This method replaces the former Body field to ensure the output always
// reflects the canonical Lines data.
func (h Hunk) Body() string {
	if len(h.Lines) == 0 {
		return h.Header
	}
	var b strings.Builder
	b.WriteString(h.Header)
	for _, l := range h.Lines {
		b.WriteByte('\n')
		b.WriteRune(l.Op)
		b.WriteString(l.Content)
	}
	return b.String()
}

// ParseHunks parses the raw diff of a single file into individual hunks.
// Each hunk starts with a "@@ " line. Returns nil if rawDiff has no hunks.
func ParseHunks(rawDiff string) []Hunk {
	lines := strings.Split(rawDiff, "\n")

	var hunks []Hunk
	var current []string
	inHunk := false

	for _, line := range lines {
		if strings.HasPrefix(line, "@@ ") {
			// Save previous hunk if any
			if inHunk && len(current) > 0 {
				hunks = append(hunks, buildHunk(current))
			}
			current = []string{line}
			inHunk = true
		} else if inHunk {
			current = append(current, line)
		}
		// Lines before the first @@ (file headers) are skipped
	}

	if inHunk && len(current) > 0 {
		hunks = append(hunks, buildHunk(current))
	}

	return hunks
}

// buildHunk assembles a Hunk from a slice of lines starting with the @@ header.
func buildHunk(lines []string) Hunk {
	// Trim trailing empty lines that result from splitting on \n
	for len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	header := lines[0]
	var parsed []Line
	for _, line := range lines[1:] {
		parsed = append(parsed, ParseLine(line))
	}

	return Hunk{
		Header: header,
		Lines:  parsed,
	}
}

// ParseLine converts a raw diff line into a Line struct.
func ParseLine(raw string) Line {
	if len(raw) == 0 {
		// Empty line in diff = context line with no content
		return Line{Op: ' ', Content: "", Selected: false}
	}
	op := rune(raw[0])
	switch op {
	case '+', '-':
		return Line{Op: op, Content: raw[1:], Selected: true}
	case ' ':
		return Line{Op: ' ', Content: raw[1:], Selected: false}
	case '\\':
		// "\ No newline at end of file" - treat as context
		return Line{Op: '\\', Content: raw[1:], Selected: false}
	default:
		// Unknown prefix - treat as context
		return Line{Op: ' ', Content: raw, Selected: false}
	}
}

// ParseHunkRange extracts a HunkRange from a range string like "10,7" or "10".
func ParseHunkRange(s string) HunkRange {
	parts := strings.SplitN(s, ",", 2)
	start, _ := strconv.Atoi(parts[0])
	count := 1
	if len(parts) == 2 {
		count, _ = strconv.Atoi(parts[1])
	}
	return HunkRange{Start: start, Count: count}
}

// parseHunkHeader extracts old and new ranges from a @@ header line.
// For "@@ -10,7 +10,9 @@ context", it returns HunkRange{10,7} and HunkRange{10,9}.
func parseHunkHeader(header string) (oldRange, newRange HunkRange) {
	// Extract the part between @@ and @@
	trimmed := strings.TrimPrefix(header, "@@ ")
	before, _, ok := strings.Cut(trimmed, " @@")
	if !ok {
		return
	}
	ranges := before
	parts := strings.Fields(ranges)
	if len(parts) < 2 {
		return
	}
	oldRange = ParseHunkRange(strings.TrimPrefix(parts[0], "-"))
	newRange = ParseHunkRange(strings.TrimPrefix(parts[1], "+"))
	return
}

// RecalculateHeader computes the correct @@ -a,b +c,d @@ header from
// the hunk's line selections, given the original old-side start line.
// Context lines and selected - lines count toward old side.
// Context lines and selected + lines count toward new side.
// Deselected + lines are omitted entirely.
// Deselected - lines become context lines.
func RecalculateHeader(h Hunk) string {
	oldRange, _ := parseHunkHeader(h.Header)

	oldCount := 0
	newCount := 0
	for _, l := range h.Lines {
		switch {
		case l.Op == ' ' || l.Op == '\\':
			oldCount++
			newCount++
		case l.Op == '-' && l.Selected:
			oldCount++
		case l.Op == '-' && !l.Selected:
			// Deselected removal = context line
			oldCount++
			newCount++
		case l.Op == '+' && l.Selected:
			newCount++
		case l.Op == '+' && !l.Selected:
			// Deselected addition = omitted entirely
		}
	}

	// Preserve any trailing context text after the second @@
	suffix := ""
	trimmed := strings.TrimPrefix(h.Header, "@@ ")
	_, after, ok := strings.Cut(trimmed, " @@")
	if ok {
		after := after
		if after != "" {
			suffix = " " + strings.TrimLeft(after, " ")
		}
	}

	return fmt.Sprintf("@@ -%d,%d +%d,%d @@%s", oldRange.Start, oldCount, oldRange.Start, newCount, suffix)
}

// BuildPatch generates a valid unified diff patch from the selected lines
// in a hunk. The patchHeader should be the "diff --git..." file header.
// Returns empty string if no lines are selected (hunk should be skipped).
func BuildPatch(patchHeader string, h Hunk) string {
	// Check if any changed lines are selected
	hasSelected := false
	for _, l := range h.Lines {
		if (l.Op == '+' || l.Op == '-') && l.Selected {
			hasSelected = true
			break
		}
	}
	if !hasSelected {
		return ""
	}

	header := RecalculateHeader(h)

	var b strings.Builder
	b.WriteString(patchHeader)
	b.WriteByte('\n')
	b.WriteString(header)

	for _, l := range h.Lines {
		switch {
		case l.Op == ' ' || l.Op == '\\':
			b.WriteByte('\n')
			b.WriteRune(l.Op)
			b.WriteString(l.Content)
		case l.Op == '-' && l.Selected:
			b.WriteByte('\n')
			b.WriteRune(l.Op)
			b.WriteString(l.Content)
		case l.Op == '-' && !l.Selected:
			// Deselected removal -> context line
			b.WriteByte('\n')
			b.WriteByte(' ')
			b.WriteString(l.Content)
		case l.Op == '+' && l.Selected:
			b.WriteByte('\n')
			b.WriteRune(l.Op)
			b.WriteString(l.Content)
		case l.Op == '+' && !l.Selected:
			// Deselected addition -> omit entirely
		}
	}
	b.WriteByte('\n')

	return b.String()
}
