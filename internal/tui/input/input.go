// Package input is the bottom command box. It is NOT a free-text LLM channel:
// it parses slash-commands (/run, /approve, /runner, /new, /find, /copy,
// /select, /clear, /help, /quit) and offers a completion dropdown. It mirrors
// crush's editor + completions, scoped to control commands.
package input

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tinhtran/thanos/internal/tui/styles"
)

// Command describes a slash-command for completion + help.
type Command struct {
	Name string
	Desc string
}

// Commands is the catalog of control commands, mirroring the Thanos CLI feature
// set. Session/feature commands act on the selected session when no ID is given.
var Commands = []Command{
	// Session lifecycle
	{"/run", "run the selected session through its next phase"},
	{"/new", "create a new session: /new <title>"},
	{"/bugfix", "open a bugfix of the selected session: /bugfix <title>"},
	{"/feature", "select a session by ID, or open the picker: /feature [id]"},
	{"/runner", "switch model runner: /runner [name]"},
	{"/agent", "switch model runner (alias of /runner): /agent [name]"},
	{"/transition", "force a phase: /transition <phase>"},
	{"/approve", "approve a pending session (alias of /done)"},
	{"/done", "mark the selected pending session done"},
	{"/prompt", "render a role prompt: /prompt <role>"},
	// Workspace / inspection
	{"/status", "show the status table for all sessions"},
	{"/scan", "re-index the codebase and feature memory"},
	{"/doctor", "check runners, LSP and MCP availability"},
	{"/memory", "show feature memory: /memory [id]"},
	// Extensions / config
	{"/skill", "manage skills: /skill add <source> | /skill find"},
	{"/plugin", "manage plugins: /plugin install claude <name>"},
	{"/lsp", "add a language server: /lsp add <name> --command <cmd>"},
	{"/mcp", "add an MCP server: /mcp add <name> --type <t>"},
	// UI
	{"/find", "fuzzy-find a session"},
	{"/copy", "copy the selected chat bubble(s)"},
	{"/select", "toggle native mouse-select mode"},
	{"/clear", "clear staged attachments"},
	{"/help", "show keyboard shortcuts"},
	{"/quit", "quit Thanos"},
}

const maxContentHeight = 10000

// Model wraps a textarea plus completion state.
type Model struct {
	ti              textarea.Model
	width           int
	focused         bool
	noComplete      bool // when true (form mode), suppress slash-command completions
	completionIndex int  // highlighted row in the slash-command dropdown
	err             error
}

// SetCommandMode toggles slash-command completions. Disable it while collecting
// free-text form fields (title/description/acceptance).
func (m *Model) SetCommandMode(on bool) { m.noComplete = !on }

// DefaultPlaceholder is shown when the box is idle (command mode).
const DefaultPlaceholder = "type a / command (enter runs the selected session)"

// New returns a blurred command box.
func New() Model {
	ti := textarea.New()
	ti.Prompt = "❯ "
	ti.Placeholder = DefaultPlaceholder
	ti.ShowLineNumbers = false
	ti.CharLimit = 0
	ti.DynamicHeight = true
	ti.MinHeight = 1
	ti.MaxHeight = 6
	ti.MaxContentHeight = maxContentHeight
	// Use the real terminal cursor (crush's pattern): the parent surfaces it via
	// the View.Cursor field, offset to the input's on-screen position.
	ti.SetVirtualCursor(false)
	ti.SetHeight(1)
	return Model{ti: ti}
}

// SetPrompt changes the leading prompt string.
func (m *Model) SetPrompt(p string) {
	m.ti.Prompt = p
	m.ti.SetWidth(max(4, m.width-4))
}

// SetPlaceholder changes the placeholder text shown when empty.
func (m *Model) SetPlaceholder(s string) { m.ti.Placeholder = s }

// Cursor returns the text caret, positioned relative to the input's own view
// (X already includes the prompt width). The caller offsets it to screen
// coordinates and assigns it to the Bubble Tea View. Returns nil when blurred.
func (m *Model) Cursor() *tea.Cursor { return m.ti.Cursor() }

