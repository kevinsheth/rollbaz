package output

import (
	"strings"
	"testing"

	"github.com/kevinsheth/rollbaz/internal/app"
	"github.com/kevinsheth/rollbaz/internal/domain"
)

func TestRenderIssueListHuman(t *testing.T) {
	t.Parallel()

	occurrences := uint64(10)
	lastSeen := uint64(1700000000)
	issues := []app.IssueSummary{{
		Counter:                 domain.ItemCounter(269),
		Title:                   "RST_STREAM",
		Status:                  "active",
		Environment:             "production",
		Occurrences:             &occurrences,
		LastOccurrenceTimestamp: &lastSeen,
	}}

	got := RenderIssueListHuman(issues)
	if !strings.Contains(got, "#269") {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestRenderJSON(t *testing.T) {
	t.Parallel()

	rendered, err := RenderJSON(map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("RenderJSON() error = %v", err)
	}
	if !strings.Contains(rendered, `"x": 1`) {
		t.Fatalf("unexpected json output: %q", rendered)
	}
}

func TestRenderIssueDetailHuman(t *testing.T) {
	t.Parallel()

	detail := app.IssueDetail{IssueSummary: app.IssueSummary{Counter: domain.ItemCounter(1), ItemID: domain.ItemID(2)}}
	got := RenderIssueDetailHuman(detail)
	if !strings.Contains(got, "counter: 1 | item_id: 2") {
		t.Fatalf("unexpected output: %q", got)
	}
}
