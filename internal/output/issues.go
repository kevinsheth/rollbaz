package output

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	prettytext "github.com/jedib0t/go-pretty/v6/text"

	"github.com/kevinsheth/rollbaz/internal/app"
)

const (
	maxListTitleWidth     = 88
	maxDetailValueWidth   = 100
	maxMainErrorLineWidth = 120
)

func RenderIssueListHuman(issues []app.IssueSummary) string {
	if len(issues) == 0 {
		return "no issues found"
	}

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	configureListTable(tw)
	tw.AppendHeader(table.Row{"COUNTER", "STATUS", "ENV", "OCCURRENCES", "LAST_SEEN", "TITLE"})

	for _, issue := range issues {
		occurrences := "unknown"
		if issue.Occurrences != nil {
			occurrences = fmt.Sprintf("%d", *issue.Occurrences)
		}

		tw.AppendRow(table.Row{
			issue.Counter.String(),
			fallback(issue.Status),
			fallback(issue.Environment),
			occurrences,
			formatTimestamp(issue.LastOccurrenceTimestamp),
			fallback(issue.Title),
		})
	}

	return strings.TrimRight(tw.Render(), "\n")
}

func RenderIssueDetailHuman(detail app.IssueDetail) string {
	occurrences := "unknown"
	if detail.Occurrences != nil {
		occurrences = fmt.Sprintf("%d", *detail.Occurrences)
	}

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.SetColumnConfigs([]table.ColumnConfig{{Number: 2, WidthMax: maxDetailValueWidth, WidthMaxEnforcer: prettytext.Trim}})
	tw.AppendRow(table.Row{"Title", fallback(detail.Title)})
	tw.AppendRow(table.Row{"Status", fallback(detail.Status)})
	tw.AppendRow(table.Row{"Environment", fallback(detail.Environment)})
	tw.AppendRow(table.Row{"Occurrences", occurrences})
	tw.AppendRow(table.Row{"Counter", detail.Counter.String()})
	tw.AppendRow(table.Row{"Item ID", detail.ItemID.String()})

	heading := "Main Error: " + prettytext.Trim(fallback(detail.MainError), maxMainErrorLineWidth)
	return heading + "\n\n" + strings.TrimRight(tw.Render(), "\n")
}

func RenderJSON(value any) (string, error) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal json output: %w", err)
	}

	return string(body), nil
}

func formatTimestamp(unixSeconds *uint64) string {
	if unixSeconds == nil {
		return "unknown"
	}
	if *unixSeconds > math.MaxInt64 {
		return "unknown"
	}

	return time.Unix(int64(*unixSeconds), 0).UTC().Format(time.RFC3339)
}

func configureListTable(tw table.Writer) {
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 6, WidthMax: maxListTitleWidth, WidthMaxEnforcer: prettytext.Trim},
	})
}
