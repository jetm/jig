//go:build integration

package integration_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/jetm/gti/internal/testhelper"
	"github.com/stretchr/testify/assert"
)

func TestLog_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "content\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, gtiBinary, "log")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader("q\n")
	cmd.Env = append(os.Environ(), "TERM=dumb")
	_ = cmd.Run()

	assert.NoError(t, ctx.Err(), "process should not hang")
}

func TestLog_AllFlag(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "content\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, gtiBinary, "log", "--all")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader("q\n")
	cmd.Env = append(os.Environ(), "TERM=dumb")
	_ = cmd.Run()

	assert.NoError(t, ctx.Err(), "process should not hang with --all flag")
}
