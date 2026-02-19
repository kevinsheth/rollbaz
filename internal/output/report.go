package output

import (
	"fmt"
	"strings"

	"github.com/kevinsheth/rollbaz/internal/domain"
)

type Report struct {
	MainError   string
	Title       string
	Status      string
	Environment string
	Occurrences *uint64
	Counter     domain.ItemCounter
	ItemID      domain.ItemID
}

func RenderHuman(report Report) string {
	environment := fallback(report.Environment)
	occurrences := "unknown"
	if report.Occurrences != nil {
		occurrences = fmt.Sprintf("%d", *report.Occurrences)
	}

	lines := []string{
		"main error: " + fallback(report.MainError),
		"title: " + fallback(report.Title),
		fmt.Sprintf("status: %s | environment: %s | occurrences: %s", fallback(report.Status), environment, occurrences),
		fmt.Sprintf("counter: %s | item_id: %s", report.Counter.String(), report.ItemID.String()),
	}

	return strings.Join(lines, "\n")
}

func fallback(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown"
	}

	return trimmed
}
