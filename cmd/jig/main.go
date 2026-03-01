// Package main provides the jig CLI entry point and subcommand registration.
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/jetm/jig/internal/app"
	"github.com/jetm/jig/internal/commands"
	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/git"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "jig",
		Short: "Precision git jigs",
		// Print version in the format: jig version <version> (commit: <hash>, built: <date>)
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
	}

	// Override version template to match spec format
	root.SetVersionTemplate("jig version {{.Version}}\n")

	// Register implemented commands
	root.AddCommand(newDiffCmd())
	root.AddCommand(newAddCmd())
	root.AddCommand(newResetCmd())
	root.AddCommand(newCheckoutCmd())
	root.AddCommand(newHunkAddCmd())
	root.AddCommand(newHunkResetCmd())
	root.AddCommand(newHunkCheckoutCmd())
	root.AddCommand(newFixupCmd())
	root.AddCommand(newLogCmd())
	root.AddCommand(newRebaseInteractiveCmd())
	root.AddCommand(newCompletionCmd(root))

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

			// Detect pager mode: stdin is a pipe when git invokes jig as pager.diff
			var rawInput string
			var opts []tea.ProgramOption
			stat, _ := os.Stdin.Stat()
			if stat.Mode()&os.ModeCharDevice == 0 {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				rawInput = string(data)

				ttyIn, ttyOut, err := tea.OpenTTY()
				if err != nil {
					return fmt.Errorf("opening terminal: %w", err)
				}
				defer func() { _ = ttyIn.Close() }()
				defer func() { _ = ttyOut.Close() }()
				opts = append(opts, tea.WithInput(ttyIn), tea.WithOutput(ttyOut))
			}

			ctx := context.Background()
			runner, err := git.NewExecRunner(ctx)
			if err != nil {
				return fmt.Errorf("initializing git runner: %w", err)
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			renderer := diff.Chain(cfg)
			diffModel, err := commands.NewDiffModel(ctx, runner, cfg, renderer, revision, staged, rawInput)
			if err != nil {
				return fmt.Errorf("diff: %w", err)
			}

			appModel := app.New(commands.NewTeaModelAdapter(diffModel), runner, cfg)
			p := tea.NewProgram(appModel, opts...)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running diff: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&staged, "staged", false, "Show staged (cached) changes")

	return cmd
}

