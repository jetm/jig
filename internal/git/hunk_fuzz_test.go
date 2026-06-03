package git

import (
	"strings"
	"testing"
)

func FuzzParseHunks(f *testing.F) {
	f.Add("@@ -1,2 +1,3 @@\n-old\n+new\n context\n")
	f.Add("@@ -1 +1 @@\n-x\n+y\n")
	f.Add("")
	f.Add("not a hunk at all")
	f.Add("@@ -0,0 +1 @@\n+added\n")
	f.Add("@@ -10,5 +10,4 @@ func main() {\n context\n-removed\n context\n context\n context\n")

	f.Fuzz(func(t *testing.T, s string) {
		hunks := ParseHunks(s)
		for _, h := range hunks {
			if !strings.HasPrefix(h.Header, "@@ ") {
				t.Errorf("hunk header must start with '@@ ', got %q", h.Header)
			}
			// Round-trip: Body must contain the header.
			body := h.Body()
			if !strings.Contains(body, h.Header) {
				t.Errorf("Body() must contain Header %q", h.Header)
			}
		}
	})
}

func FuzzBuildPatch(f *testing.F) {
	f.Add("diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go", "@@ -1,2 +1,2 @@\n-old line\n+new line\n context\n")
	f.Add("diff --git a/foo b/foo", "@@ -1 +1 @@\n+added\n")
	f.Add("", "")

	f.Fuzz(func(t *testing.T, header, rawHunk string) {
		hunks := ParseHunks(rawHunk)
		for _, h := range hunks {
			// Select all changed lines so BuildPatch has something to emit.
			for i := range h.Lines {
				if h.Lines[i].Op == '+' || h.Lines[i].Op == '-' {
					h.Lines[i].Selected = true
				}
			}
			patch := BuildPatch(header, h)
			// If a patch was produced it must start with the header.
			if patch != "" && !strings.HasPrefix(patch, header) {
				t.Errorf("BuildPatch output must start with patchHeader %q", header)
			}
		}
	})
}

func FuzzParseHunkHeader(f *testing.F) {
	f.Add("@@ -1,3 +1,4 @@ func main() {")
	f.Add("@@ -0,0 +1 @@")
	f.Add("@@ -10,5 +10,5 @@")
	f.Add("")
	f.Add("@@ -1 +1 @@")

	f.Fuzz(func(t *testing.T, s string) {
		old, newRange := parseHunkHeader(s)
		// Counts must never be negative.
		if old.Count < 0 {
			t.Errorf("parseHunkHeader returned negative old count %d for %q", old.Count, s)
		}
		if newRange.Count < 0 {
			t.Errorf("parseHunkHeader returned negative new count %d for %q", newRange.Count, s)
		}
	})
}
