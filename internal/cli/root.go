package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/kevinsheth/rollbaz/internal/app"
	"github.com/kevinsheth/rollbaz/internal/config"
	"github.com/kevinsheth/rollbaz/internal/domain"
	"github.com/kevinsheth/rollbaz/internal/output"
	"github.com/kevinsheth/rollbaz/internal/redact"
	"github.com/kevinsheth/rollbaz/internal/rollbar"
)

type rootFlags struct {
	Format  string
	Project string
	Token   string
	Limit   int
}

var (
	newRollbarClient           = rollbar.New
	newConfigStore             = config.NewStore
	stdoutWriter     io.Writer = os.Stdout
	stderrWriter     io.Writer = os.Stderr
)

func NewRootCmd() *cobra.Command {
	flags := &rootFlags{}

	cmd := &cobra.Command{
		Use:          "rollbaz",
		Short:        "Fast Rollbar triage from your terminal",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActive(cmd.Context(), *flags)
		},
	}

	cmd.PersistentFlags().StringVar(&flags.Format, "format", "human", "Output format: human or json")
	cmd.PersistentFlags().StringVar(&flags.Project, "project", "", "Configured project name")
	cmd.PersistentFlags().StringVar(&flags.Token, "token", "", "Rollbar project token (overrides configured project token)")
	cmd.PersistentFlags().IntVar(&flags.Limit, "limit", 10, "Maximum number of issues to show")

	cmd.AddCommand(newActiveCmd(flags))
	cmd.AddCommand(newRecentCmd(flags))
	cmd.AddCommand(newShowCmd(flags))
	cmd.AddCommand(newProjectCmd())

	return cmd
}

func Execute() int {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(stderrWriter, err)
		return 1
	}

	return 0
}

func newActiveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "active",
		Short: "List active issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runActive(cmd.Context(), *flags)
		},
	}
}

func newRecentCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "recent",
		Short: "List most recently seen active issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecent(cmd.Context(), *flags)
		},
	}
}

func newShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show <item-counter>",
		Short: "Show details for one item counter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parsedCounter, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("parse item counter: %w", err)
			}
			return runShow(cmd.Context(), *flags, domain.ItemCounter(parsedCounter))
		},
	}
}

func newProjectCmd() *cobra.Command {
	projectCmd := &cobra.Command{Use: "project", Short: "Manage configured Rollbar projects"}
	projectCmd.AddCommand(
		newProjectAddCmd(),
		newProjectListCmd(),
		newProjectUseCmd(),
		newProjectNextCmd(),
		newProjectRemoveCmd(),
	)

	return projectCmd
}

func newProjectAddCmd() *cobra.Command {
	addToken := ""
	addCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add or update a project token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := withConfigStore(func(store *config.Store) error {
				return store.AddProject(args[0], addToken)
			}); err != nil {
				return fmt.Errorf("add project: %w", err)
			}
			return nil
		},
	}
	addCmd.Flags().StringVar(&addToken, "token", "", "Project token")
	_ = addCmd.MarkFlagRequired("token")

	return addCmd
}

func newProjectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withConfigStore(printProjects)
		},
	}
}

func newProjectUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set active project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := withConfigStore(func(store *config.Store) error {
				return store.UseProject(args[0])
			}); err != nil {
				return fmt.Errorf("use project: %w", err)
			}
			return nil
		},
	}
}

func newProjectNextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "Cycle active project",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := cycleProject()
			if err != nil {
				return fmt.Errorf("cycle project: %w", err)
			}
			_, _ = fmt.Fprintln(stdoutWriter, name)
			return nil
		},
	}
}

func newProjectRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove configured project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := withConfigStore(func(store *config.Store) error {
				return store.RemoveProject(args[0])
			}); err != nil {
				return fmt.Errorf("remove project: %w", err)
			}
			return nil
		},
	}
}

func runActive(parent context.Context, flags rootFlags) error {
	return runIssueList(parent, flags, func(ctx context.Context, service *app.Service, limit int) ([]app.IssueSummary, error) {
		return service.Active(ctx, limit)
	})
}

func runRecent(parent context.Context, flags rootFlags) error {
	return runIssueList(parent, flags, func(ctx context.Context, service *app.Service, limit int) ([]app.IssueSummary, error) {
		return service.Recent(ctx, limit)
	})
}

func runIssueList(parent context.Context, flags rootFlags, load func(context.Context, *app.Service, int) ([]app.IssueSummary, error)) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()

	service, token, err := buildService(flags)
	if err != nil {
		return err
	}

	issues, err := runWithProgress(flags.Format, "Loading issues", func() ([]app.IssueSummary, error) {
		return load(ctx, service, flags.Limit)
	})
	if err != nil {
		return sanitizeError(err, token)
	}

	jsonPayload := redact.Value(map[string]any{"issues": issues}, token)
	return printOutput(flags.Format, output.RenderIssueListHuman(issues), jsonPayload)
}

