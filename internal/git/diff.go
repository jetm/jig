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

// splitOnMarker splits raw into chunks, each starting after the marker.
// The marker prefix is stripped from each chunk.
func splitOnMarker(raw, marker string) []string {
	parts := strings.Split(raw, marker)
	// First element is everything before the first marker (usually empty).
	var chunks []string
	for _, p := range parts {
		if p == "" {
			continue
		}
		chunks = append(chunks, p)
	}
	return chunks
}

// parseOneFileDiff parses a single diff block including the "diff --git" header.
func parseOneFileDiff(block string) FileDiff {
	lines := strings.SplitN(block, "\n", 2)
	header := lines[0]

	// Extract paths from "diff --git a/old b/new"
	oldPath, newPath := extractPaths(header)

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

// extractPaths parses "diff --git a/path1 b/path2" into (path1, path2).
func extractPaths(header string) (string, string) {
	// Remove "diff --git " prefix
	rest := strings.TrimPrefix(header, "diff --git ")

	// Find the " b/" separator. The paths are "a/..." and "b/..."
	idx := strings.Index(rest, " b/")
	if idx < 0 {
		return "", ""
	}

	oldPath := strings.TrimPrefix(rest[:idx], "a/")
	newPath := strings.TrimPrefix(rest[idx+1:], "b/")
	return oldPath, newPath
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
