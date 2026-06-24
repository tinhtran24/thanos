package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tinhtran/thanos/internal/codegraph"
	"github.com/tinhtran/thanos/internal/featuregraph"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/orchestrator"
	"github.com/tinhtran/thanos/internal/project"
	"github.com/tinhtran/thanos/internal/prompts"
	"github.com/tinhtran/thanos/internal/runner"
	"github.com/tinhtran/thanos/internal/state"
	"github.com/tinhtran/thanos/internal/tui"
	"github.com/tinhtran/thanos/internal/ui"
	"github.com/tinhtran/thanos/internal/workspace"
)

var (
	runExternal     = runExternalCommand
	detectFramework = project.DetectFramework
)

func Execute(ctx context.Context, args []string, version string, stdout, stderr io.Writer) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	ws := workspace.Open(root)
	if len(args) == 0 {
		if _, err := os.Stat(ws.ConfigPath()); err == nil {
			return tui.Run(ctx, ws, version, os.Stdin, stdout)
		}
		printHelp(stdout)
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		printHelp(stdout)
		return nil
	case "version", "--version":
		ui.Line(stdout, version)
		return nil
	case "init":
		return runInit(ctx, ws, args[1:], stdout)
	case "new":
		return runNew(ws, args[1:], stdout)
	case "bugfix":
		return runBugfix(ws, args[1:], stdout)
	case "status":
		return runStatus(ws, stdout)
	case "run":
		return runFeature(ctx, ws, args[1:], stdout, stderr)
	case "continue":
		return runContinue(ctx, ws, args[1:], stdout, stderr)
	case "prompt":
		return runPrompt(ws, args[1:], stdout)
	case "transition":
		return runTransition(ws, args[1:], stdout)
	case "done":
		return runDone(ws, args[1:], stdout)
	case "doctor":
		return runDoctor(ws, stdout)
	case "skill":
		return runSkill(ctx, ws, args[1:], stdout, stderr)
	case "plugin":
		return runPlugin(ctx, ws, args[1:], stdout, stderr)
	case "runner":
		return runRunner(ws, args[1:], stdout)
	case "scan":
		return runScan(ws, stdout)
	case "ui":
		return tui.Run(ctx, ws, version, os.Stdin, stdout)
	case "lsp":
		return runLSP(ws, args[1:], stdout)
	case "mcp":
		return runMCP(ws, args[1:], stdout)
	case "memory":
		return runMemory(ws, args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q; run 'thanos help'", args[0])
	}
}

func runInit(_ context.Context, ws *workspace.Workspace, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	name := flags.String("name", "", "project name override")
	language := flags.String("language", "", "project language override")
	framework := flags.String("framework", "", "project framework override")
	runnerName := flags.String("runner", "codex", "default runner")
	runnerCommand := flags.String("runner-command", "codex", "runner executable")
	if err := flags.Parse(args); err != nil {
		return err
	}
	hasSource := codegraph.HasSource(ws.Root)
	var graph modelGraph
	languages := map[string]int{}
	if hasSource {
		built, err := codegraph.Build(ws.Root)
		if err != nil {
			return fmt.Errorf("scan existing codebase: %w", err)
		}
		graph.value = built
		graph.present = true
		languages = built.Languages
	}
	detected, err := project.Detect(ws.Root, languages)
	if err != nil {
		return fmt.Errorf("detect project: %w", err)
	}
	if *name != "" {
		detected.Name = *name
	}
	if *language != "" {
		detected.Language = *language
	}
	detected.Framework, err = detectFramework(ws.Root, detected.Language)
	if err != nil {
		return fmt.Errorf("detect framework: %w", err)
	}
	if override := strings.TrimSpace(*framework); override != "" {
		detected.Framework = override
	}
	detected.Rules = []string{
		"Keep role outputs isolated in .thanos.",
		"Do not bypass the deterministic phase state machine.",
		"Every implementation change requires objective test evidence.",
	}
	config := model.Config{
		Project:       detected,
		DefaultRunner: *runnerName,
		MaxRounds:     3,
		Locale:        "en",
		Runners: map[string]model.Runner{
			*runnerName: {
				Command:   *runnerCommand,
				Args:      defaultRunnerArgs(*runnerName),
				Agent:     defaultRunnerAgent(*runnerName),
				SkillsDir: defaultSkillsDir(*runnerName),
			},
		},
		LSP: detectedLSPs(detected.Language),
	}
	if err := ws.Init(config); err != nil {
		return err
	}
	if err := featuregraph.Save(ws.DotDir(), featuregraph.Graph{}); err != nil {
		return fmt.Errorf("initialize feature memory: %w", err)
	}
	printExecLog(stdout, ui.ExecLogEntry{
		Type: "write", Path: ws.DotDir(), Message: "Initialized Thanos workspace", Status: ui.Completed,
	})
	if graph.present {
		if err := codegraph.Save(graph.value, ws.DotDir()); err != nil {
			return fmt.Errorf("save initial codebase graph: %w", err)
		}
		printExecLog(stdout, ui.ExecLogEntry{
			Type: "write",
			Path: filepath.Join(ws.DotDir(), "codebase", "graph.json"),
			Message: fmt.Sprintf("Indexed codebase: %d files, %d symbols, %d relationships",
				graph.value.Files, graph.value.Symbols, len(graph.value.Edges)),
			Status: ui.Completed,
		})
	}
	return nil
}

