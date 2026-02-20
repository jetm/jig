package git

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// BlameResult holds the blame-based fixup target detection result.
type BlameResult struct {
	// Hash is the short commit hash of the suggested fixup target.
	Hash string
	// Confidence is the percentage of blamed lines pointing to this commit.
	Confidence int
}

// ParseBlameOutput parses `git blame --porcelain` output into a map of
// commit hash to total number of blamed lines.
func ParseBlameOutput(output string) map[string]int {
	counts := make(map[string]int)
	for line := range strings.SplitSeq(output, "\n") {
		// Porcelain format: lines starting with a 40-char hex hash followed by
		// space and line numbers are blame entries.
		if len(line) < 41 || line[40] != ' ' {
			continue
		}
		hash := line[:40]
		// Validate it looks like a hex hash
		if !isHex(hash) {
			continue
		}
		// Skip the boundary/uncommitted marker (all zeros)
		if hash == strings.Repeat("0", 40) {
			continue
		}
		counts[hash]++
	}
	return counts
}

// FindFixupTarget analyzes the staged diff via git blame to suggest a fixup
// target commit. It runs blame for each file's modified line ranges and scores
// commits by total blamed lines. Returns the top commit (abbreviated) and
// confidence percentage. Returns empty result on error or when no target is found.
func FindFixupTarget(ctx context.Context, r Runner, stagedDiff string) BlameResult {
	files := ParseFileDiffs(stagedDiff)
	if len(files) == 0 {
		return BlameResult{}
	}

	// Use a timeout context to avoid hanging on blame for large diffs.
	blameCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	totalCounts := make(map[string]int)
	totalLines := 0

	for _, f := range files {
		// Skip new files (no previous history to blame)
		if f.Status == Added {
			continue
		}

		hunks := ParseHunks(f.RawDiff)
		for _, h := range hunks {
			oldRange, _ := parseHunkHeader(h.Header)
			if oldRange.Start == 0 || oldRange.Count == 0 {
				continue
			}

			end := oldRange.Start + oldRange.Count - 1
			lineSpec := fmt.Sprintf("-L%d,%d", oldRange.Start, end)

			// Use the old path for blame since we're looking at what was changed
			out, err := r.Run(blameCtx, "blame", "--porcelain", lineSpec, f.OldPath, "HEAD")
			if err != nil {
				// Blame can fail for many reasons (new file, binary, etc.) - skip
				continue
			}

			counts := ParseBlameOutput(out)
			for hash, n := range counts {
				totalCounts[hash] += n
				totalLines += n
			}
		}
	}

	if totalLines == 0 {
		return BlameResult{}
	}

	// Find the commit with the highest score
	var topHash string
	topCount := 0
	for hash, n := range totalCounts {
		if n > topCount {
			topHash = hash
			topCount = n
		}
	}

	if topHash == "" {
		return BlameResult{}
	}

	confidence := (topCount * 100) / totalLines

	// Abbreviate the hash
	short, err := r.Run(ctx, "rev-parse", "--short", topHash)
	if err != nil {
		// Fallback: use first 7 chars
		if len(topHash) > 7 {
			short = topHash[:7]
		} else {
			short = topHash
		}
	}
	short = strings.TrimSpace(short)

	return BlameResult{
		Hash:       short,
		Confidence: confidence,
	}
}

// isHex reports whether s consists entirely of hex digits.
func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}
