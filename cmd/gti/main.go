// Package main provides the gti CLI entry point and subcommand registration.
package main

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/jetm/gti/internal/app"
	"github.com/jetm/gti/internal/commands"
	"github.com/jetm/gti/internal/config"
	"github.com/jetm/gti/internal/diff"
	"github.com/jetm/gti/internal/git"
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

	// Register stub subcommands (all except diff which is implemented)
	for _, name := range []string{
		"add", "hunk-add", "checkout",
		"fixup", "rebase-interactive", "reset", "log",
	} {
		root.AddCommand(&cobra.Command{
			Use:   name,
			Short: fmt.Sprintf("%s command (not yet implemented)", name),
			RunE: func(cmd *cobra.Command, _ []string) error {
				if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "not implemented"); err != nil {
					return fmt.Errorf("writing to stderr: %w", err)
				}
				return nil
			},
		})
	}

	// Register the diff command
	root.AddCommand(newDiffCmd())

	return root
}

func newDiffCmd() *cobra.Command {
	var staged bool

	cmd := &cobra.Command{
		Use:   "diff [revision]",
		Short: "Interactive side-by-side diff viewer",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			var revision string
			if len(args) > 0 {
				revision = args[0]
			}

			ctx := context.Background()
			runner, err := git.NewExecRunner(ctx)
			if err != nil {
				return fmt.Errorf("initializing git runner: %w", err)
			}

			cfg := config.NewDefault()
			renderer := diff.Chain(cfg)
			diffModel := commands.NewDiffModel(ctx, runner, cfg, renderer, revision, staged)

			appModel := app.NewAppModel(newDiffTeaModel(diffModel), runner, cfg)
			p := tea.NewProgram(appModel)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running diff: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&staged, "staged", false, "Show staged (cached) changes")

	return cmd
}

// diffTeaModel wraps DiffModel (child component pattern) as a tea.Model for AppModel.
type diffTeaModel struct {
	inner *commands.DiffModel
}

func newDiffTeaModel(m *commands.DiffModel) *diffTeaModel {
	return &diffTeaModel{inner: m}
}

func (d *diffTeaModel) Init() tea.Cmd { return nil }

func (d *diffTeaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := d.inner.Update(msg)
	return d, cmd
}

func (d *diffTeaModel) View() tea.View {
	return tea.NewView(d.inner.View())
}

func run() error {
	if err := newRootCmd().Execute(); err != nil {
		return fmt.Errorf("executing command: %w", err)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}
