// Package tui is the interactive Thanos workbench. It composes crush-style
// component packages (chat, sidebar, dialog, input, attachments) into a
// chat-first layout: a role-attributed agent chat log on the left, and a right
// sidebar with the logo, the Feature→EC tree, and model/MCP info. The
// orchestrator/state logic is unchanged.
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

	tea "charm.land/bubbletea/v2"
	"github.com/tinhtran/thanos/internal/featuregraph"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/state"
	"github.com/tinhtran/thanos/internal/tui/attachments"
	"github.com/tinhtran/thanos/internal/tui/chat"
	"github.com/tinhtran/thanos/internal/tui/dialog"
	"github.com/tinhtran/thanos/internal/tui/input"
	"github.com/tinhtran/thanos/internal/tui/sidebar"
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
	clarify    *dialog.Clarify
	confirm    *dialog.Confirm
	showHelp   bool
	focus      focusMode
	selectMode bool

	// chat mouse selection: viewport screen rect + drag state
	chatX0, chatY0, chatW, chatH int
	centerHeaderLines            int
	dragging                     bool

	// inputY0 is the first rendered textarea row. Attachment and completion rows
	// are above it; View adds the textarea-relative cursor row exactly once.
	inputY0      int
	inputYOffset int

	// Right-sidebar tree geometry for mouse hit-testing.
	treeRows            []sidebar.Row
	treeY0              int
	sidebarX0, sidebarW int

	// create is the guided new-session / bugfix flow.
	create createFlow

	// refs are @-file references staged for the next run's context manifest.
	refs []string

	// plans holds each feature's execution-chunk plan for the sidebar tree.
	plans map[string]model.ExecutionPlan
	// ecCursor selects an EC within the selected feature's tree (-1 = the feature
	// row itself, not descended into its chunks).
	ecCursor int
}

// createFlow walks the user through title → description → acceptance before a
// session (or bugfix) is created. Steps are entered in the command box.
type createFlow struct {
	active   bool
	bugfix   bool
	parentID string
	step     int // 0 title, 1 description, 2 acceptance
	title    string
	desc     string
	accept   string
}

const (
	stepTitle = iota
	stepDesc
	stepAccept
)

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
	// In Bubble Tea v2 the alt-screen and mouse mode are declared by the model's
	// View (see modelUI.View), not via program options.
	options := []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithOutput(out),
	}
	if in != nil {
		if inputFile, ok := in.(*os.File); !ok || inputFile != os.Stdin {
			options = append(options, tea.WithInput(in))
		}
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
	m := &modelUI{
		ctx: ctx, ws: ws, version: version, config: config,
		features: features, states: states,
		chat:     chat.New(),
		input:    input.New(),
		attach:   attachments.New(),
		focus:    focusSessions,
		output:   os.Stdout,
		self:     self,
		ecCursor: -1,
	}
	m.refreshPlans()
	return m, nil
}

// refreshPlans loads each feature's execution plan for the sidebar tree.
func (m *modelUI) refreshPlans() {
	plans := make(map[string]model.ExecutionPlan, len(m.features))
	for _, f := range m.features {
		if plan, err := m.ws.ReadPlan(f.ID); err == nil {
			plans[f.ID] = plan
		}
	}
	m.plans = plans
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
		m.refreshPlans()
		m.relayout()
	case createdMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.notice = "Created session " + msg.id
		}
		return m, m.reload()
	case tea.PasteMsg:
		// Only the focused composer owns bracketed paste. A complete supported
		// file path is staged; every other paste is ordinary editable text.
		if m.focus == focusInput && !m.modalActive() {
			if id := m.selectedFeatureID(); id != "" {
				if path, ok := attachmentCandidate(msg.Content); ok {
					if att, err := attachments.SaveFile(m.ws, id, path); err == nil {
						m.err = nil
						m.attach.Add(att)
						return m, nil
					}
				}
			}
			cmd := m.input.Update(msg)
			if err := m.input.Err(); err != nil {
				m.err = err
			} else {
				m.err = nil
			}
			return m, cmd
		}
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.MouseWheelMsg:
		if !m.selectMode {
			mm := msg.Mouse()
			switch mm.Button {
			case tea.MouseWheelUp:
				m.chat.ScrollUp(3)
			case tea.MouseWheelDown:
				m.chat.ScrollDown(3)
			}
		}
	case tea.MouseClickMsg:
		if !m.selectMode {
			return m, m.handleMousePress(msg.Mouse())
		}
	case tea.MouseMotionMsg:
		if !m.selectMode && m.dragging {
			m.chat.ExtendAtViewportRow(msg.Mouse().Y - m.chatY0)
		}
	case tea.MouseReleaseMsg:
		if !m.selectMode {
			return m, m.handleMouseRelease()
		}
	}
	return m, nil
}

