package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tinhtran/thanos/internal/featuregraph"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/orchestrator"
	"github.com/tinhtran/thanos/internal/runner"
	"github.com/tinhtran/thanos/internal/state"
	"github.com/tinhtran/thanos/internal/workspace"
)

var (
	accent     = lipgloss.Color("#B48EFA")
	accentDim  = lipgloss.Color("#6E56A8")
	text       = lipgloss.Color("#E8E3F0")
	muted      = lipgloss.Color("#8B8498")
	subtle     = lipgloss.Color("#2B2731")
	success    = lipgloss.Color("#7FC8A9")
	warning    = lipgloss.Color("#E7B86B")
	danger     = lipgloss.Color("#E07A8D")
	titleStyle = lipgloss.NewStyle().Foreground(text).Bold(true)
	mutedStyle = lipgloss.NewStyle().Foreground(muted)
)

type stateSnapshot struct {
	state model.State
	ok    bool
}

type modelUI struct {
	ctx      context.Context
	ws       *workspace.Workspace
	version  string
	config   model.Config
	features []model.Feature
	states   map[string]stateSnapshot
	cursor   int
	width    int
	height   int

	running      bool
	spinnerFrame int
	lastOutput   string
	notice       string
	err          error

	creating bool
	input    string
}

type tickMsg time.Time

type runFinishedMsg struct {
	output string
	err    error
}

type reloadedMsg struct {
	config   model.Config
	features []model.Feature
	states   map[string]stateSnapshot
	err      error
}

type createdMsg struct {
	id  string
	err error
}

func Run(ctx context.Context, ws *workspace.Workspace, version string, input io.Reader, output io.Writer) error {
	m, err := newModel(ctx, ws, version)
	if err != nil {
		return err
	}
	program := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(input), tea.WithOutput(output), tea.WithAltScreen())
	_, err = program.Run()
	return err
}

func newModel(ctx context.Context, ws *workspace.Workspace, version string) (*modelUI, error) {
	config, features, states, err := loadWorkspace(ws)
	if err != nil {
		return nil, err
	}
	return &modelUI{
		ctx: ctx, ws: ws, version: version, config: config,
		features: features, states: states,
	}, nil
}

func loadWorkspace(ws *workspace.Workspace) (model.Config, []model.Feature, map[string]stateSnapshot, error) {
	config, err := ws.ReadConfig()
	if err != nil {
		return model.Config{}, nil, nil, err
	}
	features, err := ws.ListFeatures()
	if err != nil {
		return model.Config{}, nil, nil, err
	}
	states := make(map[string]stateSnapshot, len(features))
	for _, feature := range features {
		current, readErr := ws.ReadState(feature.ID)
		states[feature.ID] = stateSnapshot{state: current, ok: readErr == nil}
	}
	return config, features, states, nil
}

func (m *modelUI) Init() tea.Cmd {
	return tick()
}

func tick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(now time.Time) tea.Msg { return tickMsg(now) })
}

func (m *modelUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		if m.running {
			m.spinnerFrame++
		}
		return m, tick()
	case runFinishedMsg:
		m.running = false
		m.lastOutput = strings.TrimSpace(msg.output)
		m.err = msg.err
		if msg.err == nil {
			m.notice = "Task stopped at its next human or workflow boundary."
		}
		return m, m.reload()
	case reloadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.config, m.features, m.states = msg.config, msg.features, msg.states
		if m.cursor >= len(m.features) {
			m.cursor = intMax(0, len(m.features)-1)
		}
	case createdMsg:
		m.creating = false
		m.input = ""
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.notice = "Created session " + msg.id
		}
		return m, m.reload()
	case tea.KeyMsg:
		if m.creating {
			return m.handleCreateKey(msg)
		}
		if m.running {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.features)-1 {
				m.cursor++
			}
		case "enter", " ":
			if feature, ok := m.selected(); ok {
				m.running = true
				m.err, m.notice, m.lastOutput = nil, "", ""
				return m, m.runFeature(feature)
			}
		case "m":
			m.cycleRunner()
		case "n":
			m.creating = true
			m.input = ""
			m.err, m.notice = nil, ""
		case "d":
			if err := m.markDone(); err != nil {
				m.err = err
			} else {
				m.notice = "Session approved and marked done."
				return m, m.reload()
			}
		case "r":
			return m, m.reload()
		}
	}
	return m, nil
}