type modelGraph struct {
	value   codegraph.Graph
	present bool
}

func runScan(ws *workspace.Workspace, stdout io.Writer) error {
	if _, err := ws.ReadConfig(); err != nil {
		return fmt.Errorf("open Thanos workspace: %w", err)
	}
	graph, err := codegraph.Build(ws.Root)
	if err != nil {
		return err
	}
	if err := codegraph.Save(graph, ws.DotDir()); err != nil {
		return err
	}
	features, err := ws.ListFeatures()
	if err != nil {
		return err
	}
	if err := featuregraph.Rebuild(ws.DotDir(), features); err != nil {
		return err
	}
	for _, feature := range features {
		if err := featuregraph.UpdateFromArtifacts(ws.DotDir(), feature); err != nil {
			return err
		}
	}
	printExecLog(stdout, ui.ExecLogEntry{
		Type: "write",
		Path: filepath.Join(ws.DotDir(), "codebase", "graph.json"),
		Message: fmt.Sprintf("Indexed codebase: %d files, %d symbols, %d relationships",
			graph.Files, graph.Symbols, len(graph.Edges)),
		Status: ui.Completed,
	})
	printExecLog(stdout, ui.ExecLogEntry{
		Type:    "write",
		Path:    filepath.Join(ws.DotDir(), "codebase", "summary.md"),
		Message: "Codebase summary completed",
		Status:  ui.Completed,
	})
	printExecLog(stdout, ui.ExecLogEntry{
		Type: "write", Path: featuregraph.PathFor(ws.DotDir()),
		Message: "Refreshed feature memory", Status: ui.Completed,
	})
	return nil
}

func runNew(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("new", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	description := flags.String("description", "", "feature description")
	acceptance := flags.String("acceptance", "", "semicolon-separated acceptance criteria")
	featureType := flags.String("type", "feature", "feature type")
	parent := flags.String("parent", "", "parent feature for a bugfix")
	rules := flags.String("rules", "", "semicolon-separated business rules")
	decisions := flags.String("decisions", "", "semicolon-separated architectural decisions")
	scope := flags.String("scope", "", "semicolon-separated affected paths or scope")
	related := flags.String("related", "", "comma-separated related feature IDs")
	dependencies := flags.String("depends-on", "", "comma-separated dependency feature IDs")
	priority := flags.String("priority", "medium", "priority")
	args, err := intersperseFlags(args, map[string]bool{
		"--description": true,
		"--acceptance":  true,
		"--type":        true,
		"--parent":      true,
		"--rules":       true,
		"--decisions":   true,
		"--scope":       true,
		"--related":     true,
		"--depends-on":  true,
		"--priority":    true,
	})
	if err != nil {
		return err
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() == 0 {
		return errors.New("usage: thanos new \"Feature title\" [--description text]")
	}
	title := strings.Join(flags.Args(), " ")
	if *featureType != "feature" && *featureType != "bugfix" {
		return fmt.Errorf("unsupported feature type %q", *featureType)
	}
	parentID := ""
	if strings.TrimSpace(*parent) != "" {
		parentFeature, err := ws.LoadFeature(strings.TrimSpace(*parent))
		if err != nil {
			return fmt.Errorf("parent feature: %w", err)
		}
		parentID = parentFeature.ID
		if err := featuregraph.Sync(ws.DotDir(), parentFeature); err != nil {
			return fmt.Errorf("sync parent feature memory: %w", err)
		}
	}
	if *featureType == "bugfix" && parentID == "" {
		return errors.New("bugfix requires --parent FEATURE_ID")
	}
	id, err := ws.NextFeatureID(title)
	if err != nil {
		return err
	}
	criteria := splitList(*acceptance)
	feature := model.Feature{
		ID: id, Title: title, Type: *featureType, Parent: parentID,
		Description: *description, Acceptance: criteria, Rules: splitList(*rules),
		Decisions: splitList(*decisions),
		Scope:     splitList(*scope), Related: splitCSV(*related), Dependencies: splitCSV(*dependencies),
		Priority: *priority, Status: "todo",
	}
	if err := ws.SaveFeature(feature); err != nil {
		return err
	}
	if err := featuregraph.Sync(ws.DotDir(), feature); err != nil {
		return fmt.Errorf("save feature memory: %w", err)
	}
	message := "Created " + id
	if parentID != "" {
		message += " mapped to " + parentID
	}
	printExecLog(stdout, ui.ExecLogEntry{
		Type: "write", Path: ws.FeaturePath(id), Message: message, Status: ui.Completed,
	})
	return nil
}

func runBugfix(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: thanos bugfix FEATURE_ID \"Bugfix title\" [options]")
	}
	parent := args[0]
	bugfixArgs := append([]string{}, args[1:]...)
	bugfixArgs = append(bugfixArgs, "--type", "bugfix", "--parent", parent)
	return runNew(ws, bugfixArgs, stdout)
}

