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

const (
	paneGap                     = 2
	wideSidebarWidth            = 30
	compactModeWidthBreakpoint  = 120
	compactModeHeightBreakpoint = 30
)

// compactLayout always reports true: the cli-sample layout is a single column
// (header block, conversation stream, input box, status line) with no sidebar.
func (m *modelUI) compactLayout() bool {
	return true
}

func (m *modelUI) contentWidth() int {
	width := util.Max(1, m.width-2)
	if !m.compactLayout() {
		width -= wideSidebarWidth + paneGap
	}
	return util.Max(1, width)
}

// relayout resizes layout-dependent components when the window changes.
func (m *modelUI) relayout() {
	if m.width == 0 || m.height == 0 {
		return
	}
	m.input.SetMaxHeight(util.Min(6, util.Max(1, m.height/3)))
	m.input.SetWidth(m.contentWidth())
	if m.picker != nil {
		m.picker.SetSize(m.width, m.height)
	}
}

// View returns the rendered frame plus the declarative terminal modes. In
// Bubble Tea v2, AltScreen and MouseMode are fields on the View. We never
// capture the mouse (MouseMode=None) so the terminal's native click-drag text
// selection — and copy — is always available, matching the cli-sample UX.
func (m *modelUI) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	view.MouseMode = tea.MouseModeNone
	// Place the real terminal cursor at the command box when it has focus
	// (crush's pattern). The input's cursor X already includes the prompt width;
	// the bottom bar is rendered at column 0, so only the row needs offsetting.
	if m.focus == focusInput && m.picker == nil && m.confirm == nil && m.clarify == nil && !m.showHelp {
		if cur := m.input.Cursor(); cur != nil {
			cur.X++
			cur.Y += m.inputY0
			view.Cursor = cur
		}
	}
	return view
}

// render composes the single-column cli-sample layout:
//
//	┌ header block ┐   (logo · tagline · project/runner/feature/status grid)
//	  conversation stream (full width)
//	  thinking line       (spinner, only while running)
//	─ input box ─         (top/bottom rule, ❯ prompt)
//	 status line          ([ready]/[busy] · cwd ............ feature · phase)
func (m *modelUI) render() string {
	if m.width == 0 || m.height == 0 {
		return "Starting Thanos…"
	}

	status := m.renderStatusLine()
	thinking := ""
	if m.running {
		thinking = m.renderThinking()
	}
	bottom := m.renderBottom() // bordered input box; its top rule adds one row

	statusH := lipgloss.Height(status)
	bottomH := lipgloss.Height(bottom)
	thinkingH := 0
	if thinking != "" {
		thinkingH = lipgloss.Height(thinking)
	}

	// Adaptive header: the full rounded block when it fits, else a one-line slim
	// header, else nothing — so the frame never exceeds the terminal height.
	header := m.renderHeaderBlock()
	headerH := lipgloss.Height(header)
	blankH := 1
	fits := func(hh, bh int) bool { return hh + bh + bottomH + statusH + thinkingH <= m.height }
	if !fits(headerH, blankH) {
		header = m.renderHeaderSlim()
		headerH = lipgloss.Height(header)
		if !fits(headerH, blankH) {
			header = ""
			headerH = 0
			blankH = 0
		}
	}

	bodyHeight := util.Max(0, m.height-headerH-blankH-bottomH-statusH-thinkingH)
	chatWidth := m.contentWidth()

	m.chatH = 0
	m.centerHeaderLines = 0
	center := fitPanelHeight(m.renderCenter(chatWidth, bodyHeight), bodyHeight)
	m.chatX0 = 1
	m.chatY0 = headerH + blankH + m.centerHeaderLines
	m.chatW = chatWidth

	// Single column: no sidebar/tree geometry.
	m.sidebarX0 = 0
	m.sidebarW = 0
	m.treeY0 = 0
	m.treeRows = nil

	body := ""
	if bodyHeight > 0 {
		body = lipgloss.NewStyle().Width(chatWidth).Height(bodyHeight).Render(center)
		body = lipgloss.NewStyle().PaddingLeft(1).Render(body)
	}

	// Textarea origin: header + blank line + body + thinking + input top rule,
	// plus the attachment/completion rows counted in inputYOffset.
	m.inputY0 = headerH + blankH + bodyHeight + thinkingH + 1 + m.inputYOffset
	sections := make([]string, 0, 6)
	if header != "" {
		sections = append(sections, header, "")
	}
	if body != "" {
		sections = append(sections, body)
	}
	if thinking != "" {
		sections = append(sections, thinking)
	}
	sections = append(sections, bottom, status)

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

// cli-sample-style chrome.
var (
	headerBlockStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(styles.Accent).
				Padding(0, 2)

	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, true, false).
			BorderForeground(styles.Muted)
)

