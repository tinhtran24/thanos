package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/tui/chat"
	"github.com/tinhtran/thanos/internal/tui/dialog"
	"github.com/tinhtran/thanos/internal/tui/sidebar"
	"github.com/tinhtran/thanos/internal/tui/styles"
	"github.com/tinhtran/thanos/internal/tui/util"
)

const paneGap = 2

// sidebarWidth is the width of the right column (logo + feature/EC tree + model).
func (m *modelUI) sidebarWidth() int {
	sw := util.Min(44, util.Max(32, m.width*2/5))
	if m.width-sw-paneGap < 36 {
		sw = util.Max(24, m.width-36-paneGap)
	}
	return sw
}

// relayout resizes layout-dependent components when the window changes.
func (m *modelUI) relayout() {
	if m.width == 0 || m.height == 0 {
		return
	}
	m.input.SetMaxHeight(util.Min(6, util.Max(1, m.height/3)))
	m.input.SetWidth(m.width - 2)
	if m.picker != nil {
		m.picker.SetSize(m.width, m.height)
	}
}

// View returns the rendered frame plus the declarative terminal modes. In
// Bubble Tea v2, AltScreen and MouseMode are fields on the View: when the user
// enters mouse-select mode we set MouseMode=None so the terminal performs its
// own text selection; otherwise CellMotion lets the app handle clicks/drags.
func (m *modelUI) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	if m.selectMode {
		view.MouseMode = tea.MouseModeNone
	}
	// Place the real terminal cursor at the command box when it has focus
	// (crush's pattern). The input's cursor X already includes the prompt width;
	// the bottom bar is rendered at column 0, so only the row needs offsetting.
	if m.focus == focusInput && m.picker == nil && m.confirm == nil && m.clarify == nil && !m.showHelp {
		if cur := m.input.Cursor(); cur != nil {
			cur.Y += m.inputY0
			view.Cursor = cur
		}
	}
	return view
}

func (m *modelUI) render() string {
	if m.width == 0 || m.height == 0 {
		return "Starting Thanos…"
	}

	header := m.renderHeader()
	footer := m.renderFooter()
	bottom := m.renderBottom()

	bodyTop := lipgloss.Height(header)
	bodyHeight := util.Max(0, m.height-lipgloss.Height(header)-lipgloss.Height(footer)-lipgloss.Height(bottom)-3)

	sidebarW := m.sidebarWidth()
	chatWidth := util.Max(30, m.width-sidebarW-paneGap)

	// Chat is the main pane on the LEFT; the feature/EC tree sidebar is on the RIGHT.
	m.chatH = 0
	center := m.renderCenter(chatWidth-4, bodyHeight)
	right := m.renderRightSidebar(sidebarW-4, bodyHeight)
	chatPanel := m.renderPanel(center, chatWidth, bodyHeight, m.focus == focusChat)
	m.chatX0 = 2 // left panel border + padding
	m.chatY0 = bodyTop + 1 + m.centerHeaderLines
	m.chatW = chatWidth - 4

	sidePanel := m.renderPanel(right, sidebarW, bodyHeight, m.focus == focusSessions)
	// Tree row geometry (border + logo(2) + blank(1) inside the right panel).
	m.sidebarX0 = chatWidth + paneGap + 2
	m.sidebarW = sidebarW - 4
	m.treeY0 = bodyTop + 1 + 3

	body := ""
	if bodyHeight > 0 {
		body = lipgloss.JoinHorizontal(lipgloss.Top, chatPanel, strings.Repeat(" ", paneGap), sidePanel)
	}

	// Record the textarea origin after any attachment and completion rows.
	m.inputY0 = lipgloss.Height(header) + m.inputYOffset
	sections := []string{header}
	if body != "" {
		m.inputY0 += lipgloss.Height(body)
		sections = append(sections, body)
	}
	sections = append(sections, bottom, footer)

	screen := strings.Join(sections, "\n")

	if m.showHelp {
		screen = util.Overlay(screen, dialog.RenderHelp(helpEntries()), m.width, m.height)
	}
	if m.confirm != nil {
		screen = util.Overlay(screen, m.confirm.View(), m.width, m.height)
	}
	if m.clarify != nil {
		screen = util.Overlay(screen, m.clarify.View(), m.width, m.height)
	}
	if m.picker != nil {
		screen = util.Overlay(screen, m.picker.View(), m.width, m.height)
	}
	return screen
}

func (m *modelUI) renderPanel(content string, width, height int, focused bool) string {
	if height <= 0 {
		return ""
	}
	if height < 3 {
		lines := strings.Split(fitPanelHeight(content, height), "\n")
		for i := range lines {
			lines[i] = util.Truncate(lines[i], util.Max(1, width))
		}
		return strings.Join(lines, "\n")
	}
	content = fitPanelHeight(content, util.Max(1, height-2))
	style := styles.Panel(width, height)
	if focused {
		style = styles.FocusedPanel(width, height)
	}
	return style.Render(content)
}