// Focus gives the box keyboard focus.
func (m *Model) Focus() tea.Cmd { m.focused = true; return m.ti.Focus() }

// Blur removes focus.
func (m *Model) Blur() { m.focused = false; m.ti.Blur() }

// Focused reports focus state.
func (m *Model) Focused() bool { return m.focused }

// SetWidth sizes the box.
func (m *Model) SetWidth(w int) { m.width = w; m.ti.SetWidth(max(4, w-4)) }

// SetMaxHeight caps the visible composer viewport. Content has a separate
// 10,000-visual-row limit and scrolls when it exceeds this viewport.
func (m *Model) SetMaxHeight(h int) {
	m.ti.MaxHeight = max(1, h)
	m.ti.SetHeight(m.ti.Height())
}

// Height returns the textarea's current visible height.
func (m *Model) Height() int { return m.ti.Height() }

// Value returns the current text.
func (m *Model) Value() string { return m.ti.Value() }

// Reset clears the box.
func (m *Model) Reset() {
	m.err = nil
	m.ti.SetValue("")
}

// SetValue replaces the text and moves the cursor to the end.
func (m *Model) SetValue(v string) {
	m.err = nil
	m.setValue(normalizeLineEndings(v))
	m.ti.CursorEnd()
}

// Err returns the most recent input validation error.
func (m *Model) Err() error { return m.err }

// HasCompletions reports whether the slash-command dropdown is currently shown.
func (m *Model) HasCompletions() bool {
	return m.focused && len(m.matches()) > 0
}

// MoveCompletion moves the highlighted completion by delta, wrapping around.
func (m *Model) MoveCompletion(delta int) {
	n := len(m.matches())
	if n == 0 {
		m.completionIndex = 0
		return
	}
	m.completionIndex = ((m.completionIndex+delta)%n + n) % n
}

// AcceptCompletion replaces the input with the highlighted command name plus a
// trailing space (ready for arguments). Returns false when no dropdown is shown.
func (m *Model) AcceptCompletion() bool {
	matches := m.matches()
	if len(matches) == 0 {
		return false
	}
	idx := m.completionIndex
	if idx < 0 || idx >= len(matches) {
		idx = 0
	}
	m.setValue(matches[idx].Name + " ")
	m.ti.CursorEnd()
	m.completionIndex = 0
	return true
}

// Update forwards key/edit messages to the textarea.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	m.err = nil
	before := m.ti.Value()
	// Editing the text resets the completion highlight to the top; blink/cursor
	// messages (which don't change the value) leave the selection alone.
	defer func() {
		if m.ti.Value() != before {
			m.completionIndex = 0
		}
	}()
	if pasted, ok := msg.(tea.PasteMsg); ok {
		pasted.Content = sanitizeTextareaInput(normalizeLineEndings(pasted.Content))
		if !m.pasteFits(pasted.Content) {
			m.err = errors.New("paste exceeds the 10,000-row composer limit")
			return nil
		}
		// Bubbles v2.1.0 reserves the textarea's initial row when bulk
		// inserting, so a configured limit of N accepts at most N-1 rows.
		// Raise the insertion budget for this already-preflighted paste only,
		// then restore the real 10,000-row editing ceiling immediately.
		m.ti.MaxContentHeight = maxContentHeight + 1
		m.ti, _ = m.ti.Update(pasted)
		m.ti.MaxContentHeight = maxContentHeight
		return nil
	}
	var cmd tea.Cmd
	m.ti, cmd = m.ti.Update(msg)
	return cmd
}

// pasteFits uses an equivalently configured textarea as an exact preflight.
// The probe receives the value that would result from insertion at the current
// logical cursor, so the real textarea is never partially mutated.
func (m *Model) pasteFits(pasted string) bool {
	value := m.ti.Value()
	lines := strings.Split(value, "\n")
	row := min(m.ti.Line(), len(lines)-1)
	col := m.ti.Column()
	line := []rune(lines[row])
	col = min(col, len(line))
	lines[row] = string(line[:col]) + pasted + string(line[col:])
	candidate := strings.Join(lines, "\n")

	probe := textarea.New()
	probe.Prompt = m.ti.Prompt
	probe.ShowLineNumbers = false
	probe.CharLimit = 0
	probe.DynamicHeight = true
	probe.MinHeight = 1
	probe.MaxHeight = max(1, m.ti.MaxHeight)
	probe.MaxContentHeight = maxContentHeight + 1
	probe.SetWidth(max(4, m.width-4))
	probe.SetValue(candidate)
	return probe.Value() == candidate
}

