// Package tui is the interactive Thanos workbench. It composes crush-style
// component packages (chat, list, dialog, input, attachments) into a three-pane
// layout: sessions on the left, a role-attributed agent chat log in the center,
// and capabilities on the right. The orchestrator/state logic is unchanged.
package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tinhtran/thanos/internal/featuregraph"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/state"
	"github.com/tinhtran/thanos/internal/tui/attachments"
	"github.com/tinhtran/thanos/internal/tui/chat"
	"github.com/tinhtran/thanos/internal/tui/dialog"
	"github.com/tinhtran/thanos/internal/tui/input"
	"github.com/tinhtran/thanos/internal/tui/list"
	"github.com/tinhtran/thanos/internal/tui/styles"
	"github.com/tinhtran/thanos/internal/tui/util"
	"github.com/tinhtran/thanos/internal/workspace"
)

// focusMode tracks which pane currently receives keyboard input.
type focusMode int

const (
	focusSessions focusMode = iota
	focusChat
	focusInput
)

type stateSnapshot struct {
	state model.State
	ok    bool
}

type modelUI struct {
	ctx     context.Context
	ws      *workspace.Workspace
	version string
	output  io.Writer
	self    string // path to the thanos binary, for CLI passthrough commands

	config   model.Config
	features []model.Feature
	states   map[string]stateSnapshot
	cursor   int
	width    int
	height   int

	running      bool
	runningID    string
	spinnerFrame int
	lastOutput   string
	activity     chan string
	eventCount   int
	notice       string
	copyFlash    string
	err          error

	// components
	chat   chat.Model
	input  input.Model
	attach attachments.Model

	// overlays / modes
	picker     *dialog.FeaturePicker
	showHelp   bool
	focus      focusMode
	selectMode bool

	// chat mouse selection: viewport screen rect + drag state
	chatX0, chatY0, chatW, chatH int
	centerHeaderLines            int
	dragging                     bool
}

type tickMsg time.Time

type runFinishedMsg struct {
	output string
	err    error
}

type activityMsg string
type activityClosedMsg struct{}

type eventsMsg struct{ events []model.Event }

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

// Run starts the interactive TUI. Its signature is part of the public surface
// used by internal/cli and must remain stable.
func Run(ctx context.Context, ws *workspace.Workspace, version string, in io.Reader, out io.Writer) error {
	m, err := newModel(ctx, ws, version)
	if err != nil {
		return err
	}
	m.output = out
	options := []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithOutput(out),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	}
	if in == nil {
		options = append(options, tea.WithInput(nil))
	} else if inputFile, ok := in.(*os.File); !ok || inputFile != os.Stdin {
		options = append(options, tea.WithInput(in))
	}
	program := tea.NewProgram(m, options...)
	_, err = program.Run()
	return err
}

func newModel(ctx context.Context, ws *workspace.Workspace, version string) (*modelUI, error) {
	config, features, states, err := loadWorkspace(ws)
	if err != nil {
		return nil, err
	}
	self, err := os.Executable()
	if err != nil || self == "" {
		self = "thanos"
	}
	return &modelUI{
		ctx: ctx, ws: ws, version: version, config: config,
		features: features, states: states,
		chat:   chat.New(),
		input:  input.New(),
		attach: attachments.New(),
		focus:  focusSessions,
		output: os.Stdout,
		self:   self,
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

func (m *modelUI) Init() tea.Cmd { return tick() }

func tick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(now time.Time) tea.Msg { return tickMsg(now) })
}

