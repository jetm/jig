//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var gtiBinary string

func TestMain(m *testing.M) {
	// Build the gti binary once for all integration tests.
	tmpDir, err := os.MkdirTemp("", "gti-integration-*")
	if err != nil {
		panic("creating temp dir: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	gtiBinary = filepath.Join(tmpDir, "gti")
	cmd := exec.Command("go", "build", "-o", gtiBinary, "../../cmd/gti")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("building gti: " + err.Error())
	}

	os.Exit(m.Run())
}

// runTUI launches gti in TUI mode with a quit keystroke and timeout.
// It returns stderr content (with expected TTY errors filtered out) and any
// error. The caller asserts stderr is empty to catch pre-TUI startup failures
// (e.g. git command errors, precondition failures) while tolerating the
// bubbletea TTY error that occurs in environments without /dev/tty.
func runTUI(t *testing.T, repoDir string, args ...string) (stderr string, err error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, gtiBinary, args...)
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader("q\n")
	cmd.Env = append(os.Environ(), "TERM=dumb")
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	err = cmd.Run()

	require.NoError(t, ctx.Err(), "process should not hang")
	return filterTTYError(stderrBuf.String()), err
}

// filterTTYError filters out the expected bubbletea TTY error from stderr.
// In environments without /dev/tty (CI, sandboxes), bubbletea fails to open
// the terminal and cobra prints the error plus usage text. This is expected
// and not a startup bug. Pre-TUI errors (git failures, precondition checks)
// happen before bubbletea starts and never contain "could not open TTY".
func filterTTYError(stderr string) string {
	if strings.Contains(stderr, "could not open TTY") {
		return ""
	}
	return stderr
}
