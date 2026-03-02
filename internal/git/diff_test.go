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
		{
			name: "empty new file with mnemonic prefix (no ---/+++ lines)",
			input: "diff --git c/foo i/foo\n" +
				"new file mode 100644\n" +
				"index 0000000..e69de29\n",
			wantLen: 1,
			wantDiff: []struct {
				oldPath string
				newPath string
				status  FileStatus
			}{
				{oldPath: "foo", newPath: "foo", status: Added},
			},
		},
		{
			name: "empty new file in subdirectory with mnemonic prefix",
			input: "diff --git c/pkg/util/empty.go i/pkg/util/empty.go\n" +
				"new file mode 100644\n" +
				"index 0000000..e69de29\n",
			wantLen: 1,
			wantDiff: []struct {
				oldPath string
				newPath string
				status  FileStatus
			}{
				{oldPath: "pkg/util/empty.go", newPath: "pkg/util/empty.go", status: Added},
			},
		},
		{
			name: "diff --git inside hunk body does not split",
			input: "diff --git a/test.go b/test.go\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- a/test.go\n" +
				"+++ b/test.go\n" +
				"@@ -1,3 +1,4 @@\n" +
				" line1\n" +
				"+const s = \"diff --git a/foo b/foo\"\n" +
				" line2\n",
			wantLen: 1,
			wantDiff: []struct {
				oldPath string
				newPath string
				status  FileStatus
			}{
				{oldPath: "test.go", newPath: "test.go", status: Modified},
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

func TestParseFileDiffs_BinaryDetection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantLen    int
		wantBinary []bool
		wantStatus []FileStatus
		wantPaths  []string // NewPath for each diff
	}{
		{
			name: "new binary file",
			input: "diff --git a/image.png b/image.png\n" +
				"new file mode 100644\n" +
				"index 0000000..abcdefg\n" +
				"Binary files /dev/null and b/image.png differ\n",
			wantLen:    1,
			wantBinary: []bool{true},
			wantStatus: []FileStatus{Added},
			wantPaths:  []string{"image.png"},
		},
		{
			name: "modified binary file",
			input: "diff --git a/icon.png b/icon.png\n" +
				"index 1234567..abcdefg 100644\n" +
				"Binary files a/icon.png and b/icon.png differ\n",
			wantLen:    1,
			wantBinary: []bool{true},
			wantStatus: []FileStatus{Modified},
			wantPaths:  []string{"icon.png"},
		},
		{
			name: "deleted binary file",
			input: "diff --git a/old.bin b/old.bin\n" +
				"deleted file mode 100644\n" +
				"index abcdefg..0000000\n" +
				"Binary files a/old.bin and /dev/null differ\n",
			wantLen:    1,
			wantBinary: []bool{true},
			wantStatus: []FileStatus{Deleted},
			wantPaths:  []string{"old.bin"},
		},
		{
			name: "text file is not binary",
			input: "diff --git a/main.go b/main.go\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- a/main.go\n" +
				"+++ b/main.go\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n",
			wantLen:    1,
			wantBinary: []bool{false},
			wantStatus: []FileStatus{Modified},
			wantPaths:  []string{"main.go"},
		},
		{
			name: "mixed binary and text diffs",
			input: "diff --git a/readme.md b/readme.md\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- a/readme.md\n" +
				"+++ b/readme.md\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n" +
				"diff --git a/logo.png b/logo.png\n" +
				"new file mode 100644\n" +
				"index 0000000..abcdefg\n" +
				"Binary files /dev/null and b/logo.png differ\n" +
				"diff --git a/app.go b/app.go\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- a/app.go\n" +
				"+++ b/app.go\n" +
				"@@ -1 +1 @@\n" +
				"-foo\n" +
				"+bar\n",
			wantLen:    3,
			wantBinary: []bool{false, true, false},
			wantStatus: []FileStatus{Modified, Added, Modified},
			wantPaths:  []string{"readme.md", "logo.png", "app.go"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diffs := ParseFileDiffs(tc.input)

			if got := len(diffs); got != tc.wantLen {
				t.Fatalf("ParseFileDiffs() returned %d diffs, want %d", got, tc.wantLen)
			}

			for i, d := range diffs {
				if d.Binary != tc.wantBinary[i] {
					t.Errorf("diffs[%d].Binary = %v, want %v", i, d.Binary, tc.wantBinary[i])
				}
				if d.Status != tc.wantStatus[i] {
					t.Errorf("diffs[%d].Status = %v, want %v", i, d.Status, tc.wantStatus[i])
				}
				if d.NewPath != tc.wantPaths[i] {
					t.Errorf("diffs[%d].NewPath = %q, want %q", i, d.NewPath, tc.wantPaths[i])
				}
			}
		})
	}
}

