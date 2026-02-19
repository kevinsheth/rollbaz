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
	defaultListRowWidth   = 120
	minListTitleWidth     = 24
	maxListTitleWidth     = 120
	listNonTitleWidth     = 74
	defaultDetailRowWidth = 120
	minDetailValueWidth   = 40
	maxDetailValueWidth   = 100
	detailNonValueWidth   = 20
)

func RenderIssueListHuman(issues []app.IssueSummary) string {
	return RenderIssueListHumanWithWidth(issues, defaultListRowWidth)
}

func RenderIssueListHumanWithWidth(issues []app.IssueSummary, maxWidth int) string {
	if len(issues) == 0 {
		return "no issues found"
	}

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	configureListTable(tw, maxWidth)
	tw.AppendHeader(table.Row{"COUNTER", "STATUS", "ENV", "OCCURRENCES", "LAST_SEEN", "TITLE"})

	for _, issue := range issues {
		tw.AppendRow(table.Row{
			issue.Counter.String(),
			fallback(issue.Status),
			fallback(issue.Environment),
			formatOccurrences(issue.Occurrences),
			formatTimestamp(issue.LastOccurrenceTimestamp),
			fallback(issue.Title),
		})
	}

	return strings.TrimRight(tw.Render(), "\n")
}

func RenderIssueDetailHuman(detail app.IssueDetail) string {
	return RenderIssueDetailHumanWithWidth(detail, defaultDetailRowWidth)
}

func RenderIssueDetailHumanWithWidth(detail app.IssueDetail, maxWidth int) string {
	valueWidth := detailValueWidth(maxWidth)
	rowWidth := normalizeWidth(maxWidth, defaultDetailRowWidth)

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.SetAllowedRowLength(rowWidth)
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 2, WidthMax: valueWidth, WidthMaxEnforcer: prettytext.Trim},
	})
	tw.AppendRow(table.Row{"Title", fallback(detail.Title)})
	tw.AppendRow(table.Row{"Status", fallback(detail.Status)})
	tw.AppendRow(table.Row{"Environment", fallback(detail.Environment)})
	tw.AppendRow(table.Row{"Occurrences", formatOccurrences(detail.Occurrences)})
	tw.AppendRow(table.Row{"Counter", detail.Counter.String()})
	tw.AppendRow(table.Row{"Item ID", detail.ItemID.String()})

	renderedTable := strings.TrimRight(tw.Render(), "\n")
	if shouldIncludeMainErrorLine(detail) {
		heading := "Main Error: " + prettytext.Trim(fallback(detail.MainError), valueWidth)
		return heading + "\n\n" + renderedTable
	}

	return renderedTable
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

func configureListTable(tw table.Writer, maxWidth int) {
	targetWidth := normalizeWidth(maxWidth, defaultListRowWidth)
	titleWidth := targetWidth - listNonTitleWidth
	if titleWidth < minListTitleWidth {
		titleWidth = minListTitleWidth
	}
	if titleWidth > maxListTitleWidth {
		titleWidth = maxListTitleWidth
	}

	tw.SetAllowedRowLength(targetWidth)
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 6, WidthMax: titleWidth, WidthMaxEnforcer: prettytext.Trim},
	})
}

func detailValueWidth(maxWidth int) int {
	targetWidth := normalizeWidth(maxWidth, defaultDetailRowWidth)
	valueWidth := targetWidth - detailNonValueWidth
	if valueWidth < minDetailValueWidth {
		valueWidth = minDetailValueWidth
	}
	if valueWidth > maxDetailValueWidth {
		valueWidth = maxDetailValueWidth
	}

	return valueWidth
}

func normalizeWidth(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}

	return value
}

func formatOccurrences(occurrences *uint64) string {
	if occurrences == nil {
		return "unknown"
	}

	return fmt.Sprintf("%d", *occurrences)
}

func shouldIncludeMainErrorLine(detail app.IssueDetail) bool {
	mainError := strings.TrimSpace(detail.MainError)
	if mainError == "" || strings.EqualFold(mainError, "unknown") {
		return false
	}

	title := strings.TrimSpace(detail.Title)
	if title == "" {
		return true
	}

	return !strings.Contains(strings.ToLower(title), strings.ToLower(mainError))
}
