package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/kevinsheth/rollbaz/internal/domain"
	"github.com/kevinsheth/rollbaz/internal/rollbar"
	"github.com/kevinsheth/rollbaz/internal/summary"
)

type RollbarAPI interface {
	ResolveItemIDByCounter(ctx context.Context, counter domain.ItemCounter) (domain.ItemID, error)
	GetItem(ctx context.Context, itemID domain.ItemID) (rollbar.Item, error)
	GetLatestInstance(ctx context.Context, itemID domain.ItemID) (*rollbar.ItemInstance, error)
	ListActiveItems(ctx context.Context, limit int) ([]rollbar.Item, error)
	ListItems(ctx context.Context, status string, page int) ([]rollbar.Item, error)
}

type Service struct {
	api RollbarAPI
}

func NewService(api RollbarAPI) *Service {
	return &Service{api: api}
}

type IssueSummary struct {
	ItemID                  domain.ItemID      `json:"item_id"`
	Counter                 domain.ItemCounter `json:"counter"`
	Title                   string             `json:"title"`
	Status                  string             `json:"status"`
	Environment             string             `json:"environment"`
	LastOccurrenceTimestamp *uint64            `json:"last_occurrence_timestamp,omitempty"`
	Occurrences             *uint64            `json:"occurrences,omitempty"`
	Raw                     json.RawMessage    `json:"raw,omitempty"`
}

type IssueDetail struct {
	IssueSummary
	MainError   string                `json:"main_error"`
	ItemRaw     json.RawMessage       `json:"item_raw,omitempty"`
	Instance    *rollbar.ItemInstance `json:"instance,omitempty"`
	InstanceRaw json.RawMessage       `json:"instance_raw,omitempty"`
}

func (s *Service) Active(ctx context.Context, limit int) ([]IssueSummary, error) {
	items, err := s.api.ListActiveItems(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list active items: %w", err)
	}

	return mapSummaries(items), nil
}

func (s *Service) Recent(ctx context.Context, limit int) ([]IssueSummary, error) {
	items, err := s.api.ListItems(ctx, "active", 1)
	if err != nil {
		return nil, fmt.Errorf("list recent items: %w", err)
	}

	sort.SliceStable(items, func(i int, j int) bool {
		leftTS := uint64(0)
		if items[i].LastOccurrenceTimestamp != nil {
			leftTS = *items[i].LastOccurrenceTimestamp
		}
		rightTS := uint64(0)
		if items[j].LastOccurrenceTimestamp != nil {
			rightTS = *items[j].LastOccurrenceTimestamp
		}
		if leftTS != rightTS {
			return leftTS > rightTS
		}

		leftOccurrence := uint64(0)
		if items[i].LastOccurrenceID != nil {
			leftOccurrence = *items[i].LastOccurrenceID
		}
		rightOccurrence := uint64(0)
		if items[j].LastOccurrenceID != nil {
			rightOccurrence = *items[j].LastOccurrenceID
		}

		return leftOccurrence > rightOccurrence
	})

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	return mapSummaries(items), nil
}

func (s *Service) Show(ctx context.Context, counter domain.ItemCounter) (IssueDetail, error) {
	itemID, err := s.api.ResolveItemIDByCounter(ctx, counter)
	if err != nil {
		return IssueDetail{}, fmt.Errorf("resolve item id: %w", err)
	}

	item, err := s.api.GetItem(ctx, itemID)
	if err != nil {
		return IssueDetail{}, fmt.Errorf("get item: %w", err)
	}

	instance, err := s.api.GetLatestInstance(ctx, itemID)
	if err != nil {
		return IssueDetail{}, fmt.Errorf("get latest instance: %w", err)
	}

	mainError := "unknown"
	instanceRaw := json.RawMessage(nil)
	if instance != nil {
		mainError = summary.MainError(instance.Body, instance.Data)
		instanceRaw = instance.Raw
	}
	if mainError == "unknown" && strings.TrimSpace(item.Title) != "" {
		mainError = item.Title
	}

	return IssueDetail{
		IssueSummary: mapSummary(item),
		MainError:    mainError,
		ItemRaw:      item.Raw,
		Instance:     instance,
		InstanceRaw:  instanceRaw,
	}, nil
}

func mapSummaries(items []rollbar.Item) []IssueSummary {
	summaries := make([]IssueSummary, 0, len(items))
	for _, item := range items {
		summaries = append(summaries, mapSummary(item))
	}

	return summaries
}

func mapSummary(item rollbar.Item) IssueSummary {
	occurrences := item.TotalOccurrences
	if occurrences == nil {
		occurrences = item.Occurrences
	}

	return IssueSummary{
		ItemID:                  item.ID,
		Counter:                 domain.ItemCounter(item.Counter),
		Title:                   item.Title,
		Status:                  item.Status,
		Environment:             item.Environment,
		LastOccurrenceTimestamp: item.LastOccurrenceTimestamp,
		Occurrences:             occurrences,
		Raw:                     item.Raw,
	}
}