func (m *modelUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.relayout()
	case tickMsg:
		if m.running {
			m.spinnerFrame++
			if m.runningID != "" {
				return m, tea.Batch(tick(), m.tailEvents())
			}
		}
		return m, tick()
	case runFinishedMsg:
		wasAgent := m.runningID != ""
		m.running = false
		m.runningID = ""
		m.lastOutput = strings.TrimSpace(msg.output)
		m.err = msg.err
		m.chat.CloseOpen()
		if msg.err != nil {
			m.chat.AddError(util.Truncate(msg.err.Error(), 400))
		} else if wasAgent {
			m.notice = "Task stopped at its next human or workflow boundary."
			m.chat.AddNotice(m.notice)
		} else {
			m.notice = "Command finished."
		}
		return m, m.reload()
	case activityMsg:
		if !m.running {
			return m, nil
		}
		stripped := util.StripANSI(strings.ReplaceAll(string(msg), "\r", "\n"))
		m.lastOutput = appendActivity(m.lastOutput, string(msg))
		m.chat.Append(stripped)
		return m, waitForActivity(m.activity)
	case activityClosedMsg:
		return m, nil
	case eventsMsg:
		if len(msg.events) > m.eventCount {
			for _, ev := range msg.events[m.eventCount:] {
				m.chat.OnEvent(ev)
			}
			m.eventCount = len(msg.events)
		}
		return m, nil
	case reloadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.config, m.features, m.states = msg.config, msg.features, msg.states
		if m.cursor >= len(m.features) {
			m.cursor = util.Max(0, len(m.features)-1)
		}
		m.relayout()
	case createdMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.notice = "Created session " + msg.id
		}
		return m, m.reload()
	case util.CopiedMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.copyFlash = fmt.Sprintf("Copied %d line(s) to clipboard.", msg.Lines)
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		if !m.selectMode {
			return m, m.handleMouse(msg)
		}
	}
	return m, nil
}

func (m *modelUI) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Always-on global keys.
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.selectMode {
		if key == "esc" {
			m.selectMode = false
			return m, tea.EnableMouseCellMotion
		}
		return m, nil
	}

	// Modal overlays take keys first.
	if m.showHelp {
		if key == "esc" || key == "?" || key == "q" {
			m.showHelp = false
		}
		return m, nil
	}
	if m.picker != nil {
		cmd := m.picker.Update(msg)
		if m.picker.Done() {
			if !m.picker.Cancelled() {
				m.selectFeatureByID(m.picker.Chosen())
			}
			m.picker = nil
		}
		return m, cmd
	}

	// Command input focus: typing + submit.
	if m.focus == focusInput {
		switch key {
		case "esc":
			m.blurInput()
			return m, nil
		case "enter":
			return m.submitCommand()
		case "tab":
			m.cycleFocus()
			return m, nil
		case "backspace":
			if m.input.Value() == "" && m.attach.Len() > 0 {
				m.attach.RemoveLast()
				return m, nil
			}
		}
		// Paste of a filesystem path becomes an attachment.
		if msg.Type == tea.KeyRunes {
			if pasted := string(msg.Runes); looksLikePath(pasted) && m.runningID != "" {
				if att, err := attachments.SaveFile(m.ws, m.runningID, strings.TrimSpace(pasted)); err == nil {
					m.attach.Add(att)
					return m, nil
				}
			}
		}
		cmd := m.input.Update(msg)
		return m, cmd
	}

	// Global keys (non-input focus).
	switch key {
	case "q":
		return m, tea.Quit
	case "ctrl+p":
		m.openPicker()
		return m, nil
	case "?":
		m.showHelp = true
		return m, nil
	case "ctrl+s":
		m.selectMode = true
		return m, tea.DisableMouse
	case "tab":
		m.cycleFocus()
		return m, nil
	case "/", ":":
		m.focus = focusInput
		m.chat.Blur()
		return m, m.input.Focus()
	case "pgup":
		m.chat.ScrollUp(5)
		return m, nil
	case "pgdown":
		m.chat.ScrollDown(5)
		return m, nil
	}

	if m.focus == focusChat {
		return m.handleChatKey(msg)
	}
	return m.handleSessionsKey(msg)
}

func (m *modelUI) handleSessionsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.features)-1 {
			m.cursor++
		}
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		if len(m.features) > 0 {
			m.cursor = len(m.features) - 1
		}
	case "enter", " ":
		if m.running {
			return m, nil
		}
		return m.startSelected()
	case "m":
		m.cycleRunner()
	case "n":
		m.focus = focusInput
		m.chat.Blur()
		m.input.SetValue("/new ")
		return m, m.input.Focus()
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
	if runes := msg.Runes; len(runes) == 1 && runes[0] >= '1' && runes[0] <= '9' {
		if index := int(runes[0] - '1'); index < len(m.features) {
			m.cursor = index
		}
	}
	return m, nil
}

