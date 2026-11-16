package commands

import (
	"fmt"
	"os"
	"testing"
)

// TestMain points XDG_CONFIG_HOME at a throwaway directory for the whole
// package. Tests that press the panel-resize keys reach config.Save, which
// writes the primary XDG config path, so without this every `go test` run
// overwrites the developer's real ~/.config/jig/config.yaml with defaults
// (commit.command and commit.titleOnlyFlag included). Individual tests that
// need a different location still override it with t.Setenv.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "jig-commands-test-config-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating isolated config dir: %v\n", err)
		os.Exit(1)
	}
	if err := os.Setenv("XDG_CONFIG_HOME", dir); err != nil {
		fmt.Fprintf(os.Stderr, "setting XDG_CONFIG_HOME: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	_ = os.RemoveAll(dir)
	os.Exit(code)
}