func (m *modelUI) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Always-on global keys.
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.selectMode {
		// In select mode the View reports MouseMode=None, so the terminal does
		// native text selection; esc resumes app mouse handling.
		if key == "esc" {
			m.selectMode = false
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
	if m.confirm != nil {
		m.confirm.Update(msg)
		if m.confirm.Done() {
			m.confirm = nil
		}
		return m, nil
	}
	if m.clarify != nil {
		m.clarify.Update(msg)
		if m.clarify.Done() {
			chosen, cancelled := m.clarify.Chosen(), m.clarify.Cancelled()
			m.clarify = nil
			if !cancelled && chosen != "" {
				if id := m.selectedFeatureID(); id != "" {
					return m.runCLI("clarify "+id, []string{"clarify", id, chosen})
				}
			}
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
			if m.create.active {
				m.cancelCreate()
				return m, nil
			}
			m.blurInput()
			return m, nil
		case "enter":
			if m.create.active {
				return m.advanceCreate()
			}
			return m.submitCommand()
		case "tab":
			if m.create.active {
				return m, nil // stay in the form; enter advances
			}
			m.cycleFocus()
			return m, nil
		case "backspace":
			if m.input.Value() == "" && m.attach.Len() > 0 {
				m.attach.RemoveLast()
				return m, nil
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
		// Enter native mouse-select mode; View will set MouseMode=None so the
		// terminal handles drag-selection of text.
		m.selectMode = true
		m.chat.ClearSelection()
		return m, nil
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

func (m *modelUI) handleSessionsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "up", "k":
		m.treeMove(-1)
	case "down", "j":
		m.treeMove(1)
	case "right", "l":
		if m.ecCursor < 0 && len(m.chunksFor(m.selectedFeatureID())) > 0 {
			m.ecCursor = 0
		}
	case "left", "h":
		m.ecCursor = -1
	case "home", "g":
		m.cursor, m.ecCursor = 0, -1
	case "end", "G":
		if len(m.features) > 0 {
			m.cursor, m.ecCursor = len(m.features)-1, -1
		}
	case "enter", "space":
		if m.running {
			return m, nil
		}
		return m.startSelected()
	case "m":
		m.cycleRunner()
	case "n":
		return m.startCreate(false, "", "")
	case "d":
		if err := m.markDone(); err != nil {
			m.err = err
		} else {
			m.notice = "Session approved and marked done."
			return m, m.reload()
		}
	case "x":
		return m.removeSelectedEC()
	case "c":
		m.openClarify()
	case "r":
		return m, m.reload()
	default:
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			if index := int(key[0] - '1'); index < len(m.features) {
				m.cursor, m.ecCursor = index, -1
			}
		}
	}
	return m, nil
}

// treeMove moves the selection within the sidebar tree: across features when at
// the feature level, or across the selected feature's ECs when descended.
func (m *modelUI) treeMove(delta int) {
	if m.ecCursor >= 0 {
		chunks := m.chunksFor(m.selectedFeatureID())
		next := m.ecCursor + delta
		if next < 0 {
			m.ecCursor = -1 // back up to the feature row
		} else if next < len(chunks) {
			m.ecCursor = next
		}
		return
	}
	if delta < 0 && m.cursor > 0 {
		m.cursor--
	} else if delta > 0 && m.cursor < len(m.features)-1 {
		m.cursor++
	}
	m.ecCursor = -1
}

func (m *modelUI) chunksFor(featureID string) []model.ExecutionChunk {
	if featureID == "" {
		return nil
	}
	return m.plans[featureID].ActiveChunks()
}

// removeSelectedEC removes the highlighted execution chunk from the plan.
func (m *modelUI) removeSelectedEC() (tea.Model, tea.Cmd) {
	if m.ecCursor < 0 {
		m.notice = "Select an EC (→ then ↑↓) before removing."
		return m, nil
	}
	feature, ok := m.selected()
	if !ok {
		return m, nil
	}
	chunks := m.chunksFor(feature.ID)
	if m.ecCursor >= len(chunks) {
		return m, nil
	}
	target := chunks[m.ecCursor]
	plan := m.plans[feature.ID]
	for i := range plan.Chunks {
		if plan.Chunks[i].ID == target.ID {
			plan.Chunks[i].Status = "removed"
		}
	}
	if err := m.ws.WritePlan(feature.ID, plan); err != nil {
		m.err = err
		return m, nil
	}
	m.ecCursor = -1
	m.notice = "Removed EC-" + fmt.Sprint(target.Index)
	return m, m.reload()
}

// openClarify loads the selected feature's pending clarify.json into a popup.
func (m *modelUI) openClarify() {
	feature, ok := m.selected()
	if !ok {
		return
	}
	snap := m.states[feature.ID]
	name := "clarify.json"
	if snap.ok && snap.state.ECTotal > 1 && snap.state.ECIndex >= 1 {
		name = fmt.Sprintf("ec-%d/clarify.json", snap.state.ECIndex)
	}
	raw, err := m.ws.ReadArtifact(feature.ID, name)
	if err != nil {
		m.notice = "No pending clarification for this session."
		return
	}
	var q struct {
		Question string   `json:"question"`
		Options  []string `json:"options"`
	}
	if err := json.Unmarshal([]byte(raw), &q); err != nil || q.Question == "" {
		m.notice = "Clarification file is malformed."
		return
	}
	if len(q.Options) == 0 {
		q.Options = []string{"proceed"}
	}
	c := dialog.NewClarify(q.Question, q.Options)
	m.clarify = &c
}

func (m *modelUI) handleChatKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
			return m, m.copyText(text)
		}
	}
	return m, nil
}

