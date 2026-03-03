package diff

import (
	"regexp"
	"strings"
)

// LineType classifies a line in a unified diff.
type LineType int

// LineType constants classify lines in a unified diff.
const (
	LineHeader     LineType = iota // diff --git, index, ---, +++
	LineHunkHeader                 // @@ ... @@
	LineContext                    // unchanged (space prefix)
	LineAdded                      // + prefix
	LineRemoved                    // - prefix
)

// LineInfo holds the source file line number and type for a single diff output line.
type LineInfo struct {
	Num      int      // source line number (0 = no number, blank gutter)
	LineType LineType // classification of this diff line
}

var hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// ParseLineNumbers parses raw unified diff text and returns a LineInfo
// for each line, mapping diff-output indices to source file line numbers.
func ParseLineNumbers(raw string) []LineInfo {
	if raw == "" {
		return nil
	}

	lines := strings.Split(raw, "\n")
	result := make([]LineInfo, len(lines))

	var oldLine, newLine int
	inHunk := false

	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "@@"):
			m := hunkHeaderRe.FindStringSubmatch(line)
			if m != nil {
				oldLine = atoi(m[1])
				newLine = atoi(m[2])
				inHunk = true
			}
			result[i] = LineInfo{LineType: LineHunkHeader}

		case strings.HasPrefix(line, "diff "),
			strings.HasPrefix(line, "index "),
			strings.HasPrefix(line, "--- "),
			strings.HasPrefix(line, "+++ "):
			inHunk = false
			result[i] = LineInfo{LineType: LineHeader}

		case inHunk && strings.HasPrefix(line, "+"):
			result[i] = LineInfo{Num: newLine, LineType: LineAdded}
			newLine++

		case inHunk && strings.HasPrefix(line, "-"):
			result[i] = LineInfo{Num: oldLine, LineType: LineRemoved}
			oldLine++

		case inHunk:
			// Context line (space prefix or empty within hunk)
			result[i] = LineInfo{Num: newLine, LineType: LineContext}
			oldLine++
			newLine++

		default:
			result[i] = LineInfo{LineType: LineHeader}
		}
	}

	return result
}

// atoi converts a string to int without error handling (caller ensures valid input from regex).
func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}
