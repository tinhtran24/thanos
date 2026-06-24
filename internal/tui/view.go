package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/tui/chat"
	"github.com/tinhtran/thanos/internal/tui/dialog"
	"github.com/tinhtran/thanos/internal/tui/list"
	"github.com/tinhtran/thanos/internal/tui/styles"
	"github.com/tinhtran/thanos/internal/tui/util"
)

const (
	rightPaneWidth = 30
	paneGap        = 2
	rightThreshold = 112
)

func (m *modelUI) leftWidth() int {
	return util.Min(32, util.Max(24, m.width/4))
}

// relayout resizes layout-dependent components when the window changes.
func (m *modelUI) relayout() {
	if m.width == 0 || m.height == 0 {
		return
	}
	m.input.SetWidth(m.width - 2)
	if m.picker != nil {
		m.picker.SetSize(m.width, m.height)
	}
}

func (m *modelUI) View() string {
	if m.width == 0 || m.height == 0 {
		return "Starting Thanos…"
	}

	header := m.renderHeader()
	footer := m.renderFooter()
	bottom := m.renderBottom()

	bodyTop := lipgloss.Height(header)
	bodyHeight := util.Max(4, m.height-lipgloss.Height(header)-lipgloss.Height(footer)-lipgloss.Height(bottom)-3)

	leftWidth := m.leftWidth()
	showRight := m.width >= rightThreshold
	centerWidth := m.width - leftWidth - paneGap
	if showRight {
		centerWidth -= rightPaneWidth + paneGap
	}
	centerWidth = util.Max(32, centerWidth)

	// renderCenter sizes the chat viewport and records centerHeaderLines/chatH.
	m.chatH = 0
	left := m.renderPanel(m.renderSessions(leftWidth-4), leftWidth, bodyHeight, m.focus == focusSessions)
	center := m.renderPanel(m.renderCenter(centerWidth-4, bodyHeight), centerWidth, bodyHeight, m.focus == focusChat)
	// Screen rectangle of the chat viewport (border + padding + header offsets).
	m.chatX0 = leftWidth + paneGap + 2
	m.chatY0 = bodyTop + 1 + m.centerHeaderLines
	m.chatW = centerWidth - 4

	columns := []string{left, center}
	if showRight {
		right := m.renderPanel(m.renderSidebar(rightPaneWidth-4), rightPaneWidth, bodyHeight, false)
		columns = append(columns, right)
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, util.JoinWithGap(columns, paneGap)...)

	screen := header + "\n" + body + "\n" + bottom + "\n" + footer

	if m.showHelp {
		screen = util.Overlay(screen, dialog.RenderHelp(helpEntries()), m.width, m.height)
	}
	if m.picker != nil {
		screen = util.Overlay(screen, m.picker.View(), m.width, m.height)
	}
	return screen
}

func (m *modelUI) renderPanel(content string, width, height int, focused bool) string {
	style := styles.Panel(width, height)
	if focused {
		style = styles.FocusedPanel(width, height)
	}
	return style.Render(content)
}

func (m *modelUI) renderHeader() string {
	project := m.config.Project.Name
	if project == "" {
		project = filepath.Base(m.ws.Root)
	}
	brand := lipgloss.NewStyle().Foreground(styles.Accent).Bold(true).Render("THANOS")
	meta := styles.MutedS.Render(fmt.Sprintf("  %s  ·  %s  ·  v%s", project, util.CompactPath(m.ws.Root, 3), m.version))
	var status string
	if m.selectMode {
		status = styles.WarningS.Render("⊟ mouse-select — drag to select, esc to resume")
	} else if m.running {
		frames := []string{"◒", "◐", "◓", "◑"}
		status = styles.WarningS.Render(frames[m.spinnerFrame%len(frames)] + " agent running")
	} else {
		status = styles.SuccessS.Render("● ready")
	}
	space := util.Max(1, m.width-lipgloss.Width(brand+meta)-lipgloss.Width(status))
	return lipgloss.NewStyle().Width(m.width).Render(brand + meta + strings.Repeat(" ", space) + status)
}

func (m *modelUI) renderSessions(width int) string {
	sessions := make([]list.Session, 0, len(m.features))
	for _, f := range m.features {
		snap := m.states[f.ID]
		sessions = append(sessions, list.Session{Feature: f, Phase: snap.state.Phase, Started: snap.ok})
	}
	return list.Render(sessions, m.cursor, m.runningID, width)
}