func (m *modelUI) submitCommand() (tea.Model, tea.Cmd) {
	m.collectRefs(m.input.Value())
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
		return m.startCreate(false, "", arg)
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
			return m, m.copyText(text)
		}
		m.notice = "Select a chat bubble first (tab to the chat, ↑↓ to pick)."
	case "/select":
		m.selectMode = true
		m.chat.ClearSelection()
	case "/clear":
		m.attach.Clear()
		m.refs = nil
	case "/help":
		m.showHelp = true
	case "/quit":
		return m, tea.Quit

	case "/bugfix":
		feature, ok := m.selected()
		if !ok {
			m.err = errors.New("select a session to base the bugfix on")
			return m, nil
		}
		return m.startCreate(true, feature.ID, arg)

	// --- Thanos CLI passthrough (streamed into the chat) ---
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

// handleMousePress handles a left-click: select a session row in the left pane,
// or begin a chat-bubble selection in the center pane.
func (m *modelUI) handleMousePress(mm tea.Mouse) tea.Cmd {
	if m.picker != nil || m.showHelp || m.clarify != nil || m.confirm != nil || mm.Button != tea.MouseLeft {
		return nil
	}

	// Right sidebar → select a feature or EC row in the tree.
	if mm.X >= m.sidebarX0 && mm.X < m.sidebarX0+m.sidebarW {
		row := mm.Y - m.treeY0
		if row >= 0 && row < len(m.treeRows) {
			r := m.treeRows[row]
			if r.FeatureIndex >= 0 && r.FeatureIndex < len(m.features) {
				m.cursor = r.FeatureIndex
				m.ecCursor = r.ECIndex
				m.focus = focusSessions
				m.chat.Blur()
			}
		}
		return nil
	}
	// Chat pane → begin a bubble selection drag.
	if m.inChat(mm.X, mm.Y) {
		if m.chat.SelectAtViewportRow(mm.Y - m.chatY0) {
			m.focus = focusChat
			m.chat.Focus()
			m.dragging = true
		}
	}
	return nil
}