func runStatus(ws *workspace.Workspace, stdout io.Writer) error {
	features, err := ws.ListFeatures()
	if err != nil {
		return err
	}
	if len(features) == 0 {
		ui.Info(stdout, "No features.")
		return nil
	}
	rows := make([][]string, 0, len(features))
	for _, feature := range features {
		phase, round := "-", 0
		if current, err := ws.ReadState(feature.ID); err == nil {
			phase, round = string(current.Phase), current.Round
		}
		rows = append(rows, []string{
			feature.ID,
			feature.Status,
			phase,
			strconv.Itoa(round),
			feature.Title,
		})
	}
	ui.Block(stdout, ui.Table([]string{"ID", "STATUS", "PHASE", "ROUND", "TITLE"}, rows))
	return nil
}

func runFeature(ctx context.Context, ws *workspace.Workspace, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	runnerName := flags.String("runner", "", "runner override")
	args, err := intersperseFlags(args, map[string]bool{"--runner": true})
	if err != nil {
		return err
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: thanos run FEATURE_ID [--runner name]")
	}
	orch := orchestrator.Orchestrator{
		Workspace: ws, Runner: runner.Subprocess{}, Stdout: stdout, Stderr: stderr,
	}
	return orch.Run(ctx, flags.Arg(0), *runnerName)
}

func runContinue(ctx context.Context, ws *workspace.Workspace, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("continue", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	runnerName := flags.String("runner", "", "runner override")
	args, err := intersperseFlags(args, map[string]bool{"--runner": true})
	if err != nil {
		return err
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: thanos continue FEATURE_ID [--runner name]")
	}
	featureID, err := prepareContinue(ws, flags.Arg(0), stdout)
	if err != nil {
		return err
	}
	orch := orchestrator.Orchestrator{
		Workspace: ws, Runner: runner.Subprocess{}, Stdout: stdout, Stderr: stderr,
	}
	return orch.Run(ctx, featureID, *runnerName)
}

func prepareContinue(ws *workspace.Workspace, featureID string, stdout io.Writer) (string, error) {
	feature, err := ws.LoadFeature(featureID)
	if err != nil {
		return "", err
	}
	current, err := ws.ReadState(feature.ID)
	if err != nil {
		return "", err
	}
	failedRound, reportPath, err := latestFailedRound(ws, feature.ID, current.Round)
	if err != nil {
		return "", err
	}
	next, err := state.ResumeFailedRound(current, failedRound)
	if err != nil {
		return "", err
	}
	if err := ws.WriteState(next); err != nil {
		return "", err
	}
	_ = ws.AppendEvent(model.Event{
		Type:      "continue",
		FeatureID: feature.ID,
		Timestamp: time.Now().UTC(),
		Phase:     next.Phase,
		Role:      next.Role,
		Round:     next.Round,
		Data: map[string]any{
			"from":          current.Phase,
			"failed_report": reportPath,
		},
	})
	printExecLog(stdout, ui.ExecLogEntry{
		Type:    "read",
		Path:    reportPath,
		Message: fmt.Sprintf("Continuing failed round %d", failedRound),
		Status:  ui.Completed,
	})
	return feature.ID, nil
}

func latestFailedRound(ws *workspace.Workspace, featureID string, currentRound int) (int, string, error) {
	reportNames := []string{"deep-review-report.md", "test-report.md", "review-report.md"}
	for round := currentRound; round >= 1; round-- {
		for _, reportName := range reportNames {
			relative := filepath.Join("rounds", fmt.Sprintf("round-%d", round), reportName)
			content, err := ws.ReadArtifact(featureID, relative)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return 0, "", err
			}
			if reportVerdictFailed(content) {
				return round, filepath.Join(ws.RuntimeDir(featureID), relative), nil
			}
		}
	}
	return 0, "", fmt.Errorf("%s has no failed round report to continue", featureID)
}

func reportVerdictFailed(content string) bool {
	verdict := strings.ToUpper(content)
	if index := strings.LastIndex(verdict, "VERDICT"); index >= 0 {
		verdict = verdict[index:]
	}
	return strings.Contains(verdict, "FAIL")
}

func runPrompt(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	if len(args) != 2 {
		return errors.New("usage: thanos prompt FEATURE_ID ROLE")
	}
	feature, err := ws.LoadFeature(args[0])
	if err != nil {
		return err
	}
	config, err := ws.ReadConfig()
	if err != nil {
		return err
	}
	current, err := ws.EnsureRuntime(feature, config)
	if err != nil {
		return err
	}
	role := model.Role(args[1])
	current.Role = role
	current.Phase = phaseForRole(role)
	prompt, err := prompts.Render(role, prompts.Data{Feature: feature, Config: config, State: current, Root: ws.Root})
	if err != nil {
		return err
	}
	ui.Raw(stdout, prompt)
	return nil
}

