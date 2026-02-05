package commands

import (
	"path/filepath"
	"strings"
)

// expandGlobs expands any path that contains glob characters ('*', '?', '[')
// via filepath.Glob. Literal paths (no glob characters) are passed through as-is.
// Returns nil if the resulting slice is empty (no matches and no literals).
func expandGlobs(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}

	var result []string
	for _, p := range paths {
		if isGlob(p) {
			matches, err := filepath.Glob(p)
			if err != nil || len(matches) == 0 {
				// Glob error or no matches: skip this pattern.
				continue
			}
			result = append(result, matches...)
		} else {
			result = append(result, p)
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// isGlob reports whether path contains glob metacharacters.
func isGlob(path string) bool {
	return strings.ContainsAny(path, "*?[")
}