func (m *modelUI) handleChatKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.chat.SelectMove(-1)
	case "down", "j":
		m.chat.SelectMove(1)
	case "K", "shift+up":
		m.chat.SelectExtend(-1)
	case "J", "shift+down":
		m.chat.SelectExtend(1)
	case "esc":
		m.chat.ClearSelection()
		m.focus = focusSessions
		m.chat.Blur()
	case "y", "enter":
		if text := m.chat.SelectedText(); text != "" {
			return m, util.Copy(m.output, text)
		}
	}
	return m, nil
}

func (m *modelUI) submitCommand() (tea.Model, tea.Cmd) {
	cmd, arg := input.Parse(m.input.Value())
	m.input.Reset()
	m.blurInput()
	switch cmd {
	// --- UI-native commands ---
	case "/run":
		if arg != "" {
			return m, m.createFeature(arg)
		}
		return m.startSelected()
	case "/continue":
		return m.startContinueSelected()
	case "/new":
		if strings.TrimSpace(arg) == "" {
			m.err = errors.New("usage: /new <title>")
			return m, nil
		}
		return m, m.createFeature(arg)
	case "/runner":
		if strings.HasPrefix(arg, "add ") || arg == "add" {
			return m.runCLI("runner "+arg, append([]string{"runner"}, strings.Fields(arg)...))
		}
		m.setRunner(arg)
	case "/approve", "/done":
		if err := m.markDone(); err != nil {
			m.err = err
		} else {
			m.notice = "Session approved and marked done."
			return m, m.reload()
		}
	case "/find":
		m.openPicker()
	case "/copy":
		if text := m.chat.SelectedText(); text != "" {
			return m, util.Copy(m.output, text)
		}
		m.notice = "Select a chat bubble first (tab to the chat, ↑↓ to pick)."
	case "/select":
		m.selectMode = true
		return m, tea.DisableMouse
	case "/clear":
		m.attach.Clear()
	case "/help":
		m.showHelp = true
	case "/quit":
		return m, tea.Quit

	// --- Thanos CLI passthrough (streamed into the chat) ---
	case "/bugfix":
		feature, ok := m.selected()
		if !ok || strings.TrimSpace(arg) == "" {
			m.err = errors.New("usage: select a session, then /bugfix <title>")
			return m, nil
		}
		return m.runCLI("bugfix "+feature.ID+" "+arg,
			append([]string{"bugfix", feature.ID}, strings.Fields(arg)...))
	case "/transition":
		return m.runCLIForSelected("transition", "transition", arg, "usage: /transition <phase>")
	case "/prompt":
		return m.runCLIForSelected("prompt", "prompt", arg, "usage: /prompt <role>")
	case "/status":
		return m.runCLI("status", []string{"status"})
	case "/scan":
		return m.runCLI("scan", []string{"scan"})
	case "/doctor":
		return m.runCLI("doctor", []string{"doctor"})
	case "/memory":
		if strings.TrimSpace(arg) == "" {
			if feature, ok := m.selected(); ok {
				return m.runCLI("memory "+feature.ID, []string{"memory", feature.ID})
			}
			return m.runCLI("memory", []string{"memory"})
		}
		return m.runCLI("memory "+arg, append([]string{"memory"}, strings.Fields(arg)...))
	case "/skill", "/plugin", "/lsp", "/mcp":
		name := strings.TrimPrefix(cmd, "/")
		if strings.TrimSpace(arg) == "" {
			m.err = fmt.Errorf("usage: %s <args> (see help)", cmd)
			return m, nil
		}
		return m.runCLI(name+" "+arg, append([]string{name}, strings.Fields(arg)...))

	default:
		m.err = fmt.Errorf("unknown command %q", cmd)
	}
	return m, nil
}

// runCLIForSelected runs a CLI subcommand that takes the selected feature ID as
// its first argument followed by the user-supplied arg(s).
func (m *modelUI) runCLIForSelected(label, sub, arg, usage string) (tea.Model, tea.Cmd) {
	feature, ok := m.selected()
	if !ok || strings.TrimSpace(arg) == "" {
		m.err = errors.New(usage)
		return m, nil
	}
	return m.runCLI(label+" "+feature.ID+" "+arg,
		append([]string{sub, feature.ID}, strings.Fields(arg)...))
}

