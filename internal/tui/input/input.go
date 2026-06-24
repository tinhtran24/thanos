// Package input is the bottom command box. It is NOT a free-text LLM channel:
// it parses slash-commands (/run, /approve, /runner, /new, /find, /copy,
// /select, /clear, /help, /quit) and offers a completion dropdown. It mirrors
// crush's editor + completions, scoped to control commands.
package input

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	ti      textinput.Model
	width   int
	focused bool
}

// New returns a blurred command box.
func New() Model {
	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.Placeholder = "type a / command (enter runs the selected session)"
	ti.PromptStyle = styles.AccentS
	ti.TextStyle = lipgloss.NewStyle().Foreground(styles.Text)
	ti.PlaceholderStyle = styles.MutedS
	return Model{ti: ti}
}

// Focus gives the box keyboard focus.
func (m *Model) Focus() tea.Cmd { m.focused = true; return m.ti.Focus() }

// Blur removes focus.
func (m *Model) Blur() { m.focused = false; m.ti.Blur() }

// Focused reports focus state.
func (m *Model) Focused() bool { return m.focused }

// SetWidth sizes the box.
func (m *Model) SetWidth(w int) { m.width = w; m.ti.Width = max(4, w-4) }

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
