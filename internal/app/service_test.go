package app

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/kevinsheth/rollbaz/internal/domain"
	"github.com/kevinsheth/rollbaz/internal/rollbar"
)

type fakeAPI struct {
	activeItems []rollbar.Item
	listItems   []rollbar.Item
	item        rollbar.Item
	instance    *rollbar.ItemInstance
	err         error
}

func (f fakeAPI) ResolveItemIDByCounter(ctx context.Context, counter domain.ItemCounter) (domain.ItemID, error) {
	if f.err != nil {
		return 0, f.err
	}
	return domain.ItemID(123), nil
}

func (f fakeAPI) GetItem(ctx context.Context, itemID domain.ItemID) (rollbar.Item, error) {
	if f.err != nil {
		return rollbar.Item{}, f.err
	}
	return f.item, nil
}

func (f fakeAPI) UpdateItem(ctx context.Context, itemID domain.ItemID, patch rollbar.ItemPatch) error {
	if f.err != nil {
		return f.err
	}

	return nil
}

func (f fakeAPI) GetLatestInstance(ctx context.Context, itemID domain.ItemID) (*rollbar.ItemInstance, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.instance, nil
}

func (f fakeAPI) ListActiveItems(ctx context.Context, limit int) ([]rollbar.Item, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.activeItems, nil
}

func (f fakeAPI) ListItems(ctx context.Context, status string, page int) ([]rollbar.Item, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.listItems, nil
}

func TestServiceActive(t *testing.T) {
	t.Parallel()

	occurrences := uint64(3)
	service := NewService(fakeAPI{activeItems: []rollbar.Item{{ID: 1, Counter: 7, Title: "x", TotalOccurrences: &occurrences}}})

	issues, err := service.Active(context.Background(), 10, IssueFilters{})
	if err != nil {
		t.Fatalf("Active() error = %v", err)
	}
	if len(issues) != 1 || issues[0].Counter != domain.ItemCounter(7) {
		t.Fatalf("unexpected active issues: %+v", issues)
	}
}

func TestServiceRecentSortsByTimestamp(t *testing.T) {
	t.Parallel()

	ts1 := uint64(100)
	ts2 := uint64(200)

	service := NewService(fakeAPI{listItems: []rollbar.Item{
		{ID: 1, Counter: 1, LastOccurrenceTimestamp: &ts1},
		{ID: 2, Counter: 2, LastOccurrenceTimestamp: &ts2},
	}})

	issues, err := service.Recent(context.Background(), 10, IssueFilters{})
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(issues) < 2 || issues[0].Counter != 2 {
		t.Fatalf("unexpected sort order: %+v", issues)
	}
}

func TestServiceRecentSortsByTimestampThenTotalOccurrences(t *testing.T) {
	t.Parallel()

	ts := uint64(100)
	totalLow := uint64(10)
	totalHigh := uint64(20)

	service := NewService(fakeAPI{listItems: []rollbar.Item{
		{ID: 1, Counter: 1, LastOccurrenceTimestamp: &ts, TotalOccurrences: &totalLow},
		{ID: 2, Counter: 2, LastOccurrenceTimestamp: &ts, TotalOccurrences: &totalHigh},
	}})

	issues, err := service.Recent(context.Background(), 10, IssueFilters{})
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(issues) < 2 || issues[0].Counter != 2 {
		t.Fatalf("unexpected sort order: %+v", issues)
	}
}

func TestServiceRecentLimitAndOccurrenceFallback(t *testing.T) {
	t.Parallel()

	ts := uint64(100)
	totalLow := uint64(10)
	fallbackHigh := uint64(20)
	ignoredOccurrence := uint64(999)

	service := NewService(fakeAPI{listItems: []rollbar.Item{
		{ID: 1, Counter: 1, LastOccurrenceTimestamp: &ts, Occurrences: &fallbackHigh},
		{ID: 2, Counter: 2, LastOccurrenceTimestamp: &ts, TotalOccurrences: &totalLow, Occurrences: &ignoredOccurrence},
	}})

	issues, err := service.Recent(context.Background(), 1, IssueFilters{})
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(issues) != 1 || issues[0].Counter != 1 {
		t.Fatalf("unexpected fallback sort/limit: %+v", issues)
	}
}