func runTransition(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	if len(args) != 2 {
		return errors.New("usage: thanos transition FEATURE_ID PHASE")
	}
	feature, err := ws.LoadFeature(args[0])
	if err != nil {
		return err
	}
	config, err := ws.ReadConfig()
	if err != nil {
		return err
	}
	current, err := ws.EnsureRuntime(feature, config)
	if err != nil {
		return err
	}
	next, err := state.Transition(current, model.Phase(args[1]))
	if err != nil {
		return err
	}
	if err := ws.WriteState(next); err != nil {
		return err
	}
	printExecLog(stdout, ui.ExecLogEntry{
		Type:    "write",
		Path:    filepath.Join(ws.DotDir(), feature.ID, "state.json"),
		Message: fmt.Sprintf("%s: %s -> %s", feature.ID, current.Phase, next.Phase),
		Status:  ui.Completed,
	})
	return nil
}

func runDone(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: thanos done FEATURE_ID")
	}
	feature, err := ws.LoadFeature(args[0])
	if err != nil {
		return err
	}
	current, err := ws.ReadState(feature.ID)
	if err != nil {
		return err
	}
	if current.Phase != model.PhasePending {
		return fmt.Errorf("%s is %s, not pending-review", feature.ID, current.Phase)
	}
	next, err := state.Transition(current, model.PhaseDone)
	if err != nil {
		return err
	}
	feature.Status = "done"
	if err := ws.WriteState(next); err != nil {
		return err
	}
	if err := ws.SaveFeature(feature); err != nil {
		return err
	}
	if err := featuregraph.Sync(ws.DotDir(), feature); err != nil {
		return err
	}
	printExecLog(stdout, ui.ExecLogEntry{
		Type:    "write",
		Path:    filepath.Join(ws.DotDir(), feature.ID, "state.json"),
		Message: feature.ID + " marked done",
		Status:  ui.Completed,
	})
	return nil
}

func runMemory(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	if len(args) > 1 {
		return errors.New("usage: thanos memory [FEATURE_ID]")
	}
	if len(args) == 0 {
		graph, err := featuregraph.Load(ws.DotDir())
		if err != nil {
			return err
		}
		fmt.Fprint(stdout, featuregraph.Summary(graph))
		return nil
	}
	feature, err := ws.LoadFeature(args[0])
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, featuregraph.ContextMarkdown(ws.DotDir(), feature.ID))
	return nil
}

func runDoctor(ws *workspace.Workspace, stdout io.Writer) error {
	config, err := ws.ReadConfig()
	if err != nil {
		return err
	}
	type check struct {
		Name   string `json:"name"`
		OK     bool   `json:"ok"`
		Detail string `json:"detail"`
	}
	checks := []check{{Name: "workspace", OK: true, Detail: ws.DotDir()}}
	for name, configured := range config.Runners {
		_, lookupErr := exec.LookPath(configured.Command)
		checks = append(checks, check{Name: "runner:" + name, OK: lookupErr == nil, Detail: configured.Command})
	}
	for name, configured := range config.LSP {
		if configured.Disabled {
			continue
		}
		_, lookupErr := exec.LookPath(configured.Command)
		checks = append(checks, check{Name: "lsp:" + name, OK: lookupErr == nil, Detail: configured.Command})
	}
	for name, configured := range config.MCP {
		if configured.Disabled {
			continue
		}
		ok, detail := true, configured.Type
		switch configured.Type {
		case "stdio":
			_, lookupErr := exec.LookPath(configured.Command)
			ok, detail = lookupErr == nil, configured.Command
		case "http", "sse":
			ok, detail = configured.URL != "", configured.URL
		default:
			ok, detail = false, "unsupported transport "+configured.Type
		}
		checks = append(checks, check{Name: "mcp:" + name, OK: ok, Detail: detail})
	}
	rows := make([][]string, 0, len(checks))
	for _, item := range checks {
		status := "Success"
		if !item.OK {
			status = "Failed"
		}
		rows = append(rows, []string{item.Name, status, item.Detail})
	}
	ui.Block(stdout, ui.Section("Environment Checks", nil))
	ui.Block(stdout, ui.Table([]string{"CHECK", "STATUS", "DETAIL"}, rows))
	for _, item := range checks {
		if !item.OK {
			return fmt.Errorf("doctor found unavailable runner %s", item.Name)
		}
	}
	return nil
}

func runSkill(ctx context.Context, ws *workspace.Workspace, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: thanos skill add SOURCE [options] | thanos skill find [QUERY]")
	}
	switch args[0] {
	case "add":
		return runSkillAdd(ctx, ws, args[1:], stdout, stderr)
	case "find":
		commandArgs := []string{"--yes", "skills", "find"}
		commandArgs = append(commandArgs, args[1:]...)
		return runExternal(ctx, ws.Root, stdout, stderr, "npx", commandArgs...)
	default:
		return errors.New("usage: thanos skill add SOURCE [options] | thanos skill find [QUERY]")
	}
}