func (m *modelUI) handleMouse(msg tea.MouseMsg) tea.Cmd {
	if m.picker != nil || m.showHelp {
		return nil
	}
	event := tea.MouseEvent(msg)
	switch event.Button {
	case tea.MouseButtonWheelUp:
		m.chat.ScrollUp(3)
		return nil
	case tea.MouseButtonWheelDown:
		m.chat.ScrollDown(3)
		return nil
	case tea.MouseButtonLeft:
		// nothing special for other buttons
	default:
		return nil
	}

	switch event.Action {
	case tea.MouseActionPress:
		// Left column → select a session row (preserves existing behavior).
		if event.X >= 0 && event.X < m.leftWidth() {
			if index := list.IndexAtY(event.Y, len(m.features)); index >= 0 {
				m.cursor = index
				m.focus = focusSessions
				m.chat.Blur()
			}
			return nil
		}
		// Chat pane → begin a bubble selection.
		if m.inChat(event.X, event.Y) {
			if m.chat.SelectAtViewportRow(event.Y - m.chatY0) {
				m.focus = focusChat
				m.chat.Focus()
				m.dragging = true
			}
		}
		return nil
	case tea.MouseActionMotion:
		if m.dragging {
			m.chat.ExtendAtViewportRow(event.Y - m.chatY0)
		}
		return nil
	case tea.MouseActionRelease:
		if m.dragging {
			m.dragging = false
			if text := m.chat.SelectedText(); text != "" {
				return util.Copy(m.output, text)
			}
		}
		return nil
	}
	return nil
}

// inChat reports whether a screen coordinate falls inside the chat viewport.
func (m *modelUI) inChat(x, y int) bool {
	return m.chatH > 0 &&
		x >= m.chatX0 && x < m.chatX0+m.chatW &&
		y >= m.chatY0 && y < m.chatY0+m.chatH
}

// --- focus helpers -----------------------------------------------------------

func (m *modelUI) cycleFocus() {
	switch m.focus {
	case focusSessions:
		m.focus = focusChat
		m.chat.Focus()
	case focusChat:
		m.focus = focusInput
		m.chat.Blur()
		_ = m.input.Focus()
	default:
		m.focus = focusSessions
		m.input.Blur()
		m.chat.Blur()
	}
}

func (m *modelUI) blurInput() {
	m.input.Blur()
	m.focus = focusSessions
}

func (m *modelUI) openPicker() {
	rows := make([]dialog.FeatureRow, 0, len(m.features))
	for _, f := range m.features {
		phase := "not started"
		if snap := m.states[f.ID]; snap.ok {
			phase = styles.PhaseLabel(snap.state.Phase)
		}
		rows = append(rows, dialog.FeatureRow{ID: f.ID, Title: f.Title, Phase: phase})
	}
	picker := dialog.NewFeaturePicker(rows, m.width, m.height)
	m.picker = &picker
}

func (m *modelUI) selectFeatureByID(id string) {
	for i, f := range m.features {
		if f.ID == id {
			m.cursor = i
			return
		}
	}
}

// --- run/state commands ------------------------------------------------------

func (m *modelUI) startSelected() (tea.Model, tea.Cmd) {
	feature, ok := m.selected()
	if !ok {
		return m, nil
	}
	return m.startAgent(feature, "run")
}

func (m *modelUI) startContinueSelected() (tea.Model, tea.Cmd) {
	feature, ok := m.selected()
	if !ok {
		return m, nil
	}
	return m.startAgent(feature, "continue")
}

// startAgent runs `thanos <sub> <feature>` as a subprocess and streams its
// output into the chat, tailing the feature's events.jsonl so each phase role
// appears as its own bubble (identical for run and continue).
func (m *modelUI) startAgent(feature model.Feature, sub string) (tea.Model, tea.Cmd) {
	if m.running {
		m.err = errors.New("a task is already running")
		return m, nil
	}
	args := []string{sub, feature.ID}
	if feature.Runner != "" {
		args = append(args, "--runner", feature.Runner)
	}
	m.running = true
	m.runningID = feature.ID
	m.activity = make(chan string, 256)
	m.err, m.notice, m.lastOutput, m.copyFlash = nil, "", "", ""
	m.chat.Reset()
	m.eventCount = countEvents(m.ws, feature.ID)
	return m, tea.Batch(m.runCommand(m.self, args, m.activity), waitForActivity(m.activity))
}

