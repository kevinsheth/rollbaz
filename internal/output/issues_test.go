package output

import (
	"math"
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
	if !strings.Contains(got, "269") {
		t.Fatalf("unexpected output: %q", got)
	}
	if !strings.Contains(got, "COUNTER") {
		t.Fatalf("expected table header, got: %q", got)
	}
	if !strings.Contains(got, "2023-11-14T22:13:20Z") {
		t.Fatalf("expected formatted timestamp, got: %q", got)
	}
	if strings.Contains(got, "1700000000") {
		t.Fatalf("expected formatted timestamp, got: %q", got)
	}
}

func TestRenderIssueListHumanTruncatesLongTitle(t *testing.T) {
	t.Parallel()

	longTitle := strings.Repeat("very-long-title-", 20)
	issues := []app.IssueSummary{{
		Counter: domain.ItemCounter(1),
		Title:   longTitle,
	}}

	got := RenderIssueListHuman(issues)
	if strings.Contains(got, longTitle) {
		t.Fatalf("expected truncated title, got: %q", got)
	}
	if !strings.Contains(got, "very-long-title") {
		t.Fatalf("expected visible title prefix, got: %q", got)
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
	if !strings.Contains(got, "Main Error:") {
		t.Fatalf("unexpected output: %q", got)
	}
	if !strings.Contains(got, "Counter") || !strings.Contains(got, "Item ID") {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestRenderIssueDetailHumanTruncatesMainError(t *testing.T) {
	t.Parallel()

	longError := strings.Repeat("database-timeout-", 20)
	detail := app.IssueDetail{MainError: longError}
	got := RenderIssueDetailHuman(detail)
	if strings.Contains(got, longError) {
		t.Fatalf("expected truncated main error, got: %q", got)
	}
	if !strings.Contains(got, "Main Error:") {
		t.Fatalf("expected main error heading, got: %q", got)
	}
}

func TestFormatTimestamp(t *testing.T) {
	t.Parallel()

	if got := formatTimestamp(nil); got != "unknown" {
		t.Fatalf("formatTimestamp(nil) = %q", got)
	}

	value := uint64(1700000000)
	if got := formatTimestamp(&value); got != "2023-11-14T22:13:20Z" {
		t.Fatalf("formatTimestamp() = %q", got)
	}

	overflow := uint64(math.MaxUint64)
	if got := formatTimestamp(&overflow); got != "unknown" {
		t.Fatalf("formatTimestamp(overflow) = %q", got)
	}
}