// renderHeaderBlock draws the rounded cli-sample header: logo + tagline and a
// left-labelled status grid (project / runner / feature / status).
func (m *modelUI) renderHeaderBlock() string {
	project := m.config.Project.Name
	if project == "" {
		project = filepath.Base(m.ws.Root)
	}
	runnerName := m.config.DefaultRunner
	featureLine := styles.MutedS.Render("—")
	if feature, ok := m.selected(); ok {
		if feature.Runner != "" {
			runnerName = feature.Runner
		}
		phase := styles.PhaseLabel(m.states[feature.ID].state.Phase)
		featureLine = lipgloss.NewStyle().Foreground(styles.Text).Render(feature.ID) + styles.MutedS.Render(" · "+phase)
	}

	var statusCell string
	switch {
	case m.selectMode:
		statusCell = styles.WarningS.Render("⊟ mouse-select")
	case m.running:
		frames := []string{"✶", "✷", "✸", "✹", "✺", "✹", "✸", "✷"}
		statusCell = styles.AccentS.Render(frames[m.spinnerFrame%len(frames)] + " running")
	default:
		statusCell = styles.SuccessS.Render("● ready")
	}

	const leftCol = 9
	label := func(s string) string {
		if w := lipgloss.Width(s); w < leftCol {
			s += strings.Repeat(" ", leftCol-w)
		}
		return styles.MutedS.Render(s)
	}
	value := func(s string) string { return lipgloss.NewStyle().Foreground(styles.Text).Render(s) }

	body := strings.Join([]string{
		styles.Title.Render("Thanos ") + styles.MutedS.Render("v"+m.version),
		styles.MutedS.Render("Multi-role AI development framework"),
		"",
		label("project") + value(project),
		label("runner") + value(runnerName),
		label("feature") + featureLine,
		label("status") + statusCell,
	}, "\n")

	return headerBlockStyle.Width(util.Max(20, util.Min(m.width-2, 60))).Render(body)
}

// renderHeaderSlim is the one-line header used when the terminal is too short
// for the full rounded block: logo on the left, ready/running state on the right.
func (m *modelUI) renderHeaderSlim() string {
	logo := styles.Title.Render("Thanos ") + styles.MutedS.Render("v"+m.version)
	st := styles.SuccessS.Render("● ready")
	if m.running {
		frames := []string{"✶", "✷", "✸", "✹", "✺", "✹", "✸", "✷"}
		st = styles.AccentS.Render(frames[m.spinnerFrame%len(frames)] + " running")
	}
	space := util.Max(1, m.width-lipgloss.Width(logo)-lipgloss.Width(st))
	return lipgloss.NewStyle().Width(m.width).Render(logo + strings.Repeat(" ", space) + st)
}

// renderThinking is the cli-sample spinner activity line shown while running.
func (m *modelUI) renderThinking() string {
	frames := []string{"✶", "✷", "✸", "✹", "✺", "✹", "✸", "✷"}
	sp := styles.AccentS.Render(frames[m.spinnerFrame%len(frames)])
	detail := []string{m.config.DefaultRunner, "esc to interrupt"}
	return "  " + sp + " " + styles.AccentS.Render("running…") + " " + styles.MutedS.Render("("+strings.Join(detail, " · ")+")")
}

// renderStatusLine is the cli-sample bottom bar: mode + cwd on the left, the
// selected feature tag on the right. Errors/notices take precedence.
func (m *modelUI) renderStatusLine() string {
	width := m.width
	full := lipgloss.NewStyle().Width(width)
	switch {
	case m.err != nil:
		return full.Render(styles.DangerS.Render("✗ " + util.Truncate(m.err.Error(), util.Max(10, width-4))))
	case m.copyFlash != "":
		return full.Render(styles.SuccessS.Render(util.Truncate(m.copyFlash, width)))
	case m.notice != "":
		return full.Render(styles.SuccessS.Render(util.Truncate(m.notice, width)))
	}
	mode := styles.SuccessS.Render(" [ready]")
	if m.running {
		mode = styles.WarningS.Render(" [busy]")
	}
	left := mode + styles.MutedS.Render("  "+util.CompactPath(m.ws.Root, 3))
	right := m.featureTag() + "  "
	pad := util.Max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return full.Render(left + strings.Repeat(" ", pad) + right)
}

func (m *modelUI) featureTag() string {
	feature, ok := m.selected()
	if !ok {
		return styles.MutedS.Render("(no session)")
	}
	phase := styles.PhaseLabel(m.states[feature.ID].state.Phase)
	return lipgloss.NewStyle().Foreground(styles.Text).Render(feature.ID) + styles.MutedS.Render(" · "+phase)
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
	chatHeight := util.Max(1, bodyHeight-headerLines)
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
	width := m.contentWidth()
	var parts []string
	m.inputYOffset = 0
	if strip := m.attach.View(width); strip != "" {
		parts = append(parts, strip)
		m.inputYOffset += lipgloss.Height(strip)
	}
	composer := m.input.View(width)
	m.inputYOffset += lipgloss.Height(composer) - m.input.Height()
	parts = append(parts, composer)
	boxed := inputBoxStyle.Width(width).Render(strings.Join(parts, "\n"))
	return lipgloss.NewStyle().
		Width(m.width).
		PaddingLeft(1).
		Render(boxed)
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
		hint = "/ commands · ↑↓ pick · tab accept · enter submit · esc cancel · paste keeps line breaks"
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
		he("↑ / ↓ (in / box)", "move the command suggestion · tab accepts"),
		he("K / J (in chat)", "extend bubble selection"),
		he("y (in chat)", "copy selected bubble(s) via OSC52"),
		he("mouse drag", "select & copy any text natively (always on)"),
		he("pgup / pgdn", "scroll the chat log"),
		he("q / ctrl+c", "quit"),
		he("esc / ?", "close this help"),
	}
}
