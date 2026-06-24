// Package input is the bottom command box. It is NOT a free-text LLM channel:
// it parses slash-commands (/run, /approve, /runner, /new, /find, /copy,
// /select, /clear, /help, /quit) and offers a completion dropdown. It mirrors
// crush's editor + completions, scoped to control commands.
package input

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
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
	{"/continue", "resume the selected session from its failed round"},
	{"/new", "create a new session: /new <title>"},
	{"/bugfix", "open a bugfix of the selected session: /bugfix <title>"},
	{"/runner", "switch model runner: /runner [name]"},
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

// Model wraps a textinput plus completion state.
type Model struct {
	ti         textinput.Model
	width      int
	focused    bool
	noComplete bool // when true (form mode), suppress slash-command completions
}

// SetCommandMode toggles slash-command completions. Disable it while collecting
// free-text form fields (title/description/acceptance).
func (m *Model) SetCommandMode(on bool) { m.noComplete = !on }

// DefaultPlaceholder is shown when the box is idle (command mode).
const DefaultPlaceholder = "type a / command (enter runs the selected session)"

// New returns a blurred command box.
func New() Model {
	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.Placeholder = DefaultPlaceholder
	// Use the real terminal cursor (crush's pattern): the parent surfaces it via
	// the View.Cursor field, offset to the input's on-screen position.
	ti.SetVirtualCursor(false)
	return Model{ti: ti}
}

// SetPrompt changes the leading prompt string.
func (m *Model) SetPrompt(p string) { m.ti.Prompt = p }

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

// Value returns the current text.
func (m *Model) Value() string { return m.ti.Value() }

// Reset clears the box.
func (m *Model) Reset() { m.ti.SetValue("") }

// SetValue replaces the text and moves the cursor to the end.
func (m *Model) SetValue(v string) {
	m.ti.SetValue(v)
	m.ti.CursorEnd()
}

// Update forwards key/edit messages to the textinput.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.ti, cmd = m.ti.Update(msg)
	return cmd
}

// matches returns commands whose name has the current token as a prefix.
func (m *Model) matches() []Command {
	if m.noComplete {
		return nil
	}
	val := strings.TrimSpace(m.ti.Value())
	if !strings.HasPrefix(val, "/") || strings.Contains(val, " ") {
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

// View renders the optional completions popup above the input line.
func (m *Model) View(width int) string {
	line := m.ti.View()
	matches := m.matches()
	if !m.focused || len(matches) == 0 {
		return line
	}
	nameStyle := styles.AccentS.Bold(true)
	descStyle := styles.MutedS
	var rows []string
	for i, c := range matches {
		if i >= 6 {
			break
		}
		rows = append(rows, "  "+nameStyle.Render(c.Name)+"  "+descStyle.Render(c.Desc))
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
	parts := strings.SplitN(value, " ", 2)
	cmd = parts[0]
	if len(parts) == 2 {
		arg = strings.TrimSpace(parts[1])
	}
	return cmd, arg
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
