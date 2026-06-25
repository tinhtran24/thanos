package chat

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/tui/styles"
	"github.com/tinhtran/thanos/internal/tui/util"
)

// Kind categorizes a chat message.
type Kind int

const (
	// KindRole is agent output attributed to a phase role.
	KindRole Kind = iota
	// KindSystem is a thin divider such as a phase transition.
	KindSystem
	// KindNotice is a success/info note (e.g. run finished).
	KindNotice
	// KindError is a failure note.
	KindError
)

// Message is one bubble in the chat log.
type Message struct {
	ID      int
	Kind    Kind
	Role    model.Role
	Label   string // overrides the role label (used for CLI command bubbles)
	Phase   model.Phase
	Context string
	Body    string
	Done    bool
	TS      time.Time
}

// plain returns the copy-friendly plaintext of a message (header + body).
func (m Message) plain() string {
	switch m.Kind {
	case KindRole:
		head := styles.Role(m.Role).Label
		if m.Label != "" {
			head = m.Label
		}
		if m.Context != "" {
			head += "  " + m.Context
		}
		return head + "\n" + strings.TrimRight(m.Body, "\n")
	default:
		return strings.TrimRight(m.Body, "\n")
	}
}

// render draws the message as a bubble at the given content width. When selected
// the bubble border is accented.
func (m Message) render(width int, selected bool) string {
	switch m.Kind {
	case KindSystem:
		line := styles.MutedS.Render("─── " + m.Body + " ───")
		return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(line)
	case KindNotice:
		return styles.SuccessS.Render("✓ " + util.Wrap(m.Body, width))
	case KindError:
		return styles.DangerS.Render("✗ " + util.Wrap(m.Body, width))
	}

	rs := styles.Role(m.Role)
	avatar, label, color := rs.Avatar, rs.Label, rs.Color
	if m.Label != "" {
		avatar, label, color = "$", m.Label, styles.Info
	}
	header := lipgloss.NewStyle().Foreground(color).Bold(true).
		Render(avatar + " " + label)
	meta := ""
	if m.Context != "" {
		meta = styles.MutedS.Render("  " + m.Context)
	}
	if !m.Done && m.Body == "" {
		meta += styles.MutedS.Render("  …")
	}

	bodyWidth := util.Max(8, width-3)
	body := util.Wrap(strings.TrimRight(m.Body, "\n"), bodyWidth)
	if strings.TrimSpace(body) == "" {
		body = styles.MutedS.Render("(working…)")
	} else {
		body = lipgloss.NewStyle().Foreground(styles.Text).Render(body)
	}

	border := lipgloss.NormalBorder()
	borderColor := styles.Subtle
	if selected {
		borderColor = styles.Accent
	}
	box := lipgloss.NewStyle().
		Border(border, false, false, false, true).
		BorderForeground(color).
		PaddingLeft(1).
		Width(util.Max(10, width-2))
	if selected {
		box = box.BorderForeground(borderColor)
	}
	return box.Render(header + meta + "\n" + body)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