func runSkillAdd(ctx context.Context, ws *workspace.Workspace, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("skill add", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	rolesValue := flags.String("roles", "", "comma-separated roles; empty means all roles")
	agentValue := flags.String("agent", "universal", "skills CLI target agent")
	skillValue := flags.String("skill", "", "specific skill name")
	parsed, err := intersperseFlags(args, map[string]bool{
		"--roles": true, "--agent": true, "--skill": true,
	})
	if err != nil {
		return err
	}
	if err := flags.Parse(parsed); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: thanos skill add SOURCE [--skill NAME] [--agent AGENT] [--roles role,role]")
	}
	source := strings.TrimSpace(flags.Arg(0))
	roles, err := parseSkillRoles(*rolesValue)
	if err != nil {
		return err
	}
	before, err := discoverSkillFiles(ws.Root)
	if err != nil {
		return err
	}
	commandArgs := []string{"--yes", "skills", "add", source, "--agent", *agentValue, "--yes"}
	if *skillValue != "" {
		commandArgs = append(commandArgs, "--skill", *skillValue)
	}
	commandArgs = append(commandArgs, "--copy")
	if err := runExternal(ctx, ws.Root, stdout, stderr, "npx", commandArgs...); err != nil {
		return err
	}
	after, err := discoverSkillFiles(ws.Root)
	if err != nil {
		return err
	}
	config, err := ws.ReadConfig()
	if err != nil {
		return err
	}
	existingPaths := map[string]bool{}
	for _, skill := range config.Skills {
		existingPaths[skill.Path] = true
	}
	var added []model.Skill
	for path, name := range after {
		if before[path] != "" || existingPaths[path] {
			continue
		}
		added = append(added, model.Skill{Name: name, Path: path, Source: source, Roles: roles})
	}
	if len(added) == 0 && *skillValue != "" {
		for path, name := range after {
			if name == *skillValue && !existingPaths[path] {
				added = append(added, model.Skill{Name: name, Path: path, Source: source, Roles: roles})
			}
		}
	}
	if len(added) == 0 {
		return errors.New("skills CLI completed but no new SKILL.md files were discovered")
	}
	sort.Slice(added, func(i, j int) bool { return added[i].Path < added[j].Path })
	config.Skills = append(config.Skills, added...)
	if err := ws.WriteConfig(config); err != nil {
		return err
	}
	if err := syncSkillsToRunners(ws.Root, config, added); err != nil {
		return fmt.Errorf("skills installed and registered, but runner sync failed: %w", err)
	}
	scope := "all roles"
	if len(roles) > 0 {
		scope = strings.Join(roles, ",")
	}
	for _, skill := range added {
		printExecLog(stdout, ui.ExecLogEntry{
			Type:    "write",
			Path:    skill.Path,
			Message: fmt.Sprintf("Registered skill %s for %s", skill.Name, scope),
			Status:  ui.Completed,
		})
	}
	return nil
}

func runRunner(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "add" {
		return errors.New("usage: thanos runner add NAME [--command CMD] [--agent AGENT] [--skills-dir PATH]")
	}
	flags := flag.NewFlagSet("runner add", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	commandValue := flags.String("command", "", "runner executable")
	agentValue := flags.String("agent", "", "skills CLI agent identifier")
	skillsDirValue := flags.String("skills-dir", "", "project-relative native skills directory")
	parsed, err := intersperseFlags(args[1:], map[string]bool{
		"--command": true, "--agent": true, "--skills-dir": true,
	})
	if err != nil {
		return err
	}
	if err := flags.Parse(parsed); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: thanos runner add NAME [--command CMD] [--agent AGENT] [--skills-dir PATH]")
	}
	name := strings.ToLower(strings.TrimSpace(flags.Arg(0)))
	command := *commandValue
	if command == "" {
		command = name
	}
	agent := *agentValue
	if agent == "" {
		agent = defaultRunnerAgent(name)
	}
	skillsDir := *skillsDirValue
	if skillsDir == "" {
		skillsDir = defaultSkillsDir(name)
	}
	if skillsDir == "" {
		return fmt.Errorf("runner %q has no known skills directory; pass --skills-dir", name)
	}
	if filepath.IsAbs(skillsDir) || skillsDir == ".." || strings.HasPrefix(filepath.Clean(skillsDir), ".."+string(filepath.Separator)) {
		return errors.New("runner skills directory must stay inside the project")
	}
	config, err := ws.ReadConfig()
	if err != nil {
		return err
	}
	if config.Runners == nil {
		config.Runners = map[string]model.Runner{}
	}
	if _, exists := config.Runners[name]; exists {
		return fmt.Errorf("runner %q is already configured", name)
	}
	runnerConfig := model.Runner{
		Command:   command,
		Args:      defaultRunnerArgs(name),
		Agent:     agent,
		SkillsDir: filepath.ToSlash(filepath.Clean(skillsDir)),
	}
	if err := syncSkillsToRunner(ws.Root, runnerConfig, config.Skills); err != nil {
		return err
	}
	config.Runners[name] = runnerConfig
	if err := ws.WriteConfig(config); err != nil {
		return err
	}
	printExecLog(stdout, ui.ExecLogEntry{
		Type: "write",
		Path: filepath.Join(ws.DotDir(), "settings.json"),
		Message: fmt.Sprintf("Added runner %s; synchronized %d configured skills into %s",
			name, len(config.Skills), runnerConfig.SkillsDir),
		Status: ui.Completed,
	})
	return nil
}

