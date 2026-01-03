package git

import (
	"context"
	"fmt"
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
	// Use NUL-separated format to safely handle special chars in subjects.
	// Format: hash\x1fsubject\x1fauthor\x1freldate\x00 per commit.
	format := "--format=%h\x1f%s\x1f%an\x1f%ar\x00"
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
func CommitDiff(ctx context.Context, r Runner, hash string) (string, error) {
	out, err := r.Run(ctx, "show", hash)
	if err != nil {
		return "", fmt.Errorf("git show: %w", err)
	}
	return out, nil
}

// CreateFixupCommit creates a fixup! commit targeting the given commit hash.
func CreateFixupCommit(ctx context.Context, r Runner, hash string) error {
	_, err := r.Run(ctx, "commit", "--fixup="+hash)
	if err != nil {
		return fmt.Errorf("git commit --fixup: %w", err)
	}
	return nil
}

// parseCommitLog parses NUL-separated commit log output into CommitEntry slice.
func parseCommitLog(raw string) []CommitEntry {
	if raw == "" {
		return nil
	}

	var entries []CommitEntry
	records := strings.SplitSeq(raw, "\x00")
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
