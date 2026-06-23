package ui

import (
	"fmt"
	"strings"
)

type ExecLogEntry struct {
	Type       string
	Command    string
	Path       string
	Workdir    string
	Message    string
	Status     State
	DurationMs int64
}

func ExecLog(entry ExecLogEntry) string {
	var lines []string
	if entry.Type != "" {
		lines = append(lines, HeadingStyle.Render(entry.Type))
	}

	target := entry.Command
	if target == "" {
		target = entry.Path
	}
	if target != "" {
		if entry.Workdir != "" {
			target += MutedStyle.Render(" in " + entry.Workdir)
		}
		lines = append(lines, target)
	}

	icon, style := stateIcon(entry.Status)
	status := entry.Message
	if status == "" {
		status = string(entry.Status)
	}
	if status == "" {
		status = string(Pending)
	}

	line := style.Render(icon) + " " + status
	if showsDuration(entry) {
		line += MutedStyle.Render(fmt.Sprintf(" in %dms", entry.DurationMs))
	}
	if entry.Status == Succeeded || entry.Status == Failed {
		line += ":"
	}
	lines = append(lines, line)

	return strings.Join(lines, "\n")
}

func showsDuration(entry ExecLogEntry) bool {
	switch entry.Status {
	case Running, Succeeded, Failed:
		return true
	default:
		return entry.DurationMs > 0
	}
}