func runLSP(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "add" {
		return errors.New("usage: thanos lsp add NAME --command CMD [--args \"arg,arg\"]")
	}
	flags := flag.NewFlagSet("lsp add", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	command := flags.String("command", "", "language server executable")
	commandArgs := flags.String("args", "", "comma-separated language server arguments")
	parsed, err := intersperseFlags(args[1:], map[string]bool{"--command": true, "--args": true})
	if err != nil {
		return err
	}
	if err := flags.Parse(parsed); err != nil {
		return err
	}
	if flags.NArg() != 1 || *command == "" {
		return errors.New("usage: thanos lsp add NAME --command CMD [--args \"arg,arg\"]")
	}
	config, err := ws.ReadConfig()
	if err != nil {
		return err
	}
	if config.LSP == nil {
		config.LSP = map[string]model.LSP{}
	}
	name := strings.ToLower(strings.TrimSpace(flags.Arg(0)))
	config.LSP[name] = model.LSP{Command: *command, Args: splitCSV(*commandArgs)}
	if err := ws.WriteConfig(config); err != nil {
		return err
	}
	printExecLog(stdout, ui.ExecLogEntry{
		Type: "write", Path: ws.ConfigPath(),
		Message: fmt.Sprintf("Configured LSP %s (%s)", name, *command), Status: ui.Completed,
	})
	return nil
}

func runMCP(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "add" {
		return errors.New("usage: thanos mcp add NAME --type stdio|http|sse [--command CMD] [--url URL]")
	}
	flags := flag.NewFlagSet("mcp add", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	transport := flags.String("type", "stdio", "MCP transport")
	command := flags.String("command", "", "stdio server executable")
	commandArgs := flags.String("args", "", "comma-separated stdio server arguments")
	url := flags.String("url", "", "HTTP or SSE endpoint")
	parsed, err := intersperseFlags(args[1:], map[string]bool{
		"--type": true, "--command": true, "--args": true, "--url": true,
	})
	if err != nil {
		return err
	}
	if err := flags.Parse(parsed); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: thanos mcp add NAME --type stdio|http|sse [--command CMD] [--url URL]")
	}
	if *transport != "stdio" && *transport != "http" && *transport != "sse" {
		return fmt.Errorf("unsupported MCP transport %q", *transport)
	}
	if *transport == "stdio" && *command == "" {
		return errors.New("stdio MCP requires --command")
	}
	if (*transport == "http" || *transport == "sse") && *url == "" {
		return fmt.Errorf("%s MCP requires --url", *transport)
	}
	config, err := ws.ReadConfig()
	if err != nil {
		return err
	}
	if config.MCP == nil {
		config.MCP = map[string]model.MCP{}
	}
	name := strings.ToLower(strings.TrimSpace(flags.Arg(0)))
	config.MCP[name] = model.MCP{
		Type: *transport, Command: *command, Args: splitCSV(*commandArgs), URL: *url,
	}
	if err := ws.WriteConfig(config); err != nil {
		return err
	}
	printExecLog(stdout, ui.ExecLogEntry{
		Type: "write", Path: ws.ConfigPath(),
		Message: fmt.Sprintf("Configured MCP %s (%s)", name, *transport), Status: ui.Completed,
	})
	return nil
}

func syncSkillsToRunners(root string, config model.Config, skills []model.Skill) error {
	for _, runnerConfig := range config.Runners {
		if err := syncSkillsToRunner(root, runnerConfig, skills); err != nil {
			return err
		}
	}
	return nil
}

func syncSkillsToRunner(root string, runnerConfig model.Runner, skills []model.Skill) error {
	targetRoot := runnerConfig.SkillsDir
	if targetRoot == "" {
		targetRoot = defaultSkillsDir(runnerConfig.Agent)
	}
	if targetRoot == "" {
		return nil
	}
	targetRootAbs := filepath.Join(root, filepath.FromSlash(targetRoot))
	for _, skill := range skills {
		sourceFile := filepath.Join(root, filepath.FromSlash(skill.Path))
		sourceDir := filepath.Dir(sourceFile)
		if _, err := os.Stat(sourceFile); err != nil {
			return fmt.Errorf("sync skill %s: %w", skill.Name, err)
		}
		target := filepath.Join(targetRootAbs, skill.Name)
		if filepath.Clean(target) == filepath.Clean(sourceDir) {
			continue
		}
		if err := os.MkdirAll(targetRootAbs, 0o755); err != nil {
			return err
		}
		if info, err := os.Lstat(target); err == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				return fmt.Errorf("cannot sync skill %s: %s already exists and is not a symlink", skill.Name, target)
			}
			existing, readErr := os.Readlink(target)
			if readErr != nil {
				return readErr
			}
			resolved := existing
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(filepath.Dir(target), resolved)
			}
			if filepath.Clean(resolved) == filepath.Clean(sourceDir) {
				continue
			}
			if err := os.Remove(target); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		relative, err := filepath.Rel(filepath.Dir(target), sourceDir)
		if err != nil {
			return err
		}
		if err := os.Symlink(relative, target); err != nil {
			return fmt.Errorf("link skill %s into %s: %w", skill.Name, targetRoot, err)
		}
	}
	return nil
}

