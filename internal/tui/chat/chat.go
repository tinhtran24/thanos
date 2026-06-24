// Package chat renders the live phase/round agent activity as a chat log: each
// role is a colored "speaker" bubble. It supports keyboard selection of bubbles
// and copying their text. It mirrors crush's internal/ui/chat.
package chat

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/tui/styles"
)

const maxBodyBytes = 64 * 1024

// Model is the scrollable chat log.
type Model struct {
	vp       viewport.Model
	messages []Message
	nextID   int
	width    int
	height   int
	focused  bool

	selIdx     int // selected message index, -1 when none
	anchor     int // range anchor, -1 when no range
	userScroll bool
	lineOwner  []int // content line index → message index (-1 for separators)
}

// New returns an empty chat model.
func New() Model {
	return Model{vp: viewport.New(0, 0), selIdx: -1, anchor: -1}
}

// SetSize sizes the underlying viewport.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.vp.Width = width
	m.vp.Height = height
	m.rerender()
}

// Focus/Blur toggle whether the selection cursor is shown.
func (m *Model) Focus() { m.focused = true; m.rerender() }
func (m *Model) Blur()  { m.focused = false; m.rerender() }

// Reset clears the log, e.g. when starting a new run.
func (m *Model) Reset() {
	m.messages = nil
	m.selIdx = -1
	m.anchor = -1
	m.userScroll = false
	m.rerender()
}

// Append adds raw streamed agent output to the currently open role bubble,
// creating a generic one if none is open. Used by the stdout activity stream.
func (m *Model) Append(text string) {
	open := m.openIndex()
	if open < 0 {
		m.messages = append(m.messages, Message{ID: m.id(), Kind: KindRole})
		open = len(m.messages) - 1
	}
	body := m.messages[open].Body + text
	if len(body) > maxBodyBytes {
		body = body[len(body)-maxBodyBytes:]
		if nl := strings.IndexByte(body, '\n'); nl >= 0 {
			body = body[nl+1:]
		}
	}
	m.messages[open].Body = body
	m.rerender()
}

// AddNotice/AddError append a closing system message.
func (m *Model) AddNotice(text string) { m.add(Message{Kind: KindNotice, Body: text}) }
func (m *Model) AddError(text string)  { m.add(Message{Kind: KindError, Body: text}) }

// StartCommand opens a labeled bubble for a CLI command's streamed output.
func (m *Model) StartCommand(label string) {
	if i := m.openIndex(); i >= 0 {
		m.messages[i].Done = true
	}
	m.add(Message{Kind: KindRole, Label: label})
}

// CloseOpen marks the currently open bubble (if any) as finished.
func (m *Model) CloseOpen() {
	if i := m.openIndex(); i >= 0 {
		m.messages[i].Done = true
		m.rerender()
	}
}

// LastBody returns the body of the open role bubble (for inspection/tests).
func (m *Model) LastBody() string {
	if i := m.openIndex(); i >= 0 {
		return m.messages[i].Body
	}
	return ""
}

// Count returns the number of messages.
func (m *Model) Count() int { return len(m.messages) }

func (m *Model) add(msg Message) {
	msg.ID = m.id()
	m.messages = append(m.messages, msg)
	m.rerender()
}

func (m *Model) id() int { m.nextID++; return m.nextID }

// openIndex returns the index of the last not-yet-finished role bubble, or -1.
func (m *Model) openIndex() int {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Kind == KindRole {
			if m.messages[i].Done {
				return -1
			}
			return i
		}
		if m.messages[i].Kind == KindSystem {
			continue
		}
		return -1
	}
	return -1
}

// --- selection ---------------------------------------------------------------

// SelectMove moves the selection cursor by delta over role bubbles.
func (m *Model) SelectMove(delta int) {
	if len(m.messages) == 0 {
		return
	}
	if m.selIdx < 0 {
		m.selIdx = len(m.messages) - 1
	} else {
		m.selIdx = clamp(m.selIdx+delta, 0, len(m.messages)-1)
	}
	m.anchor = -1
	m.ensureSelectedVisible()
	m.rerender()
}

// SelectExtend moves the selection cursor while keeping a range anchor.
func (m *Model) SelectExtend(delta int) {
	if len(m.messages) == 0 {
		return
	}
	if m.selIdx < 0 {
		m.selIdx = len(m.messages) - 1
	}
	if m.anchor < 0 {
		m.anchor = m.selIdx
	}
	m.selIdx = clamp(m.selIdx+delta, 0, len(m.messages)-1)
	m.ensureSelectedVisible()
	m.rerender()
}