func newAddCmd() *cobra.Command {
	var interactive bool

	cmd := &cobra.Command{
		Use:   "add [paths...]",
		Short: "Interactively stage files",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			runner, err := git.NewExecRunner(ctx)
			if err != nil {
				return fmt.Errorf("initializing git runner: %w", err)
			}

			if len(args) > 0 && !interactive {
				return addDirect(ctx, runner, args, cmd.OutOrStdout())
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			renderer := diff.Chain(cfg)
			var addModel *commands.AddModel
			if len(args) > 0 {
				addModel, err = commands.NewAddModel(ctx, runner, cfg, renderer, args)
			} else {
				addModel, err = commands.NewAddModel(ctx, runner, cfg, renderer)
			}
			if err != nil {
				return fmt.Errorf("add: %w", err)
			}

			appModel := app.New(commands.NewTeaModelAdapter(addModel), runner, cfg)
			p := tea.NewProgram(appModel)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running add: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Open TUI even when paths are given")
	return cmd
}

func addDirect(ctx context.Context, runner git.Runner, paths []string, w io.Writer) error {
	paths = commands.ExpandGlobs(paths)
	if len(paths) == 0 {
		_, _ = fmt.Fprintln(w, "Staged 0 file(s)")
		return nil
	}
	if err := git.StageFiles(ctx, runner, paths); err != nil {
		return fmt.Errorf("staging files: %w", err)
	}
	staged, err := runner.Run(ctx, "diff", "--name-only", "--cached")
	if err != nil {
		return fmt.Errorf("listing staged files: %w", err)
	}
	var count int
	for line := range strings.SplitSeq(staged, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	_, _ = fmt.Fprintf(w, "Staged %d file(s)\n", count)
	return nil
}

func newCheckoutCmd() *cobra.Command {
	var direct bool

	cmd := &cobra.Command{
		Use:   "checkout [paths...]",
		Short: "Interactively discard file changes",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			runner, err := git.NewExecRunner(ctx)
			if err != nil {
				return fmt.Errorf("initializing git runner: %w", err)
			}

			if direct && len(args) > 0 {
				return checkoutDirect(ctx, runner, args, cmd.InOrStdin(), cmd.OutOrStdout())
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			renderer := diff.Chain(cfg)
			var checkoutModel *commands.CheckoutModel
			if len(args) > 0 {
				checkoutModel, err = commands.NewCheckoutModel(ctx, runner, cfg, renderer, args)
			} else {
				checkoutModel, err = commands.NewCheckoutModel(ctx, runner, cfg, renderer)
			}
			if err != nil {
				return fmt.Errorf("checkout: %w", err)
			}

			appModel := app.New(commands.NewTeaModelAdapter(checkoutModel), runner, cfg)
			p := tea.NewProgram(appModel)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running checkout: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&direct, "direct", "d", false, "Discard files directly without opening TUI")
	return cmd
}

func checkoutDirect(ctx context.Context, runner git.Runner, paths []string, in io.Reader, w io.Writer) error {
	paths = commands.ExpandGlobs(paths)
	if len(paths) == 0 {
		_, _ = fmt.Fprintln(w, "No changes to discard")
		return nil
	}
	// Show what will be discarded
	diffArgs := append([]string{"diff", "--name-only", "--"}, paths...)
	affected, err := runner.Run(ctx, diffArgs...)
	if err != nil {
		return fmt.Errorf("listing changed files: %w", err)
	}
	var count int
	for line := range strings.SplitSeq(affected, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	if count == 0 {
		_, _ = fmt.Fprintln(w, "No changes to discard")
		return nil
	}

	_, _ = fmt.Fprintf(w, "Discard changes to %d file(s)? [y/N] ", count)
	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		_, _ = fmt.Fprintln(w, "Aborted")
		return nil
	}
	answer := strings.TrimSpace(scanner.Text())
	if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
		_, _ = fmt.Fprintln(w, "Aborted")
		return nil
	}

	if err := git.DiscardFiles(ctx, runner, paths); err != nil {
		return fmt.Errorf("discarding files: %w", err)
	}
	_, _ = fmt.Fprintf(w, "Discarded changes to %d file(s)\n", count)
	return nil
}

func newHunkAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hunk-add [paths...]",
		Short: "Interactively stage hunks",
		Args:  cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.Background()
			runner, err := git.NewExecRunner(ctx)
			if err != nil {
				return fmt.Errorf("initializing git runner: %w", err)
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			renderer := diff.Chain(cfg)
			var hunkAddModel *commands.HunkAddModel
			if len(args) > 0 {
				hunkAddModel, err = commands.NewHunkAddModel(ctx, runner, cfg, renderer, args)
			} else {
				hunkAddModel, err = commands.NewHunkAddModel(ctx, runner, cfg, renderer)
			}
			if err != nil {
				return fmt.Errorf("hunk-add: %w", err)
			}

			appModel := app.New(commands.NewTeaModelAdapter(hunkAddModel), runner, cfg)
			p := tea.NewProgram(appModel)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running hunk-add: %w", err)
			}
			return nil
		},
	}
}

func newHunkResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hunk-reset [paths...]",
		Short: "Interactively unstage hunks",
		Args:  cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.Background()
			runner, err := git.NewExecRunner(ctx)
			if err != nil {
				return fmt.Errorf("initializing git runner: %w", err)
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			renderer := diff.Chain(cfg)
			var hunkResetModel *commands.HunkResetModel
			if len(args) > 0 {
				hunkResetModel, err = commands.NewHunkResetModel(ctx, runner, cfg, renderer, args)
			} else {
				hunkResetModel, err = commands.NewHunkResetModel(ctx, runner, cfg, renderer)
			}
			if err != nil {
				return fmt.Errorf("hunk-reset: %w", err)
			}

			appModel := app.New(commands.NewTeaModelAdapter(hunkResetModel), runner, cfg)
			p := tea.NewProgram(appModel)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running hunk-reset: %w", err)
			}
			return nil
		},
	}
}

func newHunkCheckoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hunk-checkout [paths...]",
		Short: "Interactively discard hunks",
		Args:  cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.Background()
			runner, err := git.NewExecRunner(ctx)
			if err != nil {
				return fmt.Errorf("initializing git runner: %w", err)
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			renderer := diff.Chain(cfg)
			var hunkCheckoutModel *commands.HunkCheckoutModel
			if len(args) > 0 {
				hunkCheckoutModel, err = commands.NewHunkCheckoutModel(ctx, runner, cfg, renderer, args)
			} else {
				hunkCheckoutModel, err = commands.NewHunkCheckoutModel(ctx, runner, cfg, renderer)
			}
			if err != nil {
				return fmt.Errorf("hunk-checkout: %w", err)
			}

			appModel := app.New(commands.NewTeaModelAdapter(hunkCheckoutModel), runner, cfg)
			p := tea.NewProgram(appModel)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running hunk-checkout: %w", err)
			}
			return nil
		},
	}
}

func newFixupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fixup",
		Short: "Interactively create a fixup commit",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.Background()
			runner, err := git.NewExecRunner(ctx)
			if err != nil {
				return fmt.Errorf("initializing git runner: %w", err)
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			renderer := diff.Chain(cfg)
			fixupModel, err := commands.NewFixupModel(ctx, runner, cfg, renderer)
			if err != nil {
				return fmt.Errorf("fixup: %w", err)
			}

			appModel := app.New(commands.NewTeaModelAdapter(fixupModel), runner, cfg)
			p := tea.NewProgram(appModel)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running fixup: %w", err)
			}
			return nil
		},
	}
}

func newLogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "log [revision]",
		Short: "Interactive commit log browser",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			var ref string
			if len(args) > 0 {
				ref = args[0]
			}

			ctx := context.Background()
			runner, err := git.NewExecRunner(ctx)
			if err != nil {
				return fmt.Errorf("initializing git runner: %w", err)
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			renderer := diff.Chain(cfg)
			logModel, err := commands.NewLogModel(ctx, runner, cfg, renderer, ref)
			if err != nil {
				return fmt.Errorf("log: %w", err)
			}

			appModel := app.New(commands.NewTeaModelAdapter(logModel), runner, cfg)
			p := tea.NewProgram(appModel)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running log: %w", err)
			}
			return nil
		},
	}
}

func newResetCmd() *cobra.Command {
	var interactive bool

	cmd := &cobra.Command{
		Use:   "reset [paths...]",
		Short: "Interactively unstage files",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			runner, err := git.NewExecRunner(ctx)
			if err != nil {
				return fmt.Errorf("initializing git runner: %w", err)
			}

			if len(args) > 0 && !interactive {
				return resetDirect(ctx, runner, args, cmd.OutOrStdout())
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			renderer := diff.Chain(cfg)
			var resetModel *commands.ResetModel
			if len(args) > 0 {
				resetModel, err = commands.NewResetModel(ctx, runner, cfg, renderer, args)
			} else {
				resetModel, err = commands.NewResetModel(ctx, runner, cfg, renderer)
			}
			if err != nil {
				return fmt.Errorf("reset: %w", err)
			}

			appModel := app.New(commands.NewTeaModelAdapter(resetModel), runner, cfg)
			p := tea.NewProgram(appModel)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running reset: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Open TUI even when paths are given")
	return cmd
}

func resetDirect(ctx context.Context, runner git.Runner, paths []string, w io.Writer) error {
	paths = commands.ExpandGlobs(paths)
	if len(paths) == 0 {
		_, _ = fmt.Fprintln(w, "Nothing to unstage")
		return nil
	}
	if err := git.UnstageFiles(ctx, runner, paths); err != nil {
		return fmt.Errorf("unstaging files: %w", err)
	}
	status, err := runner.Run(ctx, "status", "--short")
	if err != nil {
		return fmt.Errorf("getting status: %w", err)
	}
	_, _ = fmt.Fprint(w, status)
	return nil
}

func newRebaseInteractiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rebase-interactive [revision]",
		Short: "Interactive rebase todo editor",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			var base, todoFilePath string
			if len(args) > 0 {
				// Detect mode: if arg is an existing file, it's editor mode
				if info, err := os.Stat(args[0]); err == nil && !info.IsDir() {
					todoFilePath = args[0]
				} else {
					base = args[0]
				}
			}

			ctx := context.Background()
			runner, err := git.NewExecRunner(ctx)
			if err != nil {
				return fmt.Errorf("initializing git runner: %w", err)
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			renderer := diff.Chain(cfg)
			rebaseModel, err := commands.NewRebaseInteractiveModel(ctx, runner, cfg, renderer, base, todoFilePath)
			if err != nil {
				return fmt.Errorf("rebase-interactive: %w", err)
			}

			rebaseTeaModel := commands.NewTeaModelAdapter(rebaseModel)
			appModel := app.New(rebaseTeaModel, runner, cfg)

			// In editor mode (invoked as GIT_SEQUENCE_EDITOR), explicitly open
			// /dev/tty for TUI I/O. Git may redirect stdin/stdout when calling
			// the sequence editor, so we can't rely on inherited file descriptors.
			var opts []tea.ProgramOption
			if todoFilePath != "" {
				ttyIn, ttyOut, err := tea.OpenTTY()
				if err != nil {
					return fmt.Errorf("opening terminal: %w", err)
				}
				defer func() { _ = ttyIn.Close() }()
				defer func() { _ = ttyOut.Close() }()
				opts = append(opts, tea.WithInput(ttyIn), tea.WithOutput(ttyOut))
			}

			p := tea.NewProgram(appModel, opts...)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running rebase-interactive: %w", err)
			}
			if appModel.Aborted {
				os.Exit(1)
			}
			return nil
		},
	}
}

// newCompletionCmd returns a cobra command that generates shell completion scripts.
// root is the parent command whose subcommands are included in completions.
func newCompletionCmd(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Generate shell completion scripts",
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			}
			return nil
		},
	}
	return cmd
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
