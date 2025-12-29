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

	// Register stub subcommands (not yet implemented)
	for _, name := range []string{
		"hunk-add",
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

	// Register implemented commands
	root.AddCommand(newDiffCmd())
	root.AddCommand(newAddCmd())
	root.AddCommand(newCheckoutCmd())

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

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Interactively stage files",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.Background()
			runner, err := git.NewExecRunner(ctx)
			if err != nil {
				return fmt.Errorf("initializing git runner: %w", err)
			}

			cfg := config.NewDefault()
			renderer := diff.Chain(cfg)
			addModel := commands.NewAddModel(ctx, runner, cfg, renderer)

			appModel := app.NewAppModel(newAddTeaModel(addModel), runner, cfg)
			p := tea.NewProgram(appModel)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running add: %w", err)
			}
			return nil
		},
	}
}

// addTeaModel wraps AddModel (child component pattern) as a tea.Model for AppModel.
type addTeaModel struct {
	inner *commands.AddModel
}

func newAddTeaModel(m *commands.AddModel) *addTeaModel {
	return &addTeaModel{inner: m}
}

func (a *addTeaModel) Init() tea.Cmd { return nil }

func (a *addTeaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := a.inner.Update(msg)
	return a, cmd
}

func (a *addTeaModel) View() tea.View {
	return tea.NewView(a.inner.View())
}

func newCheckoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "checkout",
		Short: "Interactively discard file changes",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.Background()
			runner, err := git.NewExecRunner(ctx)
			if err != nil {
				return fmt.Errorf("initializing git runner: %w", err)
			}

			cfg := config.NewDefault()
			renderer := diff.Chain(cfg)
			checkoutModel := commands.NewCheckoutModel(ctx, runner, cfg, renderer)

			appModel := app.NewAppModel(newCheckoutTeaModel(checkoutModel), runner, cfg)
			p := tea.NewProgram(appModel)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running checkout: %w", err)
			}
			return nil
		},
	}
}

// checkoutTeaModel wraps CheckoutModel (child component pattern) as a tea.Model for AppModel.
type checkoutTeaModel struct {
	inner *commands.CheckoutModel
}

func newCheckoutTeaModel(m *commands.CheckoutModel) *checkoutTeaModel {
	return &checkoutTeaModel{inner: m}
}

func (c *checkoutTeaModel) Init() tea.Cmd { return nil }

func (c *checkoutTeaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := c.inner.Update(msg)
	return c, cmd
}

func (c *checkoutTeaModel) View() tea.View {
	return tea.NewView(c.inner.View())
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