func (m *modelUI) handleCreateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.creating = false
		m.input = ""
	case "enter":
		title := strings.TrimSpace(m.input)
		if title == "" {
			m.err = errors.New("session title cannot be empty")
			return m, nil
		}
		return m, m.createFeature(title)
	case "backspace":
		runes := []rune(m.input)
		if len(runes) > 0 {
			m.input = string(runes[:len(runes)-1])
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.input += string(msg.Runes)
		}
	}
	return m, nil
}

func (m *modelUI) selected() (model.Feature, bool) {
	if m.cursor < 0 || m.cursor >= len(m.features) {
		return model.Feature{}, false
	}
	return m.features[m.cursor], true
}

func (m *modelUI) runFeature(feature model.Feature) tea.Cmd {
	runnerName := feature.Runner
	return func() tea.Msg {
		var output bytes.Buffer
		orch := orchestrator.Orchestrator{
			Workspace: m.ws,
			Runner:    runner.Subprocess{},
			Stdout:    &output,
			Stderr:    &output,
		}
		err := orch.Run(m.ctx, feature.ID, runnerName)
		return runFinishedMsg{output: output.String(), err: err}
	}
}

func (m *modelUI) reload() tea.Cmd {
	return func() tea.Msg {
		config, features, states, err := loadWorkspace(m.ws)
		return reloadedMsg{config: config, features: features, states: states, err: err}
	}
}

func (m *modelUI) createFeature(title string) tea.Cmd {
	return func() tea.Msg {
		id, err := m.ws.NextFeatureID(title)
		if err != nil {
			return createdMsg{err: err}
		}
		feature := model.Feature{ID: id, Title: title, Priority: "medium", Status: "todo"}
		if err := m.ws.SaveFeature(feature); err != nil {
			return createdMsg{err: err}
		}
		if err := featuregraph.Sync(m.ws.DotDir(), feature); err != nil {
			return createdMsg{err: err}
		}
		return createdMsg{id: id}
	}
}

func (m *modelUI) cycleRunner() {
	feature, ok := m.selected()
	if !ok || len(m.config.Runners) == 0 {
		return
	}
	names := sortedRunnerNames(m.config.Runners)
	current := feature.Runner
	if current == "" {
		current = m.config.DefaultRunner
	}
	next := names[0]
	for index, name := range names {
		if name == current {
			next = names[(index+1)%len(names)]
			break
		}
	}
	feature.Runner = next
	if err := m.ws.SaveFeature(feature); err != nil {
		m.err = err
		return
	}
	if snapshot := m.states[feature.ID]; snapshot.ok {
		currentState := snapshot.state
		currentState.Runner = next
		currentState.UpdatedAt = time.Now().UTC()
		if err := m.ws.WriteState(currentState); err != nil {
			m.err = err
			return
		}
		m.states[feature.ID] = stateSnapshot{state: currentState, ok: true}
	}
	m.features[m.cursor] = feature
	m.notice = "Switched this session to " + next + "; artifacts and phase context are preserved."
}

func (m *modelUI) markDone() error {
	feature, ok := m.selected()
	if !ok {
		return errors.New("no session selected")
	}
	snapshot := m.states[feature.ID]
	if !snapshot.ok || snapshot.state.Phase != model.PhasePending {
		return fmt.Errorf("%s is not pending human review", feature.ID)
	}
	current := snapshot.state
	current, err := state.Transition(current, model.PhaseDone)
	if err != nil {
		return err
	}
	feature.Status = "done"
	if err := m.ws.WriteState(current); err != nil {
		return err
	}
	if err := m.ws.SaveFeature(feature); err != nil {
		return err
	}
	return featuregraph.Sync(m.ws.DotDir(), feature)
}

