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
			diffModel := commands.NewDiffModel(ctx, runner, cfg, renderer, revision, staged, rawInput)

			appModel := app.New(newDiffTeaModel(diffModel), runner, cfg)
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
				addModel = commands.NewAddModel(ctx, runner, cfg, renderer, args)
			} else {
				addModel = commands.NewAddModel(ctx, runner, cfg, renderer)
			}

			appModel := app.New(newAddTeaModel(addModel), runner, cfg)
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
				checkoutModel = commands.NewCheckoutModel(ctx, runner, cfg, renderer, args)
			} else {
				checkoutModel = commands.NewCheckoutModel(ctx, runner, cfg, renderer)
			}

			appModel := app.New(newCheckoutTeaModel(checkoutModel), runner, cfg)
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
				hunkAddModel = commands.NewHunkAddModel(ctx, runner, cfg, renderer, args)
			} else {
				hunkAddModel = commands.NewHunkAddModel(ctx, runner, cfg, renderer)
			}

			appModel := app.New(newHunkAddTeaModel(hunkAddModel), runner, cfg)
			p := tea.NewProgram(appModel)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running hunk-add: %w", err)
			}
			return nil
		},
	}
}

// hunkAddTeaModel wraps HunkAddModel (child component pattern) as a tea.Model for AppModel.
type hunkAddTeaModel struct {
	inner *commands.HunkAddModel
}

func newHunkAddTeaModel(m *commands.HunkAddModel) *hunkAddTeaModel {
	return &hunkAddTeaModel{inner: m}
}

func (h *hunkAddTeaModel) Init() tea.Cmd { return nil }

func (h *hunkAddTeaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := h.inner.Update(msg)
	return h, cmd
}

func (h *hunkAddTeaModel) View() tea.View {
	return tea.NewView(h.inner.View())
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
				hunkResetModel = commands.NewHunkResetModel(ctx, runner, cfg, renderer, args)
			} else {
				hunkResetModel = commands.NewHunkResetModel(ctx, runner, cfg, renderer)
			}

			appModel := app.New(newHunkResetTeaModel(hunkResetModel), runner, cfg)
			p := tea.NewProgram(appModel)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running hunk-reset: %w", err)
			}
			return nil
		},
	}
}

// hunkResetTeaModel wraps HunkResetModel (child component pattern) as a tea.Model for AppModel.
type hunkResetTeaModel struct {
	inner *commands.HunkResetModel
}

func newHunkResetTeaModel(m *commands.HunkResetModel) *hunkResetTeaModel {
	return &hunkResetTeaModel{inner: m}
}

func (h *hunkResetTeaModel) Init() tea.Cmd { return nil }

func (h *hunkResetTeaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := h.inner.Update(msg)
	return h, cmd
}

func (h *hunkResetTeaModel) View() tea.View {
	return tea.NewView(h.inner.View())
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
				hunkCheckoutModel = commands.NewHunkCheckoutModel(ctx, runner, cfg, renderer, args)
			} else {
				hunkCheckoutModel = commands.NewHunkCheckoutModel(ctx, runner, cfg, renderer)
			}

			appModel := app.New(newHunkCheckoutTeaModel(hunkCheckoutModel), runner, cfg)
			p := tea.NewProgram(appModel)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running hunk-checkout: %w", err)
			}
			return nil
		},
	}
}

// hunkCheckoutTeaModel wraps HunkCheckoutModel (child component pattern) as a tea.Model for AppModel.
type hunkCheckoutTeaModel struct {
	inner *commands.HunkCheckoutModel
}

func newHunkCheckoutTeaModel(m *commands.HunkCheckoutModel) *hunkCheckoutTeaModel {
	return &hunkCheckoutTeaModel{inner: m}
}

func (h *hunkCheckoutTeaModel) Init() tea.Cmd { return nil }

func (h *hunkCheckoutTeaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := h.inner.Update(msg)
	return h, cmd
}

func (h *hunkCheckoutTeaModel) View() tea.View {
	return tea.NewView(h.inner.View())
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

			appModel := app.New(newFixupTeaModel(fixupModel), runner, cfg)
			p := tea.NewProgram(appModel)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running fixup: %w", err)
			}
			return nil
		},
	}
}

// fixupTeaModel wraps FixupModel (child component pattern) as a tea.Model for AppModel.
type fixupTeaModel struct {
	inner *commands.FixupModel
}

func newFixupTeaModel(m *commands.FixupModel) *fixupTeaModel {
	return &fixupTeaModel{inner: m}
}

func (f *fixupTeaModel) Init() tea.Cmd { return nil }

func (f *fixupTeaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := f.inner.Update(msg)
	return f, cmd
}

func (f *fixupTeaModel) View() tea.View {
	return tea.NewView(f.inner.View())
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
			logModel := commands.NewLogModel(ctx, runner, cfg, renderer, ref)

			appModel := app.New(newLogTeaModel(logModel), runner, cfg)
			p := tea.NewProgram(appModel)
			if _, err = p.Run(); err != nil {
				return fmt.Errorf("running log: %w", err)
			}
			return nil
		},
	}
}

// logTeaModel wraps LogModel (child component pattern) as a tea.Model for AppModel.
type logTeaModel struct {
	inner *commands.LogModel
}

func newLogTeaModel(m *commands.LogModel) *logTeaModel {
	return &logTeaModel{inner: m}
}

func (l *logTeaModel) Init() tea.Cmd { return nil }

func (l *logTeaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := l.inner.Update(msg)
	return l, cmd
}

func (l *logTeaModel) View() tea.View {
	return tea.NewView(l.inner.View())
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
				resetModel = commands.NewResetModel(ctx, runner, cfg, renderer, args)
			} else {
				resetModel = commands.NewResetModel(ctx, runner, cfg, renderer)
			}

			appModel := app.New(newResetTeaModel(resetModel), runner, cfg)
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

// resetTeaModel wraps ResetModel (child component pattern) as a tea.Model for AppModel.
type resetTeaModel struct {
	inner *commands.ResetModel
}

func newResetTeaModel(m *commands.ResetModel) *resetTeaModel {
	return &resetTeaModel{inner: m}
}

func (r *resetTeaModel) Init() tea.Cmd { return nil }

func (r *resetTeaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := r.inner.Update(msg)
	return r, cmd
}

func (r *resetTeaModel) View() tea.View {
	return tea.NewView(r.inner.View())
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
			rebaseModel := commands.NewRebaseInteractiveModel(ctx, runner, cfg, renderer, base, todoFilePath)

			rebaseTeaModel := newRebaseInteractiveTeaModel(rebaseModel)
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

// rebaseInteractiveTeaModel wraps RebaseInteractiveModel (child component pattern) as a tea.Model for AppModel.
type rebaseInteractiveTeaModel struct {
	inner *commands.RebaseInteractiveModel
}

func newRebaseInteractiveTeaModel(m *commands.RebaseInteractiveModel) *rebaseInteractiveTeaModel {
	return &rebaseInteractiveTeaModel{inner: m}
}

func (r *rebaseInteractiveTeaModel) Init() tea.Cmd { return nil }

func (r *rebaseInteractiveTeaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := r.inner.Update(msg)
	return r, cmd
}

func (r *rebaseInteractiveTeaModel) View() tea.View {
	return tea.NewView(r.inner.View())
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