// handleMouseRelease ends a drag and copies the selection to the clipboard.
func (m *modelUI) handleMouseRelease() tea.Cmd {
	if !m.dragging {
		return nil
	}
	m.dragging = false
	if text := m.chat.SelectedText(); text != "" {
		return m.copyText(text)
	}
	return nil
}

// inChat reports whether a screen coordinate falls inside the chat viewport.
func (m *modelUI) inChat(x, y int) bool {
	return m.chatH > 0 &&
		x >= m.chatX0 && x < m.chatX0+m.chatW &&
		y >= m.chatY0 && y < m.chatY0+m.chatH
}

// copyText sets the clipboard via OSC52 and flashes a confirmation in the footer.
func (m *modelUI) copyText(text string) tea.Cmd {
	m.copyFlash = fmt.Sprintf("Copied %d line(s) to clipboard.", strings.Count(text, "\n")+1)
	return tea.SetClipboard(text)
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
	// Persist staged attachments + @-refs into the feature's context manifest so
	// the agent reads them (prompts.Render references it).
	if ok, _ := attachments.WriteManifest(m.ws, feature.ID, m.attach.Items(), m.refs); ok {
		m.notice = "Attached context passed to the agent."
	} else {
		attachments.ClearManifest(m.ws, feature.ID)
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

func (m *modelUI) selectedFeatureID() string {
	if f, ok := m.selected(); ok {
		return f.ID
	}
	return ""
}

func (m *modelUI) modalActive() bool {
	return m.showHelp || m.picker != nil || m.confirm != nil || m.clarify != nil
}

// collectRefs extracts @path tokens that resolve to existing workspace files and
// stages them for the next run's context manifest.
func (m *modelUI) collectRefs(text string) {
	for _, tok := range strings.Fields(text) {
		if !strings.HasPrefix(tok, "@") || len(tok) == 1 {
			continue
		}
		rel := strings.TrimPrefix(tok, "@")
		if info, err := os.Stat(filepath.Join(m.ws.Root, rel)); err != nil || info.IsDir() {
			continue
		}
		exists := false
		for _, r := range m.refs {
			if r == rel {
				exists = true
			}
		}
		if !exists {
			m.refs = append(m.refs, rel)
		}
	}
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

// --- guided new-session / bugfix flow ---------------------------------------

// startCreate begins the title → description → acceptance flow. When title is
// non-empty (e.g. "/new fix login") the flow skips straight to the description.
func (m *modelUI) startCreate(bugfix bool, parentID, title string) (tea.Model, tea.Cmd) {
	if m.running {
		m.err = errors.New("finish the running task before creating a session")
		return m, nil
	}
	m.create = createFlow{active: true, bugfix: bugfix, parentID: parentID}
	m.err, m.notice = nil, ""
	if t := strings.TrimSpace(title); t != "" {
		m.create.title = t
		m.create.step = stepDesc
	}
	m.focus = focusInput
	m.chat.Blur()
	m.input.Reset()
	m.input.SetCommandMode(false)
	m.input.SetPlaceholder(m.createPlaceholder())
	return m, m.input.Focus()
}

// advanceCreate stores the current field and moves to the next step, creating
// the session after the final (acceptance) step.
func (m *modelUI) advanceCreate() (tea.Model, tea.Cmd) {
	value := m.input.Value()
	switch m.create.step {
	case stepTitle:
		if strings.TrimSpace(value) == "" {
			m.err = errors.New("a title is required")
			return m, nil
		}
		m.create.title = strings.TrimSpace(value)
	case stepDesc:
		m.create.desc = strings.TrimSpace(value)
	case stepAccept:
		m.create.accept = strings.TrimSpace(value)
	}
	m.input.Reset()
	if m.create.step < stepAccept {
		m.create.step++
		m.input.SetPlaceholder(m.createPlaceholder())
		return m, nil
	}
	spec := sessionSpec{
		title:       m.create.title,
		description: m.create.desc,
		acceptance:  splitAcceptance(m.create.accept),
		bugfix:      m.create.bugfix,
		parentID:    m.create.parentID,
	}
	m.cancelCreate()
	return m, m.createFeatureSpec(spec)
}

// cancelCreate exits the flow and restores the command box.
func (m *modelUI) cancelCreate() {
	m.create = createFlow{}
	m.input.Reset()
	m.input.SetCommandMode(true)
	m.input.SetPlaceholder(input.DefaultPlaceholder)
	m.blurInput()
}

func (m *modelUI) createPlaceholder() string {
	switch m.create.step {
	case stepTitle:
		if m.create.bugfix {
			return "Bugfix title (what is broken?)"
		}
		return "Session title (what should be built?)"
	case stepDesc:
		return "Task description — details, context, constraints (optional)"
	default:
		return "Acceptance criteria, separated by ; (optional)"
	}
}

type sessionSpec struct {
	title       string
	description string
	acceptance  []string
	bugfix      bool
	parentID    string
}

func (m *modelUI) createFeatureSpec(spec sessionSpec) tea.Cmd {
	return func() tea.Msg {
		title := strings.TrimSpace(spec.title)
		id, err := m.ws.NextFeatureID(title)
		if err != nil {
			return createdMsg{err: err}
		}
		ftype := "feature"
		if spec.bugfix {
			ftype = "bugfix"
		}
		feature := model.Feature{
			ID: id, Title: title, Type: ftype, Parent: spec.parentID,
			Description: spec.description, Acceptance: spec.acceptance,
			Priority: "medium", Status: "todo",
		}
		if spec.bugfix && spec.parentID != "" {
			if parent, perr := m.ws.LoadFeature(spec.parentID); perr == nil {
				_ = featuregraph.Sync(m.ws.DotDir(), parent)
			}
		}
		if err := m.ws.SaveFeature(feature); err != nil {
			return createdMsg{err: err}
		}
		if err := featuregraph.Sync(m.ws.DotDir(), feature); err != nil {
			return createdMsg{err: err}
		}
		return createdMsg{id: id}
	}
}

// createFeature creates a bare session from just a title (used by "/run <title>").
func (m *modelUI) createFeature(title string) tea.Cmd {
	return m.createFeatureSpec(sessionSpec{title: title})
}

// splitAcceptance turns a "; "-separated string into trimmed criteria.
func splitAcceptance(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ";") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
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

func attachmentCandidate(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return "", false
	}
	if strings.HasPrefix(value, `"`) || strings.HasPrefix(value, "'") ||
		strings.HasSuffix(value, `"`) || strings.HasSuffix(value, "'") {
		return "", false
	}
	if !strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "~/") && !strings.HasPrefix(value, "./") {
		return "", false
	}
	path := expandHome(value)
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return path, true
}

func expandHome(value string) string {
	if strings.HasPrefix(value, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, value[2:])
		}
	}
	return value
}