func sortedRunnerNames(runners map[string]model.Runner) []string {
	names := make([]string, 0, len(runners))
	for name := range runners {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (m *modelUI) View() string {
	if m.width == 0 || m.height == 0 {
		return "Starting Thanos…"
	}
	header := m.renderHeader()
	footer := m.renderFooter()
	bodyHeight := intMax(4, m.height-lipgloss.Height(header)-lipgloss.Height(footer))

	leftWidth := intMin(32, intMax(24, m.width/4))
	rightWidth := 30
	gap := 2
	showRight := m.width >= 112
	centerWidth := m.width - leftWidth - gap
	if showRight {
		centerWidth -= rightWidth + gap
	}
	centerWidth = intMax(32, centerWidth)

	left := panelStyle(leftWidth, bodyHeight).Render(m.renderSessions(leftWidth - 4))
	center := panelStyle(centerWidth, bodyHeight).Render(m.renderTask(centerWidth - 4))
	columns := []string{left, center}
	if showRight {
		right := panelStyle(rightWidth, bodyHeight).Render(m.renderCapabilities(rightWidth - 4))
		columns = append(columns, right)
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, joinWithGap(columns, gap)...)
	return header + "\n" + body + "\n" + footer
}

func panelStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(intMax(1, width-2)).
		Height(intMax(1, height-2)).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(subtle)
}

func joinWithGap(columns []string, gap int) []string {
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

func (m *modelUI) renderHeader() string {
	project := m.config.Project.Name
	if project == "" {
		project = filepath.Base(m.ws.Root)
	}
	brand := lipgloss.NewStyle().Foreground(accent).Bold(true).Render("THANOS")
	meta := mutedStyle.Render(fmt.Sprintf("  %s  ·  %s  ·  v%s", project, compactPath(m.ws.Root, 3), m.version))
	status := ""
	if m.running {
		frames := []string{"◒", "◐", "◓", "◑"}
		status = lipgloss.NewStyle().Foreground(warning).Render(frames[m.spinnerFrame%len(frames)] + " agent running")
	} else {
		status = lipgloss.NewStyle().Foreground(success).Render("● ready")
	}
	space := intMax(1, m.width-lipgloss.Width(brand+meta)-lipgloss.Width(status))
	return lipgloss.NewStyle().Width(m.width).Render(brand + meta + strings.Repeat(" ", space) + status)
}

func (m *modelUI) renderSessions(width int) string {
	var b strings.Builder
	b.WriteString(sectionTitle("Work sessions"))
	b.WriteString("\n")
	if len(m.features) == 0 {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("No sessions yet.\nPress n to create one."))
		return b.String()
	}
	for index, feature := range m.features {
		snapshot := m.states[feature.ID]
		phase := "not started"
		if snapshot.ok {
			phase = shortPhase(snapshot.state.Phase)
		}
		cursor := "  "
		style := lipgloss.NewStyle().Foreground(text)
		if index == m.cursor {
			cursor = "› "
			style = style.Foreground(accent).Bold(true)
		}
		title := truncate(feature.Title, width-2)
		b.WriteString(style.Render(cursor + title))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render(truncate("  "+feature.ID+" · "+phase, width)))
		if index < len(m.features)-1 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

func (m *modelUI) renderTask(width int) string {
	if m.creating {
		return sectionTitle("New work session") + "\n\n" +
			mutedStyle.Render("Describe the task with a short title.") + "\n\n" +
			lipgloss.NewStyle().Foreground(accent).Render("› ") + m.input + "█" + "\n\n" +
			mutedStyle.Render("enter create  ·  esc cancel")
	}
	feature, ok := m.selected()
	if !ok {
		return sectionTitle("Task flow") + "\n\n" + mutedStyle.Render("Create a session to start the deterministic workflow.")
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
	var b strings.Builder
	b.WriteString(titleStyle.Render(feature.Title))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(feature.ID + "  ·  " + feature.Priority + " priority  ·  " + runnerName))
	if feature.Parent != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(warning).Render("bugfix of " + feature.Parent))
	}
	b.WriteString("\n\n")
	b.WriteString(sectionTitle("Flow"))
	b.WriteString("\n")
	b.WriteString(renderFlow(current.Phase, width))
	b.WriteString("\n\n")
	if feature.Description != "" {
		b.WriteString(sectionTitle("Brief"))
		b.WriteString("\n")
		b.WriteString(wrap(feature.Description, width))
		b.WriteString("\n\n")
	}
	b.WriteString(sectionTitle("Acceptance"))
	b.WriteString("\n")
	if len(feature.Acceptance) == 0 {
		b.WriteString(mutedStyle.Render("No acceptance criteria recorded. Edit the feature YAML before running for stronger gates."))
	} else {
		for _, item := range feature.Acceptance {
			b.WriteString(lipgloss.NewStyle().Foreground(success).Render("✓ "))
			b.WriteString(wrap(item, intMax(10, width-2)))
			b.WriteString("\n")
		}
	}
	if current.Reason != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(danger).Bold(true).Render("Stopped: "))
		b.WriteString(wrap(current.Reason, intMax(10, width-9)))
	}
	if memory, err := featuregraph.Resolve(m.ws.DotDir(), feature.ID); err == nil &&
		(len(memory.Rules) > 0 || len(memory.Decisions) > 0 || len(memory.Paths) > 0) {
		b.WriteString("\n\n")
		b.WriteString(sectionTitle("Impact memory"))
		b.WriteString("\n")
		fmt.Fprintf(&b, "%d rules · %d decisions · %d paths", len(memory.Rules), len(memory.Decisions), len(memory.Paths))
		for index, path := range memory.Paths {
			if index == 4 {
				fmt.Fprintf(&b, "\n%s", mutedStyle.Render(fmt.Sprintf("…and %d more", len(memory.Paths)-index)))
				break
			}
			fmt.Fprintf(&b, "\n%s", mutedStyle.Render("• "+truncate(path.Path, intMax(8, width-2))))
		}
	}
	if m.lastOutput != "" {
		b.WriteString("\n\n")
		b.WriteString(sectionTitle("Latest activity"))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render(truncateLines(m.lastOutput, width, 7)))
	}
	return b.String()
}