func TestParseFileDiffs_UnicodeFilenames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantLen  int
		wantDiff struct {
			oldPath string
			newPath string
			status  FileStatus
		}
	}{
		{
			name: "latin unicode filename",
			input: "diff --git a/caf\u00e9.txt b/caf\u00e9.txt\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- a/caf\u00e9.txt\n" +
				"+++ b/caf\u00e9.txt\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n",
			wantLen: 1,
			wantDiff: struct {
				oldPath string
				newPath string
				status  FileStatus
			}{oldPath: "caf\u00e9.txt", newPath: "caf\u00e9.txt", status: Modified},
		},
		{
			name: "CJK unicode filename",
			input: "diff --git a/\u65e5\u672c\u8a9e.go b/\u65e5\u672c\u8a9e.go\n" +
				"new file mode 100644\n" +
				"index 0000000..abcdefg\n" +
				"--- /dev/null\n" +
				"+++ b/\u65e5\u672c\u8a9e.go\n" +
				"@@ -0,0 +1 @@\n" +
				"+package main\n",
			wantLen: 1,
			wantDiff: struct {
				oldPath string
				newPath string
				status  FileStatus
			}{oldPath: "\u65e5\u672c\u8a9e.go", newPath: "\u65e5\u672c\u8a9e.go", status: Added},
		},
		{
			name: "emoji filename",
			input: "diff --git a/\U0001f680.md b/\U0001f680.md\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- a/\U0001f680.md\n" +
				"+++ b/\U0001f680.md\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n",
			wantLen: 1,
			wantDiff: struct {
				oldPath string
				newPath string
				status  FileStatus
			}{oldPath: "\U0001f680.md", newPath: "\U0001f680.md", status: Modified},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diffs := ParseFileDiffs(tc.input)

			if got := len(diffs); got != tc.wantLen {
				t.Fatalf("ParseFileDiffs() returned %d diffs, want %d", got, tc.wantLen)
			}

			got := diffs[0]
			if got.OldPath != tc.wantDiff.oldPath {
				t.Errorf("OldPath = %q, want %q", got.OldPath, tc.wantDiff.oldPath)
			}
			if got.NewPath != tc.wantDiff.newPath {
				t.Errorf("NewPath = %q, want %q", got.NewPath, tc.wantDiff.newPath)
			}
			if got.Status != tc.wantDiff.status {
				t.Errorf("Status = %v, want %v", got.Status, tc.wantDiff.status)
			}
		})
	}
}

func TestParseFileDiffs_FilenamesWithSpaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantLen  int
		wantDiff struct {
			oldPath string
			newPath string
			status  FileStatus
		}
	}{
		{
			name: "spaces with mnemonic prefix",
			input: "diff --git i/my file.go w/my file.go\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- i/my file.go\n" +
				"+++ w/my file.go\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n",
			wantLen: 1,
			wantDiff: struct {
				oldPath string
				newPath string
				status  FileStatus
			}{oldPath: "my file.go", newPath: "my file.go", status: Modified},
		},
		{
			name: "spaces in subdirectory with standard prefix",
			input: "diff --git a/path with spaces/foo.go b/path with spaces/foo.go\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- a/path with spaces/foo.go\n" +
				"+++ b/path with spaces/foo.go\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n",
			wantLen: 1,
			wantDiff: struct {
				oldPath string
				newPath string
				status  FileStatus
			}{oldPath: "path with spaces/foo.go", newPath: "path with spaces/foo.go", status: Modified},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diffs := ParseFileDiffs(tc.input)

			if got := len(diffs); got != tc.wantLen {
				t.Fatalf("ParseFileDiffs() returned %d diffs, want %d", got, tc.wantLen)
			}

			got := diffs[0]
			if got.OldPath != tc.wantDiff.oldPath {
				t.Errorf("OldPath = %q, want %q", got.OldPath, tc.wantDiff.oldPath)
			}
			if got.NewPath != tc.wantDiff.newPath {
				t.Errorf("NewPath = %q, want %q", got.NewPath, tc.wantDiff.newPath)
			}
			if got.Status != tc.wantDiff.status {
				t.Errorf("Status = %v, want %v", got.Status, tc.wantDiff.status)
			}
		})
	}
}

