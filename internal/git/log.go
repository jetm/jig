package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// CommitEntry represents a single commit from git log output.
type CommitEntry struct {
	Hash    string // short hash (7 chars)
	Subject string // first line of the commit message
	Author  string // author name
	Date    string // relative date (e.g. "2 hours ago")
}

// RecentCommits returns up to n recent commits from git log.
// Each entry includes hash, subject, author name, and relative date.
func RecentCommits(ctx context.Context, r Runner, n int) ([]CommitEntry, error) {
	return RecentCommitsFrom(ctx, r, n, "")
}

// RecentCommitsFrom returns up to n recent commits from git log, starting from
// the given revision ref (e.g. "HEAD", "main", "v1.0"). An empty ref defaults
// to HEAD (standard git log behaviour).
func RecentCommitsFrom(ctx context.Context, r Runner, n int, ref string) ([]CommitEntry, error) {
	// Use RS-separated format to safely handle special chars in subjects.
	// Format: hash\x1fsubject\x1fauthor\x1freldate\x1e per commit.
	format := "--format=%h\x1f%s\x1f%an\x1f%ar\x1e"
	args := []string{"log", fmt.Sprintf("-n%d", n), format}
	if ref != "" {
		args = append(args, ref)
	}
	out, err := r.Run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	return parseCommitLog(out), nil
}

// CommitDiff returns the diff of a single commit (git show <hash>).
// When contextLines is negative the git default is used.
// The commit metadata header (commit hash, Author, Date, message body) that
// git show prepends to the patch is stripped; only the patch text is returned.
func CommitDiff(ctx context.Context, r Runner, hash string, contextLines int) (string, error) {
	args := []string{"show"}
	if contextLines >= 0 {
		args = append(args, fmt.Sprintf("-U%d", contextLines))
	}
	args = append(args, hash)
	out, err := r.Run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("git show: %w", err)
	}
	return stripCommitHeader(out), nil
}

// stripCommitHeader removes the commit metadata block that git show prepends to
// the patch (commit hash, Author, Date, and message body). Returns everything
// from the first "diff --git" line onwards. When no diff header is found (e.g.
// empty commits), the original string is returned unchanged.
func stripCommitHeader(s string) string {
	if idx := strings.Index(s, "\ndiff --git "); idx >= 0 {
		return s[idx+1:]
	}
	if strings.HasPrefix(s, "diff --git ") {
		return s
	}
	return s
}

// LargeFileLineThreshold is the default maximum number of added+deleted lines
// in a single file before CommitDiffSafe excludes it from the diff.
const LargeFileLineThreshold = 20_000

// FileStat holds per-file statistics from git --numstat output.
type FileStat struct {
	Additions int
	Deletions int
	Name      string
	Binary    bool
}

// CommitNumstat returns per-file line-count statistics for a commit using
// git show --numstat. The commit metadata header is suppressed with --format=.
func CommitNumstat(ctx context.Context, r Runner, hash string) ([]FileStat, error) {
	out, err := r.Run(ctx, "show", "--numstat", "--format=", hash)
	if err != nil {
		return nil, fmt.Errorf("git show --numstat: %w", err)
	}
	return parseNumstat(out), nil
}

// parseNumstat parses git --numstat output into a FileStat slice.
// Each line is "<additions>\t<deletions>\t<filename>" or "-\t-\t<filename>" for binaries.
func parseNumstat(raw string) []FileStat {
	var stats []FileStat
	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		stat := FileStat{Name: parts[2]}
		if parts[0] == "-" && parts[1] == "-" {
			stat.Binary = true
		} else {
			stat.Additions, _ = strconv.Atoi(parts[0])
			stat.Deletions, _ = strconv.Atoi(parts[1])
		}
		stats = append(stats, stat)
	}
	return stats
}

// CommitDiffSafe returns the diff for a commit, excluding any file whose total
// changed lines (additions + deletions) exceeds maxLinesPerFile. Excluded files
// are listed in a human-readable notice prepended to the patch. When no files
// exceed the threshold the behaviour is identical to CommitDiff.
func CommitDiffSafe(ctx context.Context, r Runner, hash string, contextLines, maxLinesPerFile int) (string, error) {
	stats, err := CommitNumstat(ctx, r, hash)
	if err != nil {
		return CommitDiff(ctx, r, hash, contextLines)
	}

	var large []string
	for _, s := range stats {
		if !s.Binary && s.Additions+s.Deletions > maxLinesPerFile {
			large = append(large, s.Name)
		}
	}

	if len(large) == 0 {
		return CommitDiff(ctx, r, hash, contextLines)
	}

	// Build diff call with :(exclude) pathspecs for large files.
	args := []string{"show"}
	if contextLines >= 0 {
		args = append(args, fmt.Sprintf("-U%d", contextLines))
	}
	args = append(args, hash, "--")
	for _, name := range large {
		args = append(args, ":(exclude)"+name)
	}

	out, err := r.Run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("git show: %w", err)
	}

	diff := stripCommitHeader(out)

	// Prepend a notice listing the excluded files.
	var notice strings.Builder
	notice.WriteString("# Large files excluded from diff (>" + strconv.Itoa(maxLinesPerFile) + " changed lines):\n")
	for _, name := range large {
		notice.WriteString("#   " + name + "\n")
	}
	notice.WriteString("#\n")
	return notice.String() + diff, nil
}

// CreateFixupCommit creates a fixup! commit targeting the given commit hash.
func CreateFixupCommit(ctx context.Context, r Runner, hash string) error {
	_, err := r.Run(ctx, "commit", "--fixup="+hash)
	if err != nil {
		return fmt.Errorf("git commit --fixup: %w", err)
	}
	return nil
}

// AutosquashRebase runs a non-interactive autosquash rebase to squash fixup!
// commits into their targets. It uses GIT_SEQUENCE_EDITOR=true so git applies
// the reorder without prompting. For root commits (no parent), it retries with
// --root when the <hash>^ reference fails.
func AutosquashRebase(ctx context.Context, r Runner, hash string) error {
	env := []string{"GIT_SEQUENCE_EDITOR=true"}
	_, err := r.RunWithEnv(ctx, env, "rebase", "--interactive", "--autosquash", hash+"^")
	if err == nil {
		return nil
	}
	// If the first attempt failed because the parent ref doesn't exist (exit
	// code 128 = invalid object), retry with --root to handle the case where
	// hash is the initial commit (no parent). Other errors (e.g. conflicts,
	// exit code 1) are returned directly.
	if execErr, ok := err.(*ExecError); ok && execErr.ExitCode == 128 {
		_, retryErr := r.RunWithEnv(ctx, env, "rebase", "--interactive", "--autosquash", "--root")
		if retryErr == nil {
			return nil
		}
		return fmt.Errorf("git rebase --autosquash: %w", retryErr)
	}
	return fmt.Errorf("git rebase --autosquash: %w", err)
}

// parseCommitLog parses NUL-separated commit log output into CommitEntry slice.
func parseCommitLog(raw string) []CommitEntry {
	if raw == "" {
		return nil
	}

	var entries []CommitEntry
	records := strings.SplitSeq(raw, "\x1e")
	for rec := range records {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		fields := strings.SplitN(rec, "\x1f", 4)
		if len(fields) != 4 {
			continue
		}
		entries = append(entries, CommitEntry{
			Hash:    strings.TrimSpace(fields[0]),
			Subject: strings.TrimSpace(fields[1]),
			Author:  strings.TrimSpace(fields[2]),
			Date:    strings.TrimSpace(fields[3]),
		})
	}
	return entries
}
