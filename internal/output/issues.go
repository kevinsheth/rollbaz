package output

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kevinsheth/rollbaz/internal/app"
)

func RenderIssueListHuman(issues []app.IssueSummary) string {
	if len(issues) == 0 {
		return "no issues found"
	}

	lines := make([]string, 0, len(issues))
	for _, issue := range issues {
		occurrences := "unknown"
		if issue.Occurrences != nil {
			occurrences = fmt.Sprintf("%d", *issue.Occurrences)
		}
		lastSeen := "unknown"
		if issue.LastOccurrenceTimestamp != nil {
			lastSeen = fmt.Sprintf("%d", *issue.LastOccurrenceTimestamp)
		}

		lines = append(lines, fmt.Sprintf("#%s %s | %s | env=%s | occurrences=%s | last_seen=%s", issue.Counter.String(), fallback(issue.Title), fallback(issue.Status), fallback(issue.Environment), occurrences, lastSeen))
	}

	return strings.Join(lines, "\n")
}

func RenderIssueDetailHuman(detail app.IssueDetail) string {
	report := Report{
		MainError:   detail.MainError,
		Title:       detail.Title,
		Status:      detail.Status,
		Environment: detail.Environment,
		Occurrences: detail.Occurrences,
		Counter:     detail.Counter,
		ItemID:      detail.ItemID,
	}

	return RenderHuman(report)
}

func RenderJSON(value any) (string, error) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal json output: %w", err)
	}

	return string(body), nil
}
