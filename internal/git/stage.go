package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StatusFile represents a file with a working-tree status.
type StatusFile struct {
	Path   string
	Status FileStatus
}

// ListUnstagedFilesFiltered returns unstaged files optionally filtered by paths.
// When paths is empty it returns all unstaged and untracked files.
// When paths is non-empty it appends "-- <paths>" to the git diff calls so only
// the specified files are included.
func ListUnstagedFilesFiltered(ctx context.Context, r Runner, paths []string) ([]StatusFile, error) {
	args := []string{"diff", "--name-status"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	nameStatus, err := r.Run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("git diff --name-status: %w", err)
	}

	var files []StatusFile
	for line := range strings.SplitSeq(nameStatus, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sf := parseNameStatusLine(line)
		files = append(files, sf)
	}

	// Untracked files (only when no path filter; path-filtered untracked is complex)
	var untrackedFiles []StatusFile
	if len(paths) == 0 {
		untracked, err := r.Run(ctx, "ls-files", "--others", "--exclude-standard")
		if err != nil {
			return nil, fmt.Errorf("git ls-files --others: %w", err)
		}
		for line := range strings.SplitSeq(untracked, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			untrackedFiles = append(untrackedFiles, StatusFile{Path: line, Status: Added})
		}
	} else {
		// For path-filtered mode, check each path individually as untracked.
		untrackedArgs := append([]string{"ls-files", "--others", "--exclude-standard", "--"}, paths...)
		untracked, err := r.Run(ctx, untrackedArgs...)
		if err == nil {
			for line := range strings.SplitSeq(untracked, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				untrackedFiles = append(untrackedFiles, StatusFile{Path: line, Status: Added})
			}
		}
	}

	// Filter out untracked files whose paths traverse a symlink.
	// git ls-files --others walks through symlinks, but git add refuses to
	// stage paths beyond a symlink (exit 128).
	if len(untrackedFiles) > 0 {
		topLevel, err := r.Run(ctx, "rev-parse", "--show-toplevel")
		if err == nil {
			untrackedFiles = filterBeyondSymlinks(topLevel, untrackedFiles)
		}
	}
	files = append(files, untrackedFiles...)

	return files, nil
}

// ListModifiedFilesFiltered returns working-tree modified files filtered by paths.
// When paths is empty it returns all modified tracked files.
func ListModifiedFilesFiltered(ctx context.Context, r Runner, paths []string) ([]StatusFile, error) {
	args := []string{"diff", "--name-status"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	nameStatus, err := r.Run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("git diff --name-status: %w", err)
	}

	var files []StatusFile
	for line := range strings.SplitSeq(nameStatus, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sf := parseNameStatusLine(line)
		files = append(files, sf)
	}

	return files, nil
}

// ListStagedFilesFiltered returns staged files optionally filtered by paths.
// When paths is empty it returns all staged files.
func ListStagedFilesFiltered(ctx context.Context, r Runner, paths []string) ([]StatusFile, error) {
	args := []string{"diff", "--cached", "--name-status"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	nameStatus, err := r.Run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("git diff --cached --name-status: %w", err)
	}

	var files []StatusFile
	for line := range strings.SplitSeq(nameStatus, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sf := parseNameStatusLine(line)
		files = append(files, sf)
	}

	return files, nil
}

// parseNameStatusLine parses a single line from git diff --name-status.
// Format: <status>\t<path> or R<score>\t<old>\t<new> for renames.
func parseNameStatusLine(line string) StatusFile {
	parts := strings.Split(line, "\t")
	if len(parts) < 2 {
		return StatusFile{Path: line, Status: Modified}
	}

	code := parts[0]
	switch {
	case strings.HasPrefix(code, "R"):
		// Rename: R100\told\tnew
		newPath := parts[len(parts)-1]
		return StatusFile{Path: newPath, Status: Renamed}
	case code == "A":
		return StatusFile{Path: parts[1], Status: Added}
	case code == "D":
		return StatusFile{Path: parts[1], Status: Deleted}
	default:
		return StatusFile{Path: parts[1], Status: Modified}
	}
}

// StageFiles runs git add for the given file paths.
func StageFiles(ctx context.Context, r Runner, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"add", "--"}, paths...)
	_, err := r.Run(ctx, args...)
	if err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	return nil
}

// StageHunk stages a single hunk by feeding a unified diff patch to git apply --cached.
func StageHunk(ctx context.Context, r Runner, patch string) error {
	_, err := r.RunWithStdin(ctx, patch, "apply", "--cached")
	if err != nil {
		return fmt.Errorf("git apply --cached: %w", err)
	}
	return nil
}

// UnstageFiles runs git reset HEAD -- for the given file paths to remove them from the index.
func UnstageFiles(ctx context.Context, r Runner, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"reset", "HEAD", "--"}, paths...)
	_, err := r.Run(ctx, args...)
	if err != nil {
		return fmt.Errorf("git reset HEAD: %w", err)
	}
	return nil
}

// UnstageHunk removes a single hunk from the index via `git apply --cached --reverse`.
// patch is the full unified diff patch (header + hunk body) to reverse-apply.
func UnstageHunk(ctx context.Context, r Runner, patch string) error {
	_, err := r.RunWithStdin(ctx, patch, "apply", "--cached", "--reverse")
	if err != nil {
		return fmt.Errorf("git apply --cached --reverse: %w", err)
	}
	return nil
}

// DiscardHunk reverts a single hunk in the working tree via `git apply --reverse`.
// patch is the full unified diff patch (header + hunk body) to reverse-apply.
func DiscardHunk(ctx context.Context, r Runner, patch string) error {
	_, err := r.RunWithStdin(ctx, patch, "apply", "--reverse")
	if err != nil {
		return fmt.Errorf("git apply --reverse: %w", err)
	}
	return nil
}

// DiscardFiles runs git checkout -- for the given file paths to restore them to HEAD.
func DiscardFiles(ctx context.Context, r Runner, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"checkout", "--"}, paths...)
	_, err := r.Run(ctx, args...)
	if err != nil {
		return fmt.Errorf("git checkout --: %w", err)
	}
	return nil
}

// pathBeyondSymlink reports whether any directory component of relPath
// is a symbolic link when resolved from baseDir.
func pathBeyondSymlink(baseDir, relPath string) bool {
	dir := filepath.Dir(relPath)
	if dir == "." {
		return false
	}
	current := baseDir
	for part := range strings.SplitSeq(dir, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		fi, err := os.Lstat(current)
		if err != nil {
			return false
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return true
		}
	}
	return false
}

// filterBeyondSymlinks removes files whose paths traverse a symlink.
func filterBeyondSymlinks(baseDir string, files []StatusFile) []StatusFile {
	filtered := make([]StatusFile, 0, len(files))
	for _, f := range files {
		if !pathBeyondSymlink(baseDir, f.Path) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}