func TestParseFileDiffs_QuotedPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantLen  int
		wantOld  string
		wantNew  string
		wantStat FileStatus
	}{
		{
			name: "quoted header with unquoted ---/+++ lines",
			input: "diff --git \"a/path/caf\\303\\251.txt\" \"b/path/caf\\303\\251.txt\"\n" +
				"index 1234567..abcdefg 100644\n" +
				"--- a/path/caf\u00e9.txt\n" +
				"+++ b/path/caf\u00e9.txt\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n",
			wantLen:  1,
			wantOld:  "path/caf\u00e9.txt",
			wantNew:  "path/caf\u00e9.txt",
			wantStat: Modified,
		},
		{
			// Git quotes paths with special characters using octal escapes in
			// the header. When ---/+++ lines are absent (binary files), the
			// header fallback parser cannot split quoted paths, so both paths
			// are empty. This documents current behavior.
			name: "quoted header only (binary file, no ---/+++ lines)",
			input: "diff --git \"a/caf\\303\\251.bin\" \"b/caf\\303\\251.bin\"\n" +
				"index 1234567..abcdefg 100644\n" +
				"Binary files \"a/caf\\303\\251.bin\" and \"b/caf\\303\\251.bin\" differ\n",
			wantLen:  1,
			wantOld:  "",
			wantNew:  "",
			wantStat: Modified,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diffs := ParseFileDiffs(tc.input)

			if got := len(diffs); got != tc.wantLen {
				t.Fatalf("ParseFileDiffs() returned %d diffs, want %d", got, tc.wantLen)
			}

			got := diffs[0]
			if got.OldPath != tc.wantOld {
				t.Errorf("OldPath = %q, want %q", got.OldPath, tc.wantOld)
			}
			if got.NewPath != tc.wantNew {
				t.Errorf("NewPath = %q, want %q", got.NewPath, tc.wantNew)
			}
			if got.Status != tc.wantStat {
				t.Errorf("Status = %v, want %v", got.Status, tc.wantStat)
			}
		})
	}
}

