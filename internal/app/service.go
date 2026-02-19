package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/kevinsheth/rollbaz/internal/domain"
	"github.com/kevinsheth/rollbaz/internal/rollbar"
	"github.com/kevinsheth/rollbaz/internal/summary"
)

type RollbarAPI interface {
	ResolveItemIDByCounter(ctx context.Context, counter domain.ItemCounter) (domain.ItemID, error)
	GetItem(ctx context.Context, itemID domain.ItemID) (rollbar.Item, error)
	UpdateItem(ctx context.Context, itemID domain.ItemID, patch rollbar.ItemPatch) error
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

type IssueFilters struct {
	Environment    string
	Status         string
	Since          *time.Time
	Until          *time.Time
	MinOccurrences *uint64
	MaxOccurrences *uint64
}

const maxResolvedVersionLength = 40

type ItemActionResult struct {
	Action string       `json:"action"`
	Issue  IssueSummary `json:"issue"`
}

func (s *Service) Active(ctx context.Context, limit int, filters IssueFilters) ([]IssueSummary, error) {
	items, err := s.api.ListActiveItems(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list active items: %w", err)
	}
	items = filterItems(items, filters)

	return mapSummaries(items), nil
}

func (s *Service) Recent(ctx context.Context, limit int, filters IssueFilters) ([]IssueSummary, error) {
	items, err := s.api.ListItems(ctx, "active", 1)
	if err != nil {
		return nil, fmt.Errorf("list recent items: %w", err)
	}
	items = filterItems(items, filters)

	sort.SliceStable(items, func(i int, j int) bool {
		leftTS := uint64Value(items[i].LastOccurrenceTimestamp)
		rightTS := uint64Value(items[j].LastOccurrenceTimestamp)
		if leftTS != rightTS {
			return leftTS > rightTS
		}

		leftOccurrence := totalOccurrences(items[i])
		rightOccurrence := totalOccurrences(items[j])

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

func (s *Service) Resolve(ctx context.Context, counter domain.ItemCounter, resolvedInVersion string) (ItemActionResult, error) {
	trimmedVersion := strings.TrimSpace(resolvedInVersion)
	if len(trimmedVersion) > maxResolvedVersionLength {
		return ItemActionResult{}, fmt.Errorf("resolved_in_version must be <= %d characters", maxResolvedVersionLength)
	}

	patch := rollbar.ItemPatch{Status: "resolved", ResolvedInVersion: trimmedVersion}

	return s.updateItemAndFetch(ctx, counter, patch, "resolved")
}

func (s *Service) Reopen(ctx context.Context, counter domain.ItemCounter) (ItemActionResult, error) {
	return s.updateItemAndFetch(ctx, counter, rollbar.ItemPatch{Status: "active"}, "reopened")
}

func (s *Service) Mute(ctx context.Context, counter domain.ItemCounter, durationSeconds *int64) (ItemActionResult, error) {
	snoozeEnabled := true
	patch := rollbar.ItemPatch{Status: "muted", SnoozeEnabled: &snoozeEnabled, SnoozeExpirationInSeconds: durationSeconds}

	return s.updateItemAndFetch(ctx, counter, patch, "muted")
}

func (s *Service) updateItemAndFetch(ctx context.Context, counter domain.ItemCounter, patch rollbar.ItemPatch, action string) (ItemActionResult, error) {
	itemID, err := s.api.ResolveItemIDByCounter(ctx, counter)
	if err != nil {
		return ItemActionResult{}, fmt.Errorf("resolve item id: %w", err)
	}

	if err := s.api.UpdateItem(ctx, itemID, patch); err != nil {
		return ItemActionResult{}, fmt.Errorf("update item: %w", err)
	}

	item, err := s.api.GetItem(ctx, itemID)
	if err != nil {
		return ItemActionResult{}, fmt.Errorf("get item: %w", err)
	}

	return ItemActionResult{Action: action, Issue: mapSummary(item)}, nil
}

func mapSummaries(items []rollbar.Item) []IssueSummary {
	summaries := make([]IssueSummary, 0, len(items))
	for _, item := range items {
		summaries = append(summaries, mapSummary(item))
	}

	return summaries
}

func filterItems(items []rollbar.Item, filters IssueFilters) []rollbar.Item {
	normalized := normalizeIssueFilters(filters)
	if !hasIssueFilters(normalized) {
		return items
	}

	sinceUnix, untilUnix := unixBounds(normalized)

	filtered := make([]rollbar.Item, 0, len(items))
	for _, item := range items {
		if !matchesTextFilter(strings.TrimSpace(item.Environment), normalized.Environment) {
			continue
		}
		if !matchesTextFilter(strings.TrimSpace(item.Status), normalized.Status) {
			continue
		}
		if !matchesTimeFilter(item.LastOccurrenceTimestamp, sinceUnix, untilUnix) {
			continue
		}
		if !matchesOccurrenceFilter(item, normalized.MinOccurrences, normalized.MaxOccurrences) {
			continue
		}
		filtered = append(filtered, item)
	}

	return filtered
}

func hasIssueFilters(filters IssueFilters) bool {
	return filters.Environment != "" || filters.Status != "" || filters.Since != nil || filters.Until != nil || filters.MinOccurrences != nil || filters.MaxOccurrences != nil
}

func normalizeIssueFilters(filters IssueFilters) IssueFilters {
	filters.Environment = strings.TrimSpace(filters.Environment)
	filters.Status = strings.TrimSpace(filters.Status)

	return filters
}

func unixBounds(filters IssueFilters) (*int64, *int64) {
	var sinceUnix *int64
	if filters.Since != nil {
		value := filters.Since.Unix()
		sinceUnix = &value
	}

	var untilUnix *int64
	if filters.Until != nil {
		value := filters.Until.Unix()
		untilUnix = &value
	}

	return sinceUnix, untilUnix
}

func matchesTextFilter(value string, filter string) bool {
	if filter == "" {
		return true
	}

	return strings.EqualFold(value, filter)
}

func matchesTimeFilter(timestamp *uint64, sinceUnix *int64, untilUnix *int64) bool {
	if sinceUnix == nil && untilUnix == nil {
		return true
	}
	if timestamp == nil {
		return false
	}
	if *timestamp > math.MaxInt64 {
		return false
	}

	timestampUnix := int64(*timestamp)
	if sinceUnix != nil && timestampUnix < *sinceUnix {
		return false
	}
	if untilUnix != nil && timestampUnix > *untilUnix {
		return false
	}

	return true
}

func matchesOccurrenceFilter(item rollbar.Item, minOccurrences *uint64, maxOccurrences *uint64) bool {
	occurrences := totalOccurrences(item)
	if minOccurrences != nil && occurrences < *minOccurrences {
		return false
	}
	if maxOccurrences != nil && occurrences > *maxOccurrences {
		return false
	}

	return true
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

func totalOccurrences(item rollbar.Item) uint64 {
	if item.TotalOccurrences != nil {
		return *item.TotalOccurrences
	}

	return uint64Value(item.Occurrences)
}

func uint64Value(value *uint64) uint64 {
	if value == nil {
		return 0
	}

	return *value
}