func withConfigStore(action func(*config.Store) error) error {
	store, err := newConfigStore()
	if err != nil {
		return err
	}

	return action(store)
}

func printProjects(store *config.Store) error {
	file, err := store.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if len(file.Projects) == 0 {
		_, _ = fmt.Fprintln(stdoutWriter, "no configured projects")
		return nil
	}

	for _, project := range file.Projects {
		prefix := "  "
		if project.Name == file.ActiveProject {
			prefix = "* "
		}
		_, _ = fmt.Fprintf(stdoutWriter, "%s%s\n", prefix, project.Name)
	}

	return nil
}

func cycleProject() (string, error) {
	var name string
	err := withConfigStore(func(store *config.Store) error {
		next, err := store.CycleProject()
		if err != nil {
			return fmt.Errorf("cycle project: %w", err)
		}
		name = next
		return nil
	})
	if err != nil {
		return "", err
	}

	return name, nil
}

func runShow(parent context.Context, flags rootFlags, counter domain.ItemCounter) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()

	service, token, err := buildService(flags)
	if err != nil {
		return err
	}

	detail, err := runWithProgress(flags.Format, "Loading issue detail", func() (app.IssueDetail, error) {
		return service.Show(ctx, counter)
	})
	if err != nil {
		return sanitizeError(err, token)
	}

	payload := map[string]any{
		"issue":        detail.IssueSummary,
		"main_error":   detail.MainError,
		"item_raw":     detail.ItemRaw,
		"instance":     detail.Instance,
		"instance_raw": detail.InstanceRaw,
	}
	jsonPayload := redact.Value(payload, token)

	return printOutput(flags.Format, output.RenderIssueDetailHuman(detail), jsonPayload)
}

func printOutput(format string, human string, payload any) error {
	switch format {
	case "human":
		_, _ = fmt.Fprintln(stdoutWriter, human)
		return nil
	case "json":
		rendered, err := output.RenderJSON(payload)
		if err != nil {
			return fmt.Errorf("render json: %w", err)
		}
		_, _ = fmt.Fprintln(stdoutWriter, rendered)
		return nil
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func buildService(flags rootFlags) (*app.Service, string, error) {
	token, err := resolveAccessToken(flags)
	if err != nil {
		return nil, "", err
	}

	client, err := newRollbarClient(token)
	if err != nil {
		return nil, token, sanitizeError(err, token)
	}

	return app.NewService(client), token, nil
}

func resolveAccessToken(flags rootFlags) (string, error) {
	if flags.Token != "" {
		return flags.Token, nil
	}

	store, err := newConfigStore()
	if err == nil {
		token, _, resolveErr := store.ResolveToken(flags.Project)
		if resolveErr == nil {
			return token, nil
		}
	}

	token := os.Getenv("ROLLBAR_ACCESS_TOKEN")
	if token == "" {
		if flags.Project != "" {
			return "", fmt.Errorf("project %q not configured and ROLLBAR_ACCESS_TOKEN is missing", flags.Project)
		}
		return "", errors.New("no token available: add a project via `rollbaz project add ...` or set ROLLBAR_ACCESS_TOKEN")
	}

	return token, nil
}

func sanitizeError(err error, token string) error {
	return errors.New(redact.String(err.Error(), token))
}

func runWithProgress[T any](format string, message string, operation func() (T, error)) (T, error) {
	if !shouldRenderProgress(format) {
		return operation()
	}

	writer := progress.NewWriter()
	writer.SetAutoStop(true)
	writer.SetMessageLength(28)
	writer.SetTrackerLength(18)
	writer.SetUpdateFrequency(100 * time.Millisecond)
	writer.SetOutputWriter(stdoutWriter)
	writer.Style().Visibility.ETA = false
	writer.Style().Visibility.ETAOverall = false
	writer.Style().Visibility.Percentage = false
	writer.Style().Visibility.Speed = false
	writer.Style().Visibility.SpeedOverall = false
	writer.Style().Visibility.Time = false
	writer.Style().Visibility.TrackerOverall = false
	writer.Style().Visibility.Value = false

	tracker := progress.Tracker{Message: message, Total: 0, Units: progress.UnitsDefault}
	writer.AppendTracker(&tracker)

	go writer.Render()

	result, err := operation()
	if err != nil {
		tracker.MarkAsErrored()
	} else {
		tracker.MarkAsDone()
	}

	for writer.IsRenderInProgress() {
		time.Sleep(10 * time.Millisecond)
	}

	return result, err
}

func shouldRenderProgress(format string) bool {
	if format != "human" {
		return false
	}
	if os.Getenv("CI") != "" {
		return false
	}
	file, ok := stdoutWriter.(*os.File)
	if !ok {
		return false
	}

	return term.IsTerminal(int(file.Fd()))
}