func TestParseFileDiffs_RenamesWithSpecialPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantOld  string
		wantNew  string
		wantStat FileStatus
	}{
		{
			name: "rename with spaces in old and new path",
			input: "diff --git a/old file.go b/new file.go\n" +
				"similarity index 100%\n" +
				"rename from old file.go\n" +
				"rename to new file.go\n",
			wantOld:  "old file.go",
			wantNew:  "new file.go",
			wantStat: Renamed,
		},
		{
			name: "rename with unicode in old and new path",
			input: "diff --git a/caf\u00e9.txt b/\u65e5\u672c\u8a9e.txt\n" +
				"similarity index 100%\n" +
				"rename from caf\u00e9.txt\n" +
				"rename to \u65e5\u672c\u8a9e.txt\n",
			wantOld:  "caf\u00e9.txt",
			wantNew:  "\u65e5\u672c\u8a9e.txt",
			wantStat: Renamed,
		},
		{
			name: "rename with spaces and unicode combined",
			input: "diff --git a/my caf\u00e9.txt b/new \u65e5\u672c\u8a9e.txt\n" +
				"similarity index 100%\n" +
				"rename from my caf\u00e9.txt\n" +
				"rename to new \u65e5\u672c\u8a9e.txt\n",
			wantOld:  "my caf\u00e9.txt",
			wantNew:  "new \u65e5\u672c\u8a9e.txt",
			wantStat: Renamed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diffs := ParseFileDiffs(tc.input)

			if got := len(diffs); got != 1 {
				t.Fatalf("ParseFileDiffs() returned %d diffs, want 1", got)
			}

			got := diffs[0]
			if got.OldPath != tc.wantOld {
				t.Errorf("OldPath = %q, want %q", got.OldPath, tc.wantOld)
			}
			if got.NewPath != tc.wantNew {
				t.Errorf("NewPath = %q, want %q", got.NewPath, tc.wantNew)
			}
			if got.Status != tc.wantStat {
				t.Errorf("Status = %v, want %v", got.Status, tc.wantStat)
			}
			// Also verify DisplayPath shows "old -> new" for renames
			wantDisplay := tc.wantOld + " -> " + tc.wantNew
			if got.DisplayPath() != wantDisplay {
				t.Errorf("DisplayPath() = %q, want %q", got.DisplayPath(), wantDisplay)
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
		name         string
		revision     string
		staged       bool
		contextLines int
		want         []string
	}{
		{
			name:         "working tree diff default context",
			revision:     "",
			staged:       false,
			contextLines: -1,
			want:         []string{"diff"},
		},
		{
			name:         "staged diff default context",
			revision:     "",
			staged:       true,
			contextLines: -1,
			want:         []string{"diff", "--cached"},
		},
		{
			name:         "revision diff default context",
			revision:     "HEAD~3",
			staged:       false,
			contextLines: -1,
			want:         []string{"diff", "HEAD~3"},
		},
		{
			name:         "staged with revision default context",
			revision:     "HEAD~3",
			staged:       true,
			contextLines: -1,
			want:         []string{"diff", "--cached", "HEAD~3"},
		},
		{
			name:         "explicit context lines",
			revision:     "",
			staged:       false,
			contextLines: 5,
			want:         []string{"diff", "-U5"},
		},
		{
			name:         "zero context lines",
			revision:     "",
			staged:       false,
			contextLines: 0,
			want:         []string{"diff", "-U0"},
		},
		{
			name:         "context lines with staged and revision",
			revision:     "HEAD~3",
			staged:       true,
			contextLines: 10,
			want:         []string{"diff", "-U10", "--cached", "HEAD~3"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := DiffArgs(tc.revision, tc.staged, tc.contextLines)

			if len(got) != len(tc.want) {
				t.Fatalf("DiffArgs(%q, %v, %d) = %v, want %v", tc.revision, tc.staged, tc.contextLines, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("DiffArgs(%q, %v, %d)[%d] = %q, want %q", tc.revision, tc.staged, tc.contextLines, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestStripDiffPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"a/foo.go", "foo.go"},
		{"b/bar.go", "bar.go"},
		{"foo.go", "foo.go"},
		{"a/", "a/"},
		{"x", "x"},
	}
	for _, tt := range tests {
		if got := stripDiffPrefix(tt.input); got != tt.want {
			t.Errorf("stripDiffPrefix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractRenameField(t *testing.T) {
	t.Parallel()
	block := "diff --git a/old.go b/new.go\nrename from old.go\nrename to new.go\n--- a/old.go"
	if got := extractRenameField(block, "rename from "); got != "old.go" {
		t.Errorf("extractRenameField(rename from) = %q, want %q", got, "old.go")
	}
	if got := extractRenameField(block, "rename to "); got != "new.go" {
		t.Errorf("extractRenameField(rename to) = %q, want %q", got, "new.go")
	}
	if got := extractRenameField(block, "missing "); got != "" {
		t.Errorf("extractRenameField(missing) = %q, want empty", got)
	}
	// Field at end of block without trailing newline
	blockNoTrail := "header\nrename from old.go"
	if got := extractRenameField(blockNoTrail, "rename from "); got != "old.go" {
		t.Errorf("extractRenameField(no trailing newline) = %q, want %q", got, "old.go")
	}
}
