package output

import (
	"testing"

	"github.com/kevinsheth/rollbaz/internal/domain"
)

func TestRenderHuman(t *testing.T) {
	t.Parallel()

	occurrences := uint64(42)
	report := Report{
		MainError:   "boom",
		Title:       "title",
		Status:      "active",
		Environment: "production",
		Occurrences: &occurrences,
		Counter:     domain.ItemCounter(269),
		ItemID:      domain.ItemID(1755568172),
	}

	got := RenderHuman(report)
	want := "main error: boom\n" +
		"title: title\n" +
		"status: active | environment: production | occurrences: 42\n" +
		"counter: 269 | item_id: 1755568172"

	if got != want {
		t.Fatalf("RenderHuman() = %q, want %q", got, want)
	}
}

func TestRenderHumanUnknownFallbacks(t *testing.T) {
	t.Parallel()

	got := RenderHuman(Report{Counter: domain.ItemCounter(1), ItemID: domain.ItemID(2)})
	want := "main error: unknown\n" +
		"title: unknown\n" +
		"status: unknown | environment: unknown | occurrences: unknown\n" +
		"counter: 1 | item_id: 2"

	if got != want {
		t.Fatalf("RenderHuman() = %q, want %q", got, want)
	}
}
