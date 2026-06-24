package ui

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/progress"
)

type Field struct {
	Label string
	Value string
}

type ErrorDetails struct {
	Title      string
	Reason     string
	Suggestion string
	Command    string
}

func Section(title string, fields []Field) string {
	const dividerWidth = 34
	divider := DividerStyle.Render(strings.Repeat("━", dividerWidth))
	lines := []string{divider, HeadingStyle.Render(title), divider, ""}
	lines = append(lines, renderFields(fields)...)
	return strings.Join(lines, "\n")
}

func Panel(title string, fields []Field, success bool) string {
	lines := []string{HeadingStyle.Render(title)}
	if len(fields) > 0 {
		lines = append(lines, renderFields(fields)...)
	}
	content := strings.Join(lines, "\n")
	if success {
		return SuccessPanelStyle.Render(content)
	}
	return PanelStyle.Render(content)
}

func Failure(details ErrorDetails) string {
	title := details.Title
	if title == "" {
		title = "Command Failed"
	}
	lines := []string{ErrorStyle.Render("✗") + " " + HeadingStyle.Render(title)}
	if details.Reason != "" {
		lines = append(lines, "", HeadingStyle.Render("Reason:"), details.Reason)
	}
	if details.Suggestion != "" || details.Command != "" {
		lines = append(lines, "", HeadingStyle.Render("Suggestion:"))
		if details.Suggestion != "" {
			lines = append(lines, details.Suggestion)
		}
		if details.Command != "" {
			lines = append(lines, MutedStyle.Render("Run:"), details.Command)
		}
	}
	return ErrorPanelStyle.Render(strings.Join(lines, "\n"))
}

func Progress(label string, percent float64, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	if width <= 0 {
		width = 30
	}
	bar := progress.New(
		progress.WithWidth(width),
		progress.WithSolidFill(PrimaryColor),
	)
	bar.EmptyColor = MutedColor
	bar.PercentageStyle = MutedStyle
	return HeadingStyle.Render(label) + "\n" + bar.ViewAs(percent)
}

func Table(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}
	widths := make([]int, len(headers))
	for index, header := range headers {
		widths[index] = utf8.RuneCountInString(header)
	}
	for _, row := range rows {
		for index := 0; index < len(row) && index < len(widths); index++ {
			if width := utf8.RuneCountInString(row[index]); width > widths[index] {
				widths[index] = width
			}
		}
	}

	renderRow := func(values []string, header bool) string {
		cells := make([]string, len(headers))
		for index := range headers {
			value := ""
			if index < len(values) {
				value = values[index]
			}
			cell := value + strings.Repeat(" ", widths[index]-utf8.RuneCountInString(value))
			if header {
				cells[index] = TableHeaderStyle.Render(cell)
			} else {
				cells[index] = cell
			}
		}
		return strings.TrimRight(TableCellStyle.Render(strings.Join(cells, "  ")), " ")
	}

	lines := []string{renderRow(headers, true)}
	for _, row := range rows {
		lines = append(lines, renderRow(row, false))
	}
	return strings.Join(lines, "\n")
}

func Fields(values map[string]string) []Field {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fields := make([]Field, 0, len(keys))
	for _, key := range keys {
		fields = append(fields, Field{Label: key, Value: values[key]})
	}
	return fields
}

func renderFields(fields []Field) []string {
	width := 0
	for _, field := range fields {
		if size := utf8.RuneCountInString(field.Label); size > width {
			width = size
		}
	}
	lines := make([]string, 0, len(fields))
	for _, field := range fields {
		label := fmt.Sprintf("%-*s", width+1, field.Label+":")
		lines = append(lines, MutedStyle.Render(label)+" "+field.Value)
	}
	return lines
}