func TestServiceShow(t *testing.T) {
	t.Parallel()

	occurrences := uint64(4)
	instance := &rollbar.ItemInstance{Data: json.RawMessage(`{"trace":{"exception":{"description":"boom"}}}`), Raw: json.RawMessage(`{"id":1}`)}
	item := rollbar.Item{ID: 123, Counter: 9, Title: "title", TotalOccurrences: &occurrences, Raw: json.RawMessage(`{"id":123}`)}

	service := NewService(fakeAPI{item: item, instance: instance})

	detail, err := service.Show(context.Background(), 9)
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if detail.MainError != "boom" {
		t.Fatalf("Show() main error = %q", detail.MainError)
	}
}

func TestServiceErrors(t *testing.T) {
	t.Parallel()

	service := NewService(fakeAPI{err: errors.New("bad")})
	if _, err := service.Active(context.Background(), 1, IssueFilters{}); err == nil {
		t.Fatalf("expected Active error")
	}
	if _, err := service.Recent(context.Background(), 1, IssueFilters{}); err == nil {
		t.Fatalf("expected Recent error")
	}
	if _, err := service.Show(context.Background(), 1); err == nil {
		t.Fatalf("expected Show error")
	}
}

func TestServiceShowWithoutInstance(t *testing.T) {
	t.Parallel()

	service := NewService(fakeAPI{item: rollbar.Item{ID: 1, Counter: 2, Title: "x"}})
	detail, err := service.Show(context.Background(), 2)
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if detail.MainError != "x" {
		t.Fatalf("expected title fallback main error, got %q", detail.MainError)
	}
}

func TestServiceShowWithoutInstanceOrTitle(t *testing.T) {
	t.Parallel()

	service := NewService(fakeAPI{item: rollbar.Item{ID: 1, Counter: 2}})
	detail, err := service.Show(context.Background(), 2)
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if detail.MainError != "unknown" {
		t.Fatalf("expected unknown main error, got %q", detail.MainError)
	}
}

type actionAPI struct {
	resolvedID  domain.ItemID
	item        rollbar.Item
	resolveErr  error
	updateErr   error
	getItemErr  error
	updateCalls int
	lastPatch   rollbar.ItemPatch
}

func (a *actionAPI) ResolveItemIDByCounter(ctx context.Context, counter domain.ItemCounter) (domain.ItemID, error) {
	if a.resolveErr != nil {
		return 0, a.resolveErr
	}

	return a.resolvedID, nil
}

func (a *actionAPI) GetItem(ctx context.Context, itemID domain.ItemID) (rollbar.Item, error) {
	if a.getItemErr != nil {
		return rollbar.Item{}, a.getItemErr
	}

	return a.item, nil
}

func (a *actionAPI) UpdateItem(ctx context.Context, itemID domain.ItemID, patch rollbar.ItemPatch) error {
	a.lastPatch = patch
	a.updateCalls++
	if a.updateErr != nil {
		return a.updateErr
	}

	return nil
}

func (a *actionAPI) GetLatestInstance(ctx context.Context, itemID domain.ItemID) (*rollbar.ItemInstance, error) {
	return nil, nil
}

func (a *actionAPI) ListActiveItems(ctx context.Context, limit int) ([]rollbar.Item, error) {
	return nil, nil
}

func (a *actionAPI) ListItems(ctx context.Context, status string, page int) ([]rollbar.Item, error) {
	return nil, nil
}

func TestServiceResolve(t *testing.T) {
	t.Parallel()

	api := &actionAPI{
		resolvedID: 99,
		item:       rollbar.Item{ID: 99, Counter: 7, Status: "resolved", Title: "x"},
	}
	service := NewService(api)

	result, err := service.Resolve(context.Background(), 7, "v1.2.3")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if result.Action != "resolved" || result.Issue.Status != "resolved" {
		t.Fatalf("unexpected resolve result: %+v", result)
	}
	if api.updateCalls != 1 {
		t.Fatalf("expected one update call, got %d", api.updateCalls)
	}
	if api.lastPatch.Status != "resolved" || api.lastPatch.ResolvedInVersion != "v1.2.3" {
		t.Fatalf("unexpected resolve patch: %+v", api.lastPatch)
	}
}

func TestServiceResolveRejectsLongResolvedVersion(t *testing.T) {
	t.Parallel()

	api := &actionAPI{resolvedID: 99, item: rollbar.Item{ID: 99, Counter: 7, Status: "resolved"}}
	service := NewService(api)
	tooLong := strings.Repeat("a", maxResolvedVersionLength+1)

	if _, err := service.Resolve(context.Background(), 7, tooLong); err == nil {
		t.Fatalf("expected resolved version length error")
	}
	if api.updateCalls != 0 {
		t.Fatalf("expected no update call, got %d", api.updateCalls)
	}
}