func (m *Model) setValue(value string) {
	// Match paste semantics at the exact boundary despite the Bubbles bulk
	// insertion off-by-one, while leaving ordinary keyboard editing configured
	// with the required 10,000-row limit.
	m.ti.MaxContentHeight = maxContentHeight + 1
	m.ti.SetValue(value)
	m.ti.MaxContentHeight = maxContentHeight
}

func normalizeLineEndings(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

// sanitizeTextareaInput mirrors the textarea widget's clipboard sanitizer so
// the atomic content-limit preflight compares the value the widget will retain.
func sanitizeTextareaInput(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r == unicode.ReplacementChar:
			continue
		case r == '\n':
			b.WriteRune(r)
		case r == '\t':
			b.WriteString("    ")
		case unicode.IsControl(r):
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// matches returns commands whose name has the current token as a prefix.
func (m *Model) matches() []Command {
	if m.noComplete {
		return nil
	}
	val := strings.TrimSpace(m.ti.Value())
	if !strings.HasPrefix(val, "/") || strings.IndexFunc(val, unicode.IsSpace) >= 0 {
		return nil
	}
	var out []Command
	for _, c := range Commands {
		if strings.HasPrefix(c.Name, val) {
			out = append(out, c)
		}
	}
	if len(out) == 1 && out[0].Name == val {
		return nil // exact match, nothing to suggest
	}
	return out
}

const completionMaxVisible = 6

// View renders the optional completions popup above the input line. The
// highlighted row (moved with ↑/↓) is marked and accepted with tab.
func (m *Model) View(width int) string {
	line := m.ti.View()
	matches := m.matches()
	if !m.focused || len(matches) == 0 {
		return line
	}
	nameStyle := styles.AccentS.Bold(true)
	descStyle := styles.MutedS
	selNameStyle := lipgloss.NewStyle().Foreground(styles.Accent).Bold(true)

	idx := m.completionIndex
	if idx >= len(matches) {
		idx = len(matches) - 1
	}
	if idx < 0 {
		idx = 0
	}

	// Window the visible rows so the highlighted entry is always shown.
	start := 0
	if len(matches) > completionMaxVisible {
		start = idx - completionMaxVisible/2
		if start < 0 {
			start = 0
		}
		if start > len(matches)-completionMaxVisible {
			start = len(matches) - completionMaxVisible
		}
	}
	end := start + completionMaxVisible
	if end > len(matches) {
		end = len(matches)
	}

	var rows []string
	for i := start; i < end; i++ {
		c := matches[i]
		if i == idx {
			rows = append(rows, styles.AccentS.Render("> ")+selNameStyle.Render(c.Name)+"  "+descStyle.Render(c.Desc))
		} else {
			rows = append(rows, "  "+nameStyle.Render(c.Name)+"  "+descStyle.Render(c.Desc))
		}
	}
	if len(matches) > completionMaxVisible {
		rows = append(rows, descStyle.Render(fmt.Sprintf("  %d/%d · ↑↓ select · tab accept", idx+1, len(matches))))
	}
	popup := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true, false, false, false).
		BorderForeground(styles.Subtle).
		Render(strings.Join(rows, "\n"))
	return popup + "\n" + line
}

// Parse splits a submitted line into a command name and its argument. A bare
// line (no leading slash) is treated as the implicit /run command.
func Parse(value string) (cmd string, arg string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/run", ""
	}
	if !strings.HasPrefix(value, "/") {
		return "/run", value
	}
	if index := strings.IndexFunc(value, unicode.IsSpace); index >= 0 {
		cmd = value[:index]
		arg = strings.TrimSpace(value[index:])
		return cmd, arg
	}
	return value, ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
