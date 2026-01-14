package git

import (
	"testing"
)

func TestParseFileDiffs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantLen  int
		wantDiff []struct {
			oldPath string
			newPath string
			status  FileStatus
		}
	}{
		{
			name:    "empty input",
			input:   "",
			wantLen: 0,
		},
		{
			name: "single modified file",
			input: "diff --git a/foo.go b/foo.go\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- a/foo.go\n" +
				"+++ b/foo.go\n" +
				"@@ -1,3 +1,4 @@\n" +
				" line1\n" +
				"+added\n" +
				" line2\n",
			wantLen: 1,
			wantDiff: []struct {
				oldPath string
				newPath string
				status  FileStatus
			}{
				{oldPath: "foo.go", newPath: "foo.go", status: Modified},
			},
		},
		{
			name: "multiple files",
			input: "diff --git a/a.go b/a.go\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- a/a.go\n" +
				"+++ b/a.go\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n" +
				"diff --git a/b.go b/b.go\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- a/b.go\n" +
				"+++ b/b.go\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n" +
				"diff --git a/c.go b/c.go\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- a/c.go\n" +
				"+++ b/c.go\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n",
			wantLen: 3,
			wantDiff: []struct {
				oldPath string
				newPath string
				status  FileStatus
			}{
				{oldPath: "a.go", newPath: "a.go", status: Modified},
				{oldPath: "b.go", newPath: "b.go", status: Modified},
				{oldPath: "c.go", newPath: "c.go", status: Modified},
			},
		},
		{
			name: "new file",
			input: "diff --git a/new.go b/new.go\n" +
				"new file mode 100644\n" +
				"index 0000000..abcdefg\n" +
				"--- /dev/null\n" +
				"+++ b/new.go\n" +
				"@@ -0,0 +1,3 @@\n" +
				"+package main\n",
			wantLen: 1,
			wantDiff: []struct {
				oldPath string
				newPath string
				status  FileStatus
			}{
				{oldPath: "new.go", newPath: "new.go", status: Added},
			},
		},
		{
			name: "deleted file",
			input: "diff --git a/old.go b/old.go\n" +
				"deleted file mode 100644\n" +
				"index abcdefg..0000000\n" +
				"--- a/old.go\n" +
				"+++ /dev/null\n" +
				"@@ -1,3 +0,0 @@\n" +
				"-package main\n",
			wantLen: 1,
			wantDiff: []struct {
				oldPath string
				newPath string
				status  FileStatus
			}{
				{oldPath: "old.go", newPath: "old.go", status: Deleted},
			},
		},
		{
			name: "renamed file",
			input: "diff --git a/old.go b/new.go\n" +
				"similarity index 95%\n" +
				"rename from old.go\n" +
				"rename to new.go\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- a/old.go\n" +
				"+++ b/new.go\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n",
			wantLen: 1,
			wantDiff: []struct {
				oldPath string
				newPath string
				status  FileStatus
			}{
				{oldPath: "old.go", newPath: "new.go", status: Renamed},
			},
		},
		{
			name: "mnemonic prefix format",
			input: "diff --git i/.gitconfig w/.gitconfig\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- i/.gitconfig\n" +
				"+++ w/.gitconfig\n" +
				"@@ -1,3 +1,4 @@\n" +
				" [user]\n" +
				"+    name = test\n" +
				"     email = test@example.com\n",
			wantLen: 1,
			wantDiff: []struct {
				oldPath string
				newPath string
				status  FileStatus
			}{
				{oldPath: ".gitconfig", newPath: ".gitconfig", status: Modified},
			},
		},
		{
			name: "no-prefix format",
			input: "diff --git .gitconfig .gitconfig\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- .gitconfig\n" +
				"+++ .gitconfig\n" +
				"@@ -1,3 +1,4 @@\n" +
				" [user]\n" +
				"+    name = test\n" +
				"     email = test@example.com\n",
			wantLen: 1,
			wantDiff: []struct {
				oldPath string
				newPath string
				status  FileStatus
			}{
				{oldPath: ".gitconfig", newPath: ".gitconfig", status: Modified},
			},
		},
		{
			name: "new file with mnemonic prefix",
			input: "diff --git i/new.go w/new.go\n" +
				"new file mode 100644\n" +
				"index 0000000..abcdefg\n" +
				"--- /dev/null\n" +
				"+++ w/new.go\n" +
				"@@ -0,0 +1,3 @@\n" +
				"+package main\n",
			wantLen: 1,
			wantDiff: []struct {
				oldPath string
				newPath string
				status  FileStatus
			}{
				{oldPath: "new.go", newPath: "new.go", status: Added},
			},
		},
		{
			name: "deleted file with mnemonic prefix",
			input: "diff --git i/old.go w/old.go\n" +
				"deleted file mode 100644\n" +
				"index abcdefg..0000000\n" +
				"--- i/old.go\n" +
				"+++ /dev/null\n" +
				"@@ -1,3 +0,0 @@\n" +
				"-package main\n",
			wantLen: 1,
			wantDiff: []struct {
				oldPath string
				newPath string
				status  FileStatus
			}{
				{oldPath: "old.go", newPath: "old.go", status: Deleted},
			},
		},
		{
			name: "file path with spaces and standard prefix",
			input: "diff --git a/my file.go b/my file.go\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- a/my file.go\n" +
				"+++ b/my file.go\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n",
			wantLen: 1,
			wantDiff: []struct {
				oldPath string
				newPath string
				status  FileStatus
			}{
				{oldPath: "my file.go", newPath: "my file.go", status: Modified},
			},
		},
		{
			name: "file in subdirectory with mnemonic prefix",
			input: "diff --git i/internal/commands/add.go w/internal/commands/add.go\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- i/internal/commands/add.go\n" +
				"+++ w/internal/commands/add.go\n" +
				"@@ -1,3 +1,4 @@\n" +
				" package commands\n" +
				"+// added\n" +
				" import (\n",
			wantLen: 1,
			wantDiff: []struct {
				oldPath string
				newPath string
				status  FileStatus
			}{
				{oldPath: "internal/commands/add.go", newPath: "internal/commands/add.go", status: Modified},
			},
		},
		{
			name: "renamed file with mnemonic prefix",
			input: "diff --git i/old.go w/new.go\n" +
				"similarity index 95%\n" +
				"rename from old.go\n" +
				"rename to new.go\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- i/old.go\n" +
				"+++ w/new.go\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n",
			wantLen: 1,
			wantDiff: []struct {
				oldPath string
				newPath string
				status  FileStatus
			}{
				{oldPath: "old.go", newPath: "new.go", status: Renamed},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diffs := ParseFileDiffs(tc.input)

			if got := len(diffs); got != tc.wantLen {
				t.Fatalf("ParseFileDiffs() returned %d diffs, want %d", got, tc.wantLen)
			}

			for i, want := range tc.wantDiff {
				got := diffs[i]
				if got.OldPath != want.oldPath {
					t.Errorf("diffs[%d].OldPath = %q, want %q", i, got.OldPath, want.oldPath)
				}
				if got.NewPath != want.newPath {
					t.Errorf("diffs[%d].NewPath = %q, want %q", i, got.NewPath, want.newPath)
				}
				if got.Status != want.status {
					t.Errorf("diffs[%d].Status = %v, want %v", i, got.Status, want.status)
				}
			}
		})
	}
}