func renderFlow(current model.Phase, width int) string {
	phases := []model.Phase{
		model.PhaseDesign, model.PhaseDesignReview, model.PhaseCode, model.PhaseReview,
		model.PhaseTest, model.PhaseDeepReview, model.PhaseAccept, model.PhasePending, model.PhaseDone,
	}
	currentIndex := phaseIndex(current, phases)
	var parts []string
	for index, phase := range phases {
		label := shortPhase(phase)
		style := mutedStyle
		icon := "○"
		if current == model.PhaseAmend && phase == model.PhaseCode {
			style = lipgloss.NewStyle().Foreground(warning).Bold(true)
			icon = "↺"
		} else if index < currentIndex {
			style = lipgloss.NewStyle().Foreground(success)
			icon = "●"
		} else if index == currentIndex {
			style = lipgloss.NewStyle().Foreground(accent).Bold(true)
			icon = "◆"
		}
		parts = append(parts, style.Render(icon+" "+label))
	}
	var lines []string
	var line string
	for _, part := range parts {
		plainWidth := lipgloss.Width(part)
		if line != "" && lipgloss.Width(line)+3+plainWidth > width {
			lines = append(lines, line)
			line = part
		} else if line == "" {
			line = part
		} else {
			line += mutedStyle.Render(" ─ ") + part
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func phaseIndex(current model.Phase, phases []model.Phase) int {
	if current == model.PhaseInit {
		return 0
	}
	if current == model.PhaseAmend {
		return 2
	}
	for index, phase := range phases {
		if phase == current {
			return index
		}
	}
	return 0
}

func (m *modelUI) renderCapabilities(width int) string {
	feature, _ := m.selected()
	runnerName := feature.Runner
	if runnerName == "" {
		runnerName = m.config.DefaultRunner
	}
	var b strings.Builder
	b.WriteString(sectionTitle("Active model runner"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(accent).Bold(true).Render(runnerName))
	if configured, ok := m.config.Runners[runnerName]; ok {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render(truncate(configured.Command+" "+strings.Join(configured.Args, " "), width)))
	}
	b.WriteString("\n\n")
	b.WriteString(sectionTitle("Code intelligence"))
	b.WriteString("\n")
	graphPath := filepath.Join(m.ws.DotDir(), "codebase", "graph.json")
	if _, err := exec.LookPath("git"); err == nil {
		b.WriteString(statusLine(true, "Repository context"))
	}
	if fileExists(graphPath) {
		b.WriteString(statusLine(true, "Local code graph"))
	} else {
		b.WriteString(statusLine(false, "Local code graph"))
	}
	for _, name := range sortedLSPNames(m.config.LSP) {
		cfg := m.config.LSP[name]
		ok := !cfg.Disabled
		if ok {
			_, err := exec.LookPath(cfg.Command)
			ok = err == nil
		}
		b.WriteString(statusLine(ok, "LSP · "+name))
	}
	if len(m.config.LSP) == 0 {
		b.WriteString(mutedStyle.Render("\nNo LSP configured"))
	}
	b.WriteString("\n\n")
	b.WriteString(sectionTitle("Extensions"))
	for _, name := range sortedMCPNames(m.config.MCP) {
		cfg := m.config.MCP[name]
		b.WriteString(statusLine(!cfg.Disabled, "MCP · "+name+" ["+cfg.Type+"]"))
	}
	if len(m.config.MCP) == 0 {
		b.WriteString(mutedStyle.Render("\nNo MCP configured"))
	}
	for _, skill := range m.config.Skills {
		b.WriteString(statusLine(true, "Skill · "+skill.Name))
	}
	return b.String()
}

func (m *modelUI) renderFooter() string {
	message := ""
	switch {
	case m.err != nil:
		message = lipgloss.NewStyle().Foreground(danger).Render("error: " + truncate(m.err.Error(), intMax(10, m.width-8)))
	case m.notice != "":
		message = lipgloss.NewStyle().Foreground(success).Render(truncate(m.notice, m.width))
	default:
		message = mutedStyle.Render("↑↓ select  enter run/resume  m model  n new  d approve  r refresh  q quit")
	}
	return lipgloss.NewStyle().Width(m.width).Render(message)
}

func sectionTitle(value string) string {
	return lipgloss.NewStyle().Foreground(accentDim).Bold(true).Render(strings.ToUpper(value))
}

func statusLine(ok bool, label string) string {
	color, icon := muted, "○"
	if ok {
		color, icon = success, "●"
	}
	return "\n" + lipgloss.NewStyle().Foreground(color).Render(icon+" "+label)
}

func sortedLSPNames(items map[string]model.LSP) []string {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedMCPNames(items map[string]model.MCP) []string {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func shortPhase(phase model.Phase) string {
	switch phase {
	case model.PhaseDesign:
		return "design"
	case model.PhaseDesignReview:
		return "design review"
	case model.PhaseCode:
		return "code"
	case model.PhaseReview:
		return "review"
	case model.PhaseTest:
		return "test"
	case model.PhaseDeepReview:
		return "deep review"
	case model.PhaseAmend:
		return "amend"
	case model.PhaseAccept:
		return "accept"
	case model.PhasePending:
		return "human review"
	case model.PhaseDone:
		return "done"
	case model.PhaseBlocked:
		return "blocked"
	case model.PhaseAttention:
		return "attention"
	default:
		return "not started"
	}
}

func compactPath(path string, keep int) string {
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	if len(parts) <= keep {
		return path
	}
	return "…/" + strings.Join(parts[len(parts)-keep:], "/")
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

func wrap(value string, width int) string {
	if width <= 1 {
		return value
	}
	words := strings.Fields(value)
	var lines []string
	var line string
	for _, word := range words {
		if line != "" && len([]rune(line))+1+len([]rune(word)) > width {
			lines = append(lines, line)
			line = word
		} else if line == "" {
			line = word
		} else {
			line += " " + word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func truncateLines(value string, width, maxLines int) string {
	var lines []string
	for _, source := range strings.Split(value, "\n") {
		for _, line := range strings.Split(wrap(source, width), "\n") {
			lines = append(lines, line)
			if len(lines) == maxLines {
				return strings.Join(lines, "\n")
			}
		}
	}
	return strings.Join(lines, "\n")
}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