func fitPanelHeight(content string, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
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

// renderRightSidebar draws the logo, the Feature→EC tree, and the model/MCP
// info, and records the tree's row geometry for mouse hit-testing.
func (m *modelUI) renderRightSidebar(width, bodyHeight int) string {
	phase := make(map[string]model.Phase, len(m.features))
	started := make(map[string]bool, len(m.features))
	for _, f := range m.features {
		snap := m.states[f.ID]
		phase[f.ID] = snap.state.Phase
		started[f.ID] = snap.ok
	}
	tree, rows := sidebar.RenderTree(sidebar.TreeData{
		Features: m.features, Phase: phase, Started: started, Plans: m.plans,
		Cursor: m.cursor, ECCursor: m.ecCursor, RunningID: m.runningID,
	}, width)
	m.treeRows = rows

	feature, _ := m.selected()
	runnerName := feature.Runner
	if runnerName == "" {
		runnerName = m.config.DefaultRunner
	}
	caps := chat.RenderSidebar(chat.SidebarData{
		Config: m.config, Runner: runnerName, Feature: feature, DotDir: m.ws.DotDir(),
	}, width)
	return sidebar.Logo(m.version) + "\n\n" + tree + "\n" + caps
}

func (m *modelUI) renderCenter(width, bodyHeight int) string {
	if m.create.active {
		return m.renderCreateForm(width)
	}
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
	flow := chat.RenderWorkflow(current, width)

	headerLines := lipgloss.Height(title) + lipgloss.Height(meta) + lipgloss.Height(flow) + 1
	chatHeight := util.Max(1, (bodyHeight-2)-headerLines)
	m.chat.SetSize(width, chatHeight)
	m.centerHeaderLines = headerLines
	m.chatH = chatHeight

	return title + "\n" + meta + "\n" + flow + "\n\n" + m.chat.View()
}

// renderCreateForm shows the guided new-session / bugfix form: each field with
// its collected value, the active field mirroring what is typed in the box below.
func (m *modelUI) renderCreateForm(width int) string {
	m.chatH = 0 // no chat rectangle while the form is open

	heading := "New session"
	if m.create.bugfix {
		heading = "New bugfix"
	}
	var b strings.Builder
	b.WriteString(styles.SectionTitle(heading))
	b.WriteString("\n")
	if m.create.bugfix && m.create.parentID != "" {
		b.WriteString(styles.WarningS.Render("bugfix of " + m.create.parentID))
		b.WriteString("\n")
	}
	b.WriteString(styles.MutedS.Render("Type below; enter saves the field and moves on. esc cancels."))
	b.WriteString("\n\n")

	fields := []struct {
		label string
		step  int
		value string
	}{
		{"Title", stepTitle, m.create.title},
		{"Task description", stepDesc, m.create.desc},
		{"Acceptance criteria", stepAccept, m.create.accept},
	}
	for _, f := range fields {
		value := f.value
		var marker string
		switch {
		case f.step == m.create.step:
			marker = styles.AccentS.Bold(true).Render("▌ ")
			value = m.input.Value() // live mirror of the box
		case f.step < m.create.step:
			marker = styles.SuccessS.Render("✓ ")
		default:
			marker = styles.MutedS.Render("○ ")
		}
		b.WriteString(marker + styles.Title.Render(f.label) + "\n")
		shown := strings.TrimSpace(value)
		if shown == "" {
			if f.step == m.create.step {
				shown = styles.MutedS.Render("(typing in the box below…)")
			} else {
				shown = styles.MutedS.Render("—")
			}
		} else {
			shown = lipgloss.NewStyle().Foreground(styles.Text).Render(util.Wrap(shown, util.Max(8, width-2)))
		}
		b.WriteString("  " + strings.ReplaceAll(shown, "\n", "\n  ") + "\n\n")
	}
	b.WriteString(styles.MutedS.Render(fmt.Sprintf("step %d of 3", m.create.step+1)))
	return b.String()
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
	m.inputYOffset = 0
	if strip := m.attach.View(width); strip != "" {
		parts = append(parts, strip)
		m.inputYOffset += lipgloss.Height(strip)
	}
	composer := m.input.View(width)
	m.inputYOffset += lipgloss.Height(composer) - m.input.Height()
	parts = append(parts, composer)
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
	switch {
	case m.create.active:
		hint = fmt.Sprintf("new session — step %d/3 · paste single/multiline · enter: save & next · esc: cancel", m.create.step+1)
	case m.focus == focusChat:
		hint = "↑↓ pick · K/J range · y copy · click/drag mouse-copy · esc back · tab input"
	case m.focus == focusInput:
		hint = "paste single/multiline (line breaks kept) · enter submit · esc cancel · / commands · tab cycle"
	default:
		hint = "↑↓ nav · →← EC · enter run · n new · x rm-EC · c clarify · m runner · / cmd · tab chat · ? help"
	}
	return lipgloss.NewStyle().Width(m.width).Render(styles.MutedS.Render(hint))
}

func helpEntries() []dialog.HelpEntry {
	he := func(keys, desc string) dialog.HelpEntry { return dialog.HelpEntry{Keys: keys, Desc: desc} }
	return []dialog.HelpEntry{
		he("↑ / ↓ / j / k", "move in the feature/EC tree"),
		he("→ / ←", "descend into / back out of a feature's ECs"),
		he("1-9", "jump to feature"),
		he("enter / space", "run or resume the selected feature"),
		he("x", "remove the selected execution chunk (EC)"),
		he("c", "answer a pending clarification (popup)"),
		he("tab", "cycle focus: tree → chat → input"),
		he("n", "new session (guided form)"),
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
