package util

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// JoinWithGap interleaves columns with gap spaces of horizontal padding.
func JoinWithGap(columns []string, gap int) []string {
	if len(columns) < 2 {
		return columns
	}
	result := make([]string, 0, len(columns)*2-1)
	for index, column := range columns {
		if index > 0 {
			result = append(result, strings.Repeat(" ", gap))
		}
		result = append(result, column)
	}
	return result
}

// Overlay composites fg centered over bg, replacing the background cells that fg
// covers. It is a line-based compositor (no true alpha) good enough for opaque
// modal dialogs over a static page.
func Overlay(bg, fg string, totalWidth, totalHeight int) string {
	fgLines := strings.Split(fg, "\n")
	fgHeight := len(fgLines)
	fgWidth := lipgloss.Width(fg)

	bgLines := strings.Split(bg, "\n")
	// Pad background to full height so positioning is stable.
	for len(bgLines) < totalHeight {
		bgLines = append(bgLines, "")
	}

	top := (totalHeight - fgHeight) / 2
	if top < 0 {
		top = 0
	}
	left := (totalWidth - fgWidth) / 2
	if left < 0 {
		left = 0
	}

	for i, fgLine := range fgLines {
		row := top + i
		if row < 0 || row >= len(bgLines) {
			continue
		}
		bgLine := bgLines[row]
		bgw := lipgloss.Width(bgLine)
		// Right-pad the background line so the overlay sits at a stable column.
		if bgw < left {
			bgLine += strings.Repeat(" ", left-bgw)
			bgw = left
		}
		leftPart := truncateANSIWidth(bgLine, left)
		rightStart := left + fgWidth
		rightPart := ""
		if bgw > rightStart {
			rightPart = strings.Repeat(" ", 0) + cutANSIWidth(bgLine, rightStart)
		}
		bgLines[row] = leftPart + fgLine + rightPart
	}
	return strings.Join(bgLines, "\n")
}

// truncateANSIWidth returns the prefix of s up to width display cells, padding
// with spaces if s is shorter.
func truncateANSIWidth(s string, width int) string {
	if lipgloss.Width(s) >= width {
		return cutANSIWidthPrefix(s, width)
	}
	return s + strings.Repeat(" ", width-lipgloss.Width(s))
}

// cutANSIWidthPrefix returns the leading display-width cells of s. It is a simple
// rune-based cut; styled background lines in this app are plain enough for it.
func cutANSIWidthPrefix(s string, width int) string {
	var b strings.Builder
	w := 0
	for _, r := range s {
		if w >= width {
			break
		}
		b.WriteRune(r)
		w += lipgloss.Width(string(r))
	}
	return b.String()
}

// cutANSIWidth returns the suffix of s starting at display column start.
func cutANSIWidth(s string, start int) string {
	w := 0
	for i, r := range s {
		if w >= start {
			return s[i:]
		}
		w += lipgloss.Width(string(r))
	}
	return ""
}