// runCLI runs an arbitrary `thanos` subcommand as a subprocess and streams its
// output into a labeled command bubble (no event tailing).
func (m *modelUI) runCLI(label string, args []string) (tea.Model, tea.Cmd) {
	if m.running {
		m.err = errors.New("a task is already running")
		return m, nil
	}
	m.running = true
	m.runningID = ""
	m.activity = make(chan string, 256)
	m.err, m.notice, m.lastOutput, m.copyFlash = nil, "", "", ""
	m.chat.StartCommand("thanos " + label)
	return m, tea.Batch(m.runCommand(m.self, args, m.activity), waitForActivity(m.activity))
}

func (m *modelUI) runCommand(name string, args []string, activity chan string) tea.Cmd {
	ctx, root := m.ctx, m.ws.Root
	return func() tea.Msg {
		defer close(activity)
		var output bytes.Buffer
		stream := &activityWriter{target: activity, output: &output}
		command := exec.CommandContext(ctx, name, args...)
		command.Dir = root
		command.Stdout = stream
		command.Stderr = stream
		err := command.Run()
		return runFinishedMsg{output: output.String(), err: err}
	}
}

func (m *modelUI) selected() (model.Feature, bool) {
	if m.cursor < 0 || m.cursor >= len(m.features) {
		return model.Feature{}, false
	}
	return m.features[m.cursor], true
}

type activityWriter struct {
	target chan<- string
	output io.Writer
	mu     sync.Mutex
}

func (w *activityWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.output != nil {
		if _, err := w.output.Write(data); err != nil {
			return 0, err
		}
	}
	if len(data) > 0 {
		w.target <- string(append([]byte(nil), data...))
	}
	return len(data), nil
}

func waitForActivity(activity <-chan string) tea.Cmd {
	return func() tea.Msg {
		value, ok := <-activity
		if !ok {
			return activityClosedMsg{}
		}
		return activityMsg(value)
	}
}

func appendActivity(current, update string) string {
	update = util.StripANSI(strings.ReplaceAll(update, "\r", "\n"))
	combined := current + update
	const maxActivityBytes = 64 * 1024
	if len(combined) > maxActivityBytes {
		combined = combined[len(combined)-maxActivityBytes:]
		if newline := strings.IndexByte(combined, '\n'); newline >= 0 {
			combined = combined[newline+1:]
		}
	}
	return combined
}

func (m *modelUI) tailEvents() tea.Cmd {
	id := m.runningID
	ws := m.ws
	return func() tea.Msg {
		return eventsMsg{events: readEvents(ws, id)}
	}
}

func countEvents(ws *workspace.Workspace, id string) int {
	return len(readEvents(ws, id))
}

func readEvents(ws *workspace.Workspace, id string) []model.Event {
	data, err := os.ReadFile(filepath.Join(ws.RuntimeDir(id), "events.jsonl"))
	if err != nil {
		return nil
	}
	var events []model.Event
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var ev model.Event
		if json.Unmarshal(line, &ev) == nil {
			events = append(events, ev)
		}
	}
	return events
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

func (m *modelUI) setRunner(name string) {
	feature, ok := m.selected()
	if !ok {
		return
	}
	if name == "" {
		m.cycleRunner()
		return
	}
	if _, exists := m.config.Runners[name]; !exists {
		m.err = fmt.Errorf("unknown runner %q", name)
		return
	}
	m.applyRunner(feature, name)
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
	m.applyRunner(feature, next)
}

func (m *modelUI) applyRunner(feature model.Feature, next string) {
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

func looksLikePath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\n") {
		return false
	}
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "~/") || strings.HasPrefix(value, "./") {
		if _, err := os.Stat(expandHome(value)); err == nil {
			return true
		}
	}
	return false
}

func expandHome(value string) string {
	if strings.HasPrefix(value, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, value[2:])
		}
	}
	return value
}
