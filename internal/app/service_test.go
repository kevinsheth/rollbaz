package app

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

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

	issues, err := service.Active(context.Background(), 10)
	if err != nil {
		t.Fatalf("Active() error = %v", err)
	}
	if len(issues) != 1 || issues[0].Counter != domain.ItemCounter(7) {
		t.Fatalf("unexpected active issues: %+v", issues)
	}
}

func TestServiceRecentSortsByTimestampThenOccurrence(t *testing.T) {
	t.Parallel()

	ts1 := uint64(100)
	ts2 := uint64(200)
	o1 := uint64(10)
	o2 := uint64(20)

	service := NewService(fakeAPI{listItems: []rollbar.Item{
		{ID: 1, Counter: 1, LastOccurrenceTimestamp: &ts1, LastOccurrenceID: &o1},
		{ID: 2, Counter: 2, LastOccurrenceTimestamp: &ts2, LastOccurrenceID: &o2},
	}})

	issues, err := service.Recent(context.Background(), 10)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(issues) < 2 || issues[0].Counter != 2 {
		t.Fatalf("unexpected sort order: %+v", issues)
	}
}

func TestServiceRecentLimitAndOccurrenceFallback(t *testing.T) {
	t.Parallel()

	o1 := uint64(10)
	o2 := uint64(20)

	service := NewService(fakeAPI{listItems: []rollbar.Item{
		{ID: 1, Counter: 1, LastOccurrenceID: &o1},
		{ID: 2, Counter: 2, LastOccurrenceID: &o2},
	}})

	issues, err := service.Recent(context.Background(), 1)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(issues) != 1 || issues[0].Counter != 2 {
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
	if _, err := service.Active(context.Background(), 1); err == nil {
		t.Fatalf("expected Active error")
	}
	if _, err := service.Recent(context.Background(), 1); err == nil {
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
	if detail.MainError != "unknown" {
		t.Fatalf("expected unknown main error, got %q", detail.MainError)
	}
}