// ClearSelection drops the current selection.
func (m *Model) ClearSelection() {
	m.selIdx = -1
	m.anchor = -1
	m.rerender()
}

// SelectAtViewportRow selects the bubble under a viewport row (0 = top visible
// line). Returns false if the row is blank or out of range.
func (m *Model) SelectAtViewportRow(row int) bool {
	idx := m.messageAtRow(row)
	if idx < 0 {
		return false
	}
	m.focused = true
	m.selIdx = idx
	m.anchor = -1
	m.rerender()
	return true
}

// ExtendAtViewportRow extends the selection range to the bubble under a row,
// anchoring at the previous selection.
func (m *Model) ExtendAtViewportRow(row int) {
	idx := m.messageAtRow(row)
	if idx < 0 {
		return
	}
	if m.selIdx < 0 {
		m.selIdx = idx
	}
	if m.anchor < 0 {
		m.anchor = m.selIdx
	}
	m.selIdx = idx
	m.rerender()
}

// messageAtRow maps a viewport row to a message index, or -1. Clicks on a blank
// separator or below the last bubble snap to the nearest preceding bubble.
func (m *Model) messageAtRow(row int) int {
	if len(m.lineOwner) == 0 {
		return -1
	}
	if row < 0 {
		row = 0
	}
	line := m.vp.YOffset + row
	if line < 0 {
		line = 0
	}
	if line >= len(m.lineOwner) {
		line = len(m.lineOwner) - 1
	}
	for line >= 0 && m.lineOwner[line] < 0 {
		line--
	}
	if line < 0 {
		return -1
	}
	return m.lineOwner[line]
}

// HasSelection reports whether a bubble is selected.
func (m *Model) HasSelection() bool { return m.selIdx >= 0 }

// SelectedText returns the plaintext of the selected message(s) for copying.
func (m *Model) SelectedText() string {
	lo, hi := m.selRange()
	if lo < 0 {
		return ""
	}
	var parts []string
	for i := lo; i <= hi; i++ {
		parts = append(parts, m.messages[i].plain())
	}
	return strings.Join(parts, "\n\n")
}

func (m *Model) selRange() (int, int) {
	if m.selIdx < 0 {
		return -1, -1
	}
	lo, hi := m.selIdx, m.selIdx
	if m.anchor >= 0 {
		lo, hi = min(m.anchor, m.selIdx), max(m.anchor, m.selIdx)
	}
	return lo, hi
}

// --- scrolling ---------------------------------------------------------------

// ScrollUp/ScrollDown move the viewport and mark a manual scroll.
func (m *Model) ScrollUp(n int)   { m.vp.LineUp(n); m.userScroll = !m.vp.AtBottom() }
func (m *Model) ScrollDown(n int) { m.vp.LineDown(n); m.userScroll = !m.vp.AtBottom() }

func (m *Model) ensureSelectedVisible() {
	// Simplest reliable behavior: keep the bottom in view as new content streams,
	// and jump to bottom when selecting the last message.
	if m.selIdx == len(m.messages)-1 {
		m.vp.GotoBottom()
		m.userScroll = false
	}
}

// View renders the chat log viewport.
func (m *Model) View() string { return m.vp.View() }

func (m *Model) rerender() {
	lo, hi := m.selRange()
	var b strings.Builder
	owner := make([]int, 0, 256)
	for i, msg := range m.messages {
		selected := m.focused && i >= lo && i <= hi && lo >= 0
		block := msg.render(contentWidth(m.width), selected)
		b.WriteString(block)
		for k := 0; k < lipgloss.Height(block); k++ {
			owner = append(owner, i)
		}
		if i < len(m.messages)-1 {
			b.WriteString("\n\n")
			owner = append(owner, -1) // the blank separator line
		}
	}
	m.lineOwner = owner
	content := b.String()
	if content == "" {
		content = styles.MutedS.Render("No agent activity yet. Run a session to see the\nconversation between phase roles here.")
	}
	m.vp.SetContent(content)
	if !m.userScroll {
		m.vp.GotoBottom()
	}
}

func contentWidth(width int) int {
	if width < 12 {
		return 12
	}
	return width
}

// PhaseHeader renders a flow band for the current phase above the log.
func PhaseHeader(current model.Phase, width int) string {
	return lipgloss.NewStyle().Render(RenderFlow(current, width))
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
