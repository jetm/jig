package git_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/testhelper"
)

// standardDiff is a diff using standard a/b/ prefixes.
const standardDiff = `diff --git a/foo.go b/foo.go
--- a/foo.go
+++ b/foo.go
@@ -1 +1 @@
-old
+new
`

// mnemonicDiff is a diff using mnemonic i/w/ prefixes (git diff --ita-invisible-in-index).
const mnemonicDiff = `diff --git i/foo.go w/foo.go
--- i/foo.go
+++ w/foo.go
@@ -1 +1 @@
-old
+new
`

func TestEditDiff_WritesStandardPrefixToTempFile(t *testing.T) {
	// Cannot run in parallel - uses t.Setenv and writes to the same temp file path.
	t.Setenv("GIT_EDITOR", "true") // "true" command exists everywhere and does nothing
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	fr := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("no config")},
	}

	cmd := git.EditDiff(context.Background(), fr, standardDiff)
	if cmd == nil {
		t.Fatal("EditDiff returned nil cmd")
	}

	// The cmd returns an EditDiffMsg when executed - for write verification
	// we just check that the temp file contains the correct content.
	tmpPath := filepath.Join(os.TempDir(), "addp-hunk-edit.diff")
	content, err := os.ReadFile(tmpPath) //nolint:gosec
	if err != nil {
		t.Fatalf("temp file not written: %v", err)
	}
	if string(content) != standardDiff {
		t.Errorf("temp file content = %q, want %q", string(content), standardDiff)
	}
}

func TestEditDiff_WritesMnemonicPrefixToTempFile(t *testing.T) {
	// Cannot run in parallel - writes to the same temp file path.
	t.Setenv("GIT_EDITOR", "true")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	fr := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("no config")},
	}

	cmd := git.EditDiff(context.Background(), fr, mnemonicDiff)
	if cmd == nil {
		t.Fatal("EditDiff returned nil cmd")
	}

	tmpPath := filepath.Join(os.TempDir(), "addp-hunk-edit.diff")
	content, err := os.ReadFile(tmpPath) //nolint:gosec
	if err != nil {
		t.Fatalf("temp file not written: %v", err)
	}
	if string(content) != mnemonicDiff {
		t.Errorf("temp file content = %q, want %q", string(content), mnemonicDiff)
	}
}

func TestApplyEditedDiff_ModifiedDiffApplies(t *testing.T) {
	t.Parallel()
	fr := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{nil},
	}

	// Write a modified diff to temp file.
	tmpPath := filepath.Join(t.TempDir(), "addp-hunk-edit.diff")
	modifiedDiff := standardDiff + "\n// modified"
	if err := os.WriteFile(tmpPath, []byte(modifiedDiff), 0o600); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	err := git.ApplyEditedDiff(context.Background(), fr, standardDiff, tmpPath)
	if err != nil {
		t.Fatalf("ApplyEditedDiff returned error: %v", err)
	}

	// Verify git apply --cached --recount was called.
	testhelper.MustHaveCall(t, fr, "apply", "--cached", "--recount")
	testhelper.MustHaveStdin(t, fr, "// modified")
}

func TestApplyEditedDiff_UnmodifiedDiffSkipsApply(t *testing.T) {
	t.Parallel()
	fr := &testhelper.FakeRunner{}

	// Write the same content as originalDiff.
	tmpPath := filepath.Join(t.TempDir(), "addp-hunk-edit.diff")
	if err := os.WriteFile(tmpPath, []byte(standardDiff), 0o600); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	err := git.ApplyEditedDiff(context.Background(), fr, standardDiff, tmpPath)
	if err != nil {
		t.Fatalf("ApplyEditedDiff returned error for unchanged diff: %v", err)
	}

	// No git call should have been made.
	testhelper.MustHaveNoCall(t, fr)
}

func TestApplyEditedDiff_ApplyErrorIsReturned(t *testing.T) {
	t.Parallel()
	applyErr := errors.New("patch does not apply")
	fr := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{applyErr},
	}

	tmpPath := filepath.Join(t.TempDir(), "addp-hunk-edit.diff")
	modifiedDiff := standardDiff + "\n// bad patch"
	if err := os.WriteFile(tmpPath, []byte(modifiedDiff), 0o600); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	err := git.ApplyEditedDiff(context.Background(), fr, standardDiff, tmpPath)
	if err == nil {
		t.Fatal("expected error from ApplyEditedDiff when git apply fails")
	}
}

func TestApplyEditedDiff_MissingFileReturnsError(t *testing.T) {
	t.Parallel()
	fr := &testhelper.FakeRunner{}

	err := git.ApplyEditedDiff(context.Background(), fr, standardDiff, "/nonexistent/path/file.diff")
	if err == nil {
		t.Fatal("expected error when edited file does not exist")
	}
}
