package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/kevinsheth/rollbaz/internal/domain"
	"github.com/kevinsheth/rollbaz/internal/output"
	"github.com/kevinsheth/rollbaz/internal/redact"
	"github.com/kevinsheth/rollbaz/internal/rollbar"
	"github.com/kevinsheth/rollbaz/internal/summary"
)

type rootFlags struct {
	item uint64
}

var (
	newRollbarClient           = rollbar.New
	stdoutWriter     io.Writer = os.Stdout
	stderrWriter     io.Writer = os.Stderr
)

func NewRootCmd() *cobra.Command {
	flags := &rootFlags{}

	cmd := &cobra.Command{
		Use:   "rollbaz",
		Short: "Resolve Rollbar item counters into concise summaries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), *flags)
		},
		SilenceUsage: true,
	}

	cmd.Flags().Uint64VarP(&flags.item, "item", "i", 0, "Rollbar item counter")
	_ = cmd.MarkFlagRequired("item")

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

func run(parent context.Context, flags rootFlags) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()

	token, err := readAccessToken()
	if err != nil {
		return err
	}

	client, err := newRollbarClient(token)
	if err != nil {
		return sanitizeError(err, token)
	}

	counter := domain.ItemCounter(flags.item)
	itemID, err := resolveItemID(ctx, client, counter, token)
	if err != nil {
		return err
	}

	item, instance, err := loadItemAndInstance(ctx, client, itemID, token)
	if err != nil {
		return err
	}

	writeSummary(counter, itemID, item, instance)
	return nil
}

func resolveItemID(ctx context.Context, client *rollbar.Client, counter domain.ItemCounter, token string) (domain.ItemID, error) {
	itemID, err := client.ResolveItemIDByCounter(ctx, counter)
	if err != nil {
		return 0, sanitizeError(err, token)
	}

	return itemID, nil
}

func writeSummary(counter domain.ItemCounter, itemID domain.ItemID, item rollbar.Item, instance *rollbar.ItemInstance) {
	mainError := "unknown"
	if instance != nil {
		mainError = summary.MainError(instance.Body, instance.Data)
	}

	report := output.Report{
		MainError:   mainError,
		Title:       item.Title,
		Status:      item.Status,
		Environment: item.Environment,
		Occurrences: item.TotalOccurrences,
		Counter:     counter,
		ItemID:      itemID,
	}

	_, _ = fmt.Fprintln(stdoutWriter, output.RenderHuman(report))
}

func loadItemAndInstance(ctx context.Context, client *rollbar.Client, itemID domain.ItemID, token string) (rollbar.Item, *rollbar.ItemInstance, error) {
	var (
		item     rollbar.Item
		instance *rollbar.ItemInstance
		itemErr  error
		instErr  error
		wg       sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		item, itemErr = client.GetItem(ctx, itemID)
	}()

	go func() {
		defer wg.Done()
		instance, instErr = client.GetLatestInstance(ctx, itemID)
	}()

	wg.Wait()

	if itemErr != nil {
		return rollbar.Item{}, nil, sanitizeError(itemErr, token)
	}

	if instErr != nil {
		return rollbar.Item{}, nil, sanitizeError(instErr, token)
	}

	return item, instance, nil
}

func readAccessToken() (string, error) {
	token := os.Getenv("ROLLBAR_ACCESS_TOKEN")
	if token == "" {
		return "", errors.New("ROLLBAR_ACCESS_TOKEN is required")
	}

	return token, nil
}

func sanitizeError(err error, token string) error {
	return errors.New(redact.String(err.Error(), token))
}