func discoverSkillFiles(root string) (map[string]string, error) {
	roots := []string{".agents/skills", ".claude/skills", ".cursor/skills", ".gemini/skills"}
	found := map[string]string{}
	for _, relativeRoot := range roots {
		base := filepath.Join(root, filepath.FromSlash(relativeRoot))
		err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if info.IsDir() || info.Name() != "SKILL.md" {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			name := filepath.Base(filepath.Dir(path))
			found[filepath.ToSlash(relative)] = name
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	return found, nil
}

func runPlugin(ctx context.Context, ws *workspace.Workspace, args []string, stdout, stderr io.Writer) error {
	if len(args) < 1 {
		return errors.New("usage: thanos plugin marketplace add claude SOURCE | thanos plugin install claude NAME")
	}
	if args[0] == "marketplace" {
		if len(args) != 4 || args[1] != "add" {
			return errors.New("usage: thanos plugin marketplace add claude SOURCE")
		}
		runnerName, source := args[2], args[3]
		if runnerName != "claude" {
			return fmt.Errorf("plugin marketplaces are not implemented for runner %q", runnerName)
		}
		if err := runExternal(ctx, ws.Root, stdout, stderr, "claude", "plugin", "marketplace", "add", source); err != nil {
			return err
		}
		config, err := ws.ReadConfig()
		if err != nil {
			return err
		}
		config.PluginMarketplaces = appendUniqueMarketplace(config.PluginMarketplaces, model.PluginMarketplace{Runner: runnerName, Source: source})
		return ws.WriteConfig(config)
	}
	if args[0] == "install" {
		flags := flag.NewFlagSet("plugin install", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		scope := flags.String("scope", "project", "plugin scope")
		parsed, err := intersperseFlags(args[1:], map[string]bool{"--scope": true})
		if err != nil {
			return err
		}
		if err := flags.Parse(parsed); err != nil {
			return err
		}
		if flags.NArg() != 2 {
			return errors.New("usage: thanos plugin install claude NAME@MARKETPLACE [--scope project]")
		}
		runnerName, name := flags.Arg(0), flags.Arg(1)
		if runnerName != "claude" {
			return fmt.Errorf("plugin installation is not implemented for runner %q", runnerName)
		}
		if *scope != "user" && *scope != "project" && *scope != "local" {
			return fmt.Errorf("invalid plugin scope %q", *scope)
		}
		if err := runExternal(ctx, ws.Root, stdout, stderr, "claude", "plugin", "install", name, "--scope", *scope); err != nil {
			return err
		}
		config, err := ws.ReadConfig()
		if err != nil {
			return err
		}
		config.Plugins = appendUniquePlugin(config.Plugins, model.Plugin{Runner: runnerName, Name: name, Scope: *scope})
		return ws.WriteConfig(config)
	}
	return errors.New("usage: thanos plugin marketplace add claude SOURCE | thanos plugin install claude NAME")
}

func appendUniqueMarketplace(items []model.PluginMarketplace, item model.PluginMarketplace) []model.PluginMarketplace {
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

func appendUniquePlugin(items []model.Plugin, item model.Plugin) []model.Plugin {
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

func runExternalCommand(ctx context.Context, root string, stdout, stderr io.Writer, name string, args ...string) error {
	commandText := strings.TrimSpace(name + " " + strings.Join(args, " "))
	started := time.Now()
	printExecLog(stdout, ui.ExecLogEntry{
		Type: "exec", Command: commandText, Workdir: root, Status: ui.Running,
	})
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = root
	command.Stdout = stdout
	command.Stderr = stderr
	command.Stdin = os.Stdin
	if err := command.Run(); err != nil {
		printExecLog(stderr, ui.ExecLogEntry{
			Status: ui.Failed, DurationMs: time.Since(started).Milliseconds(),
		})
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	printExecLog(stdout, ui.ExecLogEntry{
		Status: ui.Succeeded, DurationMs: time.Since(started).Milliseconds(),
	})
	return nil
}

func printExecLog(output io.Writer, entry ui.ExecLogEntry) {
	ui.Block(output, ui.ExecLog(entry))
}

func parseSkillRoles(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	valid := map[string]bool{
		string(model.RoleDesigner):       true,
		string(model.RoleDesignReviewer): true,
		string(model.RoleCoder):          true,
		string(model.RoleReviewer):       true,
		string(model.RoleTester):         true,
		string(model.RoleDeepReviewer):   true,
		string(model.RoleAcceptor):       true,
		string(model.RoleMiniCoder):      true,
		string(model.RoleReVerifier):     true,
		string(model.RoleSynthesizer):    true,
		string(model.RoleGate):           true,
	}
	var roles []string
	seen := map[string]bool{}
	for _, role := range strings.Split(value, ",") {
		role = strings.TrimSpace(role)
		if !valid[role] {
			return nil, fmt.Errorf("unknown skill role %q", role)
		}
		if !seen[role] {
			roles = append(roles, role)
			seen[role] = true
		}
	}
	return roles, nil
}

func phaseForRole(role model.Role) model.Phase {
	switch role {
	case model.RoleDesigner:
		return model.PhaseDesign
	case model.RoleDesignReviewer:
		return model.PhaseDesignReview
	case model.RoleCoder:
		return model.PhaseCode
	case model.RoleReviewer:
		return model.PhaseReview
	case model.RoleTester:
		return model.PhaseTest
	case model.RoleDeepReviewer:
		return model.PhaseDeepReview
	case model.RoleAcceptor:
		return model.PhaseAccept
	default:
		return ""
	}
}

func splitList(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ";") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func splitCSV(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func detectedLSPs(language string) map[string]model.LSP {
	candidates := map[string]model.LSP{
		"go":         {Command: "gopls"},
		"typescript": {Command: "typescript-language-server", Args: []string{"--stdio"}},
		"javascript": {Command: "typescript-language-server", Args: []string{"--stdio"}},
		"python":     {Command: "pyright-langserver", Args: []string{"--stdio"}},
		"rust":       {Command: "rust-analyzer"},
	}
	candidate, ok := candidates[strings.ToLower(language)]
	if !ok {
		return nil
	}
	if _, err := exec.LookPath(candidate.Command); err != nil {
		return nil
	}
	return map[string]model.LSP{strings.ToLower(language): candidate}
}

func defaultRunnerArgs(name string) []string {
	switch strings.ToLower(name) {
	case "codex":
		return []string{"exec", "--full-auto", "-"}
	case "claude":
		return []string{"--print", "--dangerously-skip-permissions"}
	default:
		return nil
	}
}

func defaultRunnerAgent(name string) string {
	switch strings.ToLower(name) {
	case "claude", "claude-code":
		return "claude-code"
	case "codex":
		return "codex"
	case "cursor":
		return "cursor"
	case "gemini", "gemini-cli":
		return "gemini-cli"
	default:
		return name
	}
}

func defaultSkillsDir(name string) string {
	switch strings.ToLower(name) {
	case "claude", "claude-code":
		return ".claude/skills"
	case "codex", "cursor", "gemini", "gemini-cli", "universal":
		return ".agents/skills"
	default:
		return ""
	}
}

func intersperseFlags(args []string, valueFlags map[string]bool) ([]string, error) {
	var flags, positional []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		name := arg
		if equals := strings.IndexByte(arg, '='); equals >= 0 {
			name = arg[:equals]
		}
		if !valueFlags[name] {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
		if strings.Contains(arg, "=") {
			continue
		}
		if index+1 >= len(args) {
			return nil, fmt.Errorf("%s requires a value", arg)
		}
		index++
		flags = append(flags, args[index])
	}
	return append(flags, positional...), nil
}

func printHelp(output io.Writer) {
	ui.Block(output, `Thanos — multi-role AI development framework

Usage:
  thanos                              Open the session TUI in an initialized workspace
  thanos ui                           Open the session TUI
  thanos init [--name NAME] [--language LANGUAGE] [--framework FRAMEWORK] [--runner codex] [--runner-command codex]
  thanos new "Feature title" [--description TEXT] [--acceptance "A; B"]
  thanos bugfix FEATURE_ID "Bugfix title" [--description TEXT] [--rules "A; B"]
  thanos run FEATURE_ID [--runner NAME]
  thanos continue FEATURE_ID [--runner NAME]
  thanos status
  thanos prompt FEATURE_ID designer|coder|reviewer|tester
  thanos transition FEATURE_ID PHASE
  thanos done FEATURE_ID
  thanos doctor
  thanos skill find [QUERY]
  thanos skill add OWNER/REPO [--skill NAME] [--agent universal] [--roles role,role]
  thanos plugin marketplace add claude OWNER/REPO
  thanos plugin install claude NAME@MARKETPLACE [--scope project]
  thanos runner add NAME [--command CMD] [--agent AGENT] [--skills-dir PATH]
  thanos lsp add NAME --command CMD [--args "arg,arg"]
  thanos mcp add NAME --type stdio|http|sse [--command CMD] [--url URL]
  thanos memory [FEATURE_ID]
  thanos scan
  thanos version

Initialization is network-free. Skills are installed explicitly through the
open npx skills CLI; Claude plugins use Claude Code's native plugin CLI.`)
}
