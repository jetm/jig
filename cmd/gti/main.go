package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "gti",
		Short: "Interactive git TUI",
		// Print version in the format: gti version <version> (commit: <hash>, built: <date>)
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
	}

	// Override version template to match spec format
	root.SetVersionTemplate("gti version {{.Version}}\n")

	// Register all 8 subcommand stubs
	for _, name := range []string{
		"add", "hunk-add", "checkout", "diff",
		"fixup", "rebase-interactive", "reset", "log",
	} {
		name := name // capture loop variable
		root.AddCommand(&cobra.Command{
			Use:   name,
			Short: fmt.Sprintf("%s command (not yet implemented)", name),
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Fprintln(cmd.ErrOrStderr(), "not implemented")
				return nil
			},
		})
	}

	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
