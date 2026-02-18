package git

import (
	"strings"
)

// FileStatus represents the type of change applied to a file.
type FileStatus int

const (
	// Modified indicates the file was changed.
	Modified FileStatus = iota
	// Added indicates the file is new.
	Added
	// Deleted indicates the file was removed.
	Deleted
	// Renamed indicates the file was renamed.
	Renamed
)

// FileDiff represents a single file's diff from git diff output.
type FileDiff struct {
	OldPath string
	NewPath string
	Status  FileStatus
	RawDiff string
}

// DisplayPath returns a human-readable path for the file.
// Renamed files show "old -> new"; all others show NewPath.
func (fd FileDiff) DisplayPath() string {
	if fd.Status == Renamed {
		return fd.OldPath + " -> " + fd.NewPath
	}
	return fd.NewPath
}

// ParseFileDiffs parses raw unified diff output into a slice of FileDiff.
// It splits on "diff --git" boundaries and extracts paths and status.
func ParseFileDiffs(raw string) []FileDiff {
	if raw == "" {
		return nil
	}

	// Split on "diff --git " boundary
	const marker = "diff --git "
	chunks := splitOnMarker(raw, marker)

	diffs := make([]FileDiff, 0, len(chunks))
	for _, chunk := range chunks {
		fd := parseOneFileDiff(marker + chunk)
		diffs = append(diffs, fd)
	}
	return diffs
}

// splitOnMarker splits raw into chunks on lines starting with the marker.
// The marker prefix is stripped from each chunk.
func splitOnMarker(raw, marker string) []string {
	var chunks []string
	rest := raw
	for {
		// Find marker at start of string or at start of a line
		if strings.HasPrefix(rest, marker) {
			rest = rest[len(marker):]
			chunks = append(chunks, "")
			continue
		}
		idx := strings.Index(rest, "\n"+marker)
		if idx < 0 {
			break
		}
		// idx is position of \n before the marker
		if len(chunks) == 0 {
			// Preamble before first marker: skip it
			rest = rest[idx+1:]
		} else {
			// Content between markers: assign to current chunk, advance
			chunks[len(chunks)-1] = rest[:idx+1]
			rest = rest[idx+1:]
		}
	}
	// Remaining content belongs to last chunk
	if len(chunks) > 0 {
		chunks[len(chunks)-1] = rest
	}
	return chunks
}

// parseOneFileDiff parses a single diff block including the "diff --git" header.
func parseOneFileDiff(block string) FileDiff {
	// Extract paths from ---/+++ lines (primary) or diff --git header (fallback).
	oldPath, newPath := extractPaths(block)

	fd := FileDiff{
		OldPath: oldPath,
		NewPath: newPath,
		Status:  Modified,
		RawDiff: block,
	}

	// Detect status from header lines
	switch {
	case strings.Contains(block, "\nnew file mode "):
		fd.Status = Added
	case strings.Contains(block, "\ndeleted file mode "):
		fd.Status = Deleted
	case strings.Contains(block, "\nrename from ") && strings.Contains(block, "\nrename to "):
		fd.Status = Renamed
		fd.OldPath = extractRenameField(block, "rename from ")
		fd.NewPath = extractRenameField(block, "rename to ")
	}

	return fd
}

// extractPaths extracts file paths from a diff block. It parses ---/+++ lines
// as the primary source (works with all prefix formats: standard a/b/, mnemonic
// i/w/c/o/, and noprefix). Falls back to the diff --git header if ---/+++ lines
// are not found.
func extractPaths(block string) (string, string) {
	var rawOld, rawNew string
	for line := range strings.SplitSeq(block, "\n") {
		if strings.HasPrefix(line, "--- ") && rawOld == "" {
			rawOld = line[4:]
		} else if strings.HasPrefix(line, "+++ ") && rawNew == "" {
			rawNew = line[4:]
			break
		}
	}

	if rawOld == "" && rawNew == "" {
		return extractPathsFromHeader(block)
	}

	// Handle /dev/null (new or deleted files).
	if rawOld == "/dev/null" {
		path := stripDiffPrefix(rawNew)
		return path, path
	}
	if rawNew == "/dev/null" {
		path := stripDiffPrefix(rawOld)
		return path, path
	}

	// Both paths present: if they have different single-letter prefixes, strip them.
	if len(rawOld) > 2 && len(rawNew) > 2 &&
		rawOld[1] == '/' && rawNew[1] == '/' &&
		rawOld[0] != rawNew[0] {
		return rawOld[2:], rawNew[2:]
	}

	// No prefix (noprefix mode) or same prefix letter - use paths as-is.
	return rawOld, rawNew
}

// stripDiffPrefix removes a single-letter git diff prefix (a/, b/, i/, w/, c/, o/)
// from a path. Used for /dev/null cases where only one path is available.
func stripDiffPrefix(path string) string {
	if len(path) > 2 && path[1] == '/' {
		return path[2:]
	}
	return path
}

// extractPathsFromHeader parses paths from the "diff --git X/path1 Y/path2" header.
// This is the fallback for diff blocks without ---/+++ lines (e.g., empty or binary files).
// Handles standard (a/b), mnemonic (c/i/w/o), and noprefix formats.
func extractPathsFromHeader(block string) (string, string) {
	header := strings.SplitN(block, "\n", 2)[0]
	rest := strings.TrimPrefix(header, "diff --git ")

	// Find the separator between old and new paths: " X/" where X is a single letter
	// different from the first path's prefix. Both paths use single-letter prefixes
	// (a/b for standard, c/i for mnemonic, w/o for others).
	if len(rest) < 4 || rest[1] != '/' {
		return "", ""
	}
	oldPrefix := rest[0]

	// Search for " Y/" where Y != oldPrefix
	for i := 2; i < len(rest)-2; i++ {
		if rest[i] == ' ' && rest[i+2] == '/' && rest[i+1] != oldPrefix {
			return rest[2:i], rest[i+3:]
		}
	}
	return "", ""
}

// extractRenameField extracts the value from a "rename from X" or "rename to X" line.
func extractRenameField(block, prefix string) string {
	idx := strings.Index(block, "\n"+prefix)
	if idx < 0 {
		return ""
	}
	start := idx + 1 + len(prefix)
	end := strings.Index(block[start:], "\n")
	if end < 0 {
		return strings.TrimSpace(block[start:])
	}
	return strings.TrimSpace(block[start : start+end])
}

// DiffArgs constructs the git diff argument list from revision and staged flag.
func DiffArgs(revision string, staged bool) []string {
	args := []string{"diff"}
	if staged {
		args = append(args, "--cached")
	}
	if revision != "" {
		args = append(args, revision)
	}
	return args
}