func TestServiceReopen(t *testing.T) {
	t.Parallel()

	api := &actionAPI{
		resolvedID: 99,
		item:       rollbar.Item{ID: 99, Counter: 8, Status: "active", Title: "x"},
	}
	service := NewService(api)

	result, err := service.Reopen(context.Background(), 8)
	if err != nil {
		t.Fatalf("Reopen() error = %v", err)
	}
	if result.Action != "reopened" || result.Issue.Status != "active" {
		t.Fatalf("unexpected reopen result: %+v", result)
	}
	if api.lastPatch.Status != "active" {
		t.Fatalf("unexpected reopen patch: %+v", api.lastPatch)
	}
}

func TestServiceMute(t *testing.T) {
	t.Parallel()

	duration := int64(3600)
	api := &actionAPI{
		resolvedID: 99,
		item:       rollbar.Item{ID: 99, Counter: 9, Status: "muted", Title: "x"},
	}
	service := NewService(api)

	result, err := service.Mute(context.Background(), 9, &duration)
	if err != nil {
		t.Fatalf("Mute() error = %v", err)
	}
	if result.Action != "muted" || result.Issue.Status != "muted" {
		t.Fatalf("unexpected mute result: %+v", result)
	}
	if api.lastPatch.Status != "muted" || api.lastPatch.SnoozeExpirationInSeconds == nil || *api.lastPatch.SnoozeExpirationInSeconds != duration {
		t.Fatalf("unexpected mute patch: %+v", api.lastPatch)
	}
	if api.lastPatch.SnoozeEnabled == nil || !*api.lastPatch.SnoozeEnabled {
		t.Fatalf("expected snooze_enabled true, got %+v", api.lastPatch)
	}
}

func TestServiceItemActionsErrors(t *testing.T) {
	t.Parallel()

	api := &actionAPI{resolvedID: 99, item: rollbar.Item{ID: 99, Counter: 7}}
	service := NewService(api)

	api.resolveErr = errors.New("resolve")
	if _, err := service.Resolve(context.Background(), 7, ""); err == nil {
		t.Fatalf("expected resolve id error")
	}

	api.resolveErr = nil
	api.updateErr = errors.New("update")
	if _, err := service.Reopen(context.Background(), 7); err == nil {
		t.Fatalf("expected update error")
	}

	api.updateErr = nil
	api.getItemErr = errors.New("get")
	if _, err := service.Mute(context.Background(), 7, nil); err == nil {
		t.Fatalf("expected get item error")
	}
}

func TestServiceActiveFilters(t *testing.T) {
	t.Parallel()

	ts := uint64(1771495200)
	occurrences := uint64(7)
	service := NewService(fakeAPI{activeItems: []rollbar.Item{{
		ID:                      1,
		Counter:                 11,
		Environment:             "production",
		Status:                  "active",
		LastOccurrenceTimestamp: &ts,
		TotalOccurrences:        &occurrences,
	}}})

	since := time.Date(2026, 2, 19, 9, 0, 0, 0, time.UTC)
	min := uint64(5)
	issues, err := service.Active(context.Background(), 10, IssueFilters{Environment: "production", Status: "active", Since: &since, MinOccurrences: &min})
	if err != nil {
		t.Fatalf("Active() error = %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected one filtered issue, got %d", len(issues))
	}

	wrongEnv, err := service.Active(context.Background(), 10, IssueFilters{Environment: "development"})
	if err != nil {
		t.Fatalf("Active() error = %v", err)
	}
	if len(wrongEnv) != 0 {
		t.Fatalf("expected zero issues for mismatched env, got %d", len(wrongEnv))
	}
}

func TestServiceActiveFiltersRejectOverflowTimestamp(t *testing.T) {
	t.Parallel()

	overflowTS := uint64(math.MaxUint64)
	service := NewService(fakeAPI{activeItems: []rollbar.Item{{
		ID:                      1,
		Counter:                 12,
		Environment:             "production",
		Status:                  "active",
		LastOccurrenceTimestamp: &overflowTS,
	}}})

	since := time.Date(2026, 2, 19, 0, 0, 0, 0, time.UTC)
	issues, err := service.Active(context.Background(), 10, IssueFilters{Since: &since})
	if err != nil {
		t.Fatalf("Active() error = %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected overflow timestamp to be filtered out, got %d issues", len(issues))
	}
}
