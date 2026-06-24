// Package util holds pure helpers for the interactive TUI: text shaping,
// clipboard, and overlay compositing. It imports nothing from internal/tui/*.
package util

import (
	"path/filepath"
	"strings"
)

// Truncate shortens value to width runes, adding an ellipsis when it overflows.
func Truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

// Wrap word-wraps value to the given width.
func Wrap(value string, width int) string {
	if width <= 1 {
		return value
	}
	words := strings.Fields(value)
	var lines []string
	var line string
	for _, word := range words {
		if line != "" && len([]rune(line))+1+len([]rune(word)) > width {
			lines = append(lines, line)
			line = word
		} else if line == "" {
			line = word
		} else {
			line += " " + word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// TruncateLines wraps value and keeps at most maxLines lines from the top.
func TruncateLines(value string, width, maxLines int) string {
	var lines []string
	for _, source := range strings.Split(value, "\n") {
		for _, line := range strings.Split(Wrap(source, width), "\n") {
			lines = append(lines, line)
			if len(lines) == maxLines {
				return strings.Join(lines, "\n")
			}
		}
	}
	return strings.Join(lines, "\n")
}

// LastLines wraps value and keeps at most maxLines lines from the bottom.
func LastLines(value string, width, maxLines int) string {
	source := strings.Split(strings.TrimSpace(value), "\n")
	var lines []string
	for _, item := range source {
		lines = append(lines, strings.Split(Wrap(item, width), "\n")...)
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n")
}

// StripANSI removes CSI escape sequences from value.
func StripANSI(value string) string {
	var clean strings.Builder
	for index := 0; index < len(value); {
		if value[index] == '\x1b' && index+1 < len(value) && value[index+1] == '[' {
			index += 2
			for index < len(value) {
				character := value[index]
				index++
				if character >= 0x40 && character <= 0x7e {
					break
				}
			}
			continue
		}
		clean.WriteByte(value[index])
		index++
	}
	return clean.String()
}

// CompactPath shortens a filesystem path to its last keep segments.
func CompactPath(path string, keep int) string {
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	if len(parts) <= keep {
		return path
	}
	return "…/" + strings.Join(parts[len(parts)-keep:], "/")
}

// Max returns the larger of a and b.
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Min returns the smaller of a and b.
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
