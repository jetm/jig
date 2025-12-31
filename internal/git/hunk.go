package git

import (
	"strings"
)

// Hunk represents a single diff hunk within a file diff.
type Hunk struct {
	// Header is the "@@ ... @@" line.
	Header string
	// Body is the full hunk text including the header line.
	Body string
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
	body := strings.Join(lines, "\n")
	return Hunk{
		Header: lines[0],
		Body:   body,
	}
}