func TestFileDiff_DisplayPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fd   FileDiff
		want string
	}{
		{
			name: "modified file shows new path",
			fd:   FileDiff{OldPath: "foo.go", NewPath: "foo.go", Status: Modified},
			want: "foo.go",
		},
		{
			name: "added file shows new path",
			fd:   FileDiff{OldPath: "bar.go", NewPath: "bar.go", Status: Added},
			want: "bar.go",
		},
		{
			name: "deleted file shows new path",
			fd:   FileDiff{OldPath: "baz.go", NewPath: "baz.go", Status: Deleted},
			want: "baz.go",
		},
		{
			name: "renamed file shows old -> new",
			fd:   FileDiff{OldPath: "a.go", NewPath: "b.go", Status: Renamed},
			want: "a.go -> b.go",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.fd.DisplayPath(); got != tc.want {
				t.Errorf("DisplayPath() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDiffArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		revision string
		staged   bool
		want     []string
	}{
		{
			name:     "working tree diff",
			revision: "",
			staged:   false,
			want:     []string{"diff"},
		},
		{
			name:     "staged diff",
			revision: "",
			staged:   true,
			want:     []string{"diff", "--cached"},
		},
		{
			name:     "revision diff",
			revision: "HEAD~3",
			staged:   false,
			want:     []string{"diff", "HEAD~3"},
		},
		{
			name:     "staged with revision",
			revision: "HEAD~3",
			staged:   true,
			want:     []string{"diff", "--cached", "HEAD~3"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := DiffArgs(tc.revision, tc.staged)

			if len(got) != len(tc.want) {
				t.Fatalf("DiffArgs(%q, %v) = %v, want %v", tc.revision, tc.staged, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("DiffArgs(%q, %v)[%d] = %q, want %q", tc.revision, tc.staged, i, got[i], tc.want[i])
				}
			}
		})
	}
}
