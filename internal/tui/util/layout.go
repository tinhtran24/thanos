package util

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
// covers. It is ANSI-aware (via x/ansi) so colored backgrounds composite cleanly
// without the foreground bleeding into the page or vice-versa — opaque modals.
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
		// Pad the foreground line to the full overlay width so it fully covers
		// (opaquely) the background cells beneath it.
		if w := ansi.StringWidth(fgLine); w < fgWidth {
			fgLine += strings.Repeat(" ", fgWidth-w)
		}
		bgLine := bgLines[row]
		leftPart := ansi.Truncate(bgLine, left, "")
		if w := ansi.StringWidth(leftPart); w < left {
			leftPart += strings.Repeat(" ", left-w)
		}
		rightPart := ansi.TruncateLeft(bgLine, left+fgWidth, "")
		// Reset SGR on both sides so neither layer's colors leak into the other.
		bgLines[row] = leftPart + "\x1b[0m" + fgLine + "\x1b[0m" + rightPart
	}
	return strings.Join(bgLines, "\n")
}