func (m *modelUI) renderCenter(width, bodyHeight int) string {
	feature, ok := m.selected()
	if !ok {
		return styles.SectionTitle("Conversation") + "\n\n" +
			styles.MutedS.Render("Create a session (n) and run it to see the\nrole-by-role agent conversation here.")
	}
	snapshot := m.states[feature.ID]
	current := model.State{Phase: model.PhaseInit}
	if snapshot.ok {
		current = snapshot.state
	}
	runnerName := feature.Runner
	if runnerName == "" {
		runnerName = m.config.DefaultRunner
	}

	title := styles.Title.Render(util.Truncate(feature.Title, width))
	metaText := feature.ID + "  ·  " + feature.Priority + " priority  ·  " + runnerName
	if feature.Parent != "" {
		metaText += "  ·  bugfix of " + feature.Parent
	}
	meta := styles.MutedS.Render(util.Truncate(metaText, width))
	flow := chat.RenderFlow(current.Phase, width)

	headerLines := lipgloss.Height(title) + lipgloss.Height(meta) + lipgloss.Height(flow) + 1
	chatHeight := util.Max(1, (bodyHeight-2)-headerLines)
	m.chat.SetSize(width, chatHeight)
	m.centerHeaderLines = headerLines
	m.chatH = chatHeight

	return title + "\n" + meta + "\n" + flow + "\n\n" + m.chat.View()
}

func (m *modelUI) renderSidebar(width int) string {
	feature, _ := m.selected()
	runnerName := feature.Runner
	if runnerName == "" {
		runnerName = m.config.DefaultRunner
	}
	return chat.RenderSidebar(chat.SidebarData{
		Config:  m.config,
		Runner:  runnerName,
		Feature: feature,
		DotDir:  m.ws.DotDir(),
	}, width)
}

func (m *modelUI) renderBottom() string {
	width := m.width - 2
	var parts []string
	if strip := m.attach.View(width); strip != "" {
		parts = append(parts, strip)
	}
	parts = append(parts, m.input.View(width))
	return lipgloss.NewStyle().Width(m.width).Render(strings.Join(parts, "\n"))
}

func (m *modelUI) renderFooter() string {
	if m.err != nil {
		return lipgloss.NewStyle().Width(m.width).Render(
			styles.DangerS.Render("error: " + util.Truncate(m.err.Error(), util.Max(10, m.width-8))))
	}
	if m.copyFlash != "" {
		return lipgloss.NewStyle().Width(m.width).Render(styles.SuccessS.Render(m.copyFlash))
	}
	if m.notice != "" {
		return lipgloss.NewStyle().Width(m.width).Render(styles.SuccessS.Render(util.Truncate(m.notice, m.width)))
	}
	var hint string
	switch m.focus {
	case focusChat:
		hint = "↑↓ pick · K/J range · y copy · click/drag mouse-copy · esc back · tab input"
	case focusInput:
		hint = "enter submit · esc cancel · type / for all commands · tab cycle"
	default:
		hint = "↑↓ select · enter run · n new · m runner · / commands · ctrl+p find · ? help · q quit"
	}
	return lipgloss.NewStyle().Width(m.width).Render(styles.MutedS.Render(hint))
}

func helpEntries() []dialog.HelpEntry {
	he := func(keys, desc string) dialog.HelpEntry { return dialog.HelpEntry{Keys: keys, Desc: desc} }
	return []dialog.HelpEntry{
		he("↑ / ↓ / j / k", "move selection (sessions or chat)"),
		he("1-9", "jump to session"),
		he("enter / space", "run or resume the selected session"),
		he("tab", "cycle focus: sessions → chat → input"),
		he("n", "new session (prefilled /new)"),
		he("m", "switch model runner"),
		he("d", "approve a pending session"),
		he("r", "reload workspace"),
		he("ctrl+p", "fuzzy-find a session"),
		he("/", "open the command box"),
		he("K / J (in chat)", "extend bubble selection"),
		he("y (in chat)", "copy selected bubble(s) via OSC52"),
		he("click / drag (chat)", "select bubble(s); release copies"),
		he("ctrl+s", "mouse-select mode (native drag-select)"),
		he("pgup / pgdn", "scroll the chat log"),
		he("q / ctrl+c", "quit"),
	}
}
