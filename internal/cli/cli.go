package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	"github.com/tinhtran/thanos/internal/taskworkflow"
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
	case "board":
		return runBoard(ws, stdout)
	case "task":
		return runTask(ctx, ws, args[1:], stdout, stderr)
	case "run":
		return runFeature(ctx, ws, args[1:], stdout, stderr)
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
	case "ask":
		return runAsk(ctx, ws, args[1:], stdout, stderr)
	case "plan":
		return runPlan(ws, args[1:], stdout)
	case "clarify":
		return runClarify(ctx, ws, args[1:], stdout, stderr)
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
		phase := "-"
		if current, err := ws.ReadState(feature.ID); err == nil {
			phase = string(current.Phase)
		}
		rows = append(rows, []string{
			feature.ID,
			feature.Status,
			phase,
			feature.Title,
		})
	}
	ui.Block(stdout, ui.Table([]string{"ID", "STATUS", "PHASE", "TITLE"}, rows))
	return nil
}

func runBoard(ws *workspace.Workspace, stdout io.Writer) error {
	tasks, err := ws.ListTasks()
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		ui.Info(stdout, "No tasks.")
		return nil
	}
	order := []model.TaskStatus{
		model.TaskBacklog, model.TaskPlan, model.TaskExecute, model.TaskVerify, model.TaskDone,
	}
	byStatus := map[model.TaskStatus][]model.Task{}
	for _, task := range tasks {
		byStatus[task.Status] = append(byStatus[task.Status], task)
	}
	for _, status := range order {
		rows := [][]string{}
		for _, task := range byStatus[status] {
			parent := task.ParentTaskID
			if parent == "" {
				parent = "-"
			}
			rows = append(rows, []string{task.ID, task.Priority, task.AssignedAgent, parent, task.Title})
		}
		ui.Block(stdout, ui.Section(string(status), nil))
		if len(rows) == 0 {
			ui.Info(stdout, "Empty")
			continue
		}
		ui.Block(stdout, ui.Table([]string{"ID", "PRIORITY", "AGENT", "PARENT", "TITLE"}, rows))
	}
	return nil
}

func runTask(ctx context.Context, ws *workspace.Workspace, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: thanos task create|split|plan|execute|verify|done|reopen")
	}
	switch args[0] {
	case "list":
		return runTaskList(ws, args[1:], stdout)
	case "show":
		return runTaskShow(ws, args[1:], stdout)
	case "create":
		return runTaskCreate(ws, args[1:], stdout)
	case "split":
		return runTaskSplit(ws, args[1:], stdout)
	case "plan":
		return runTaskPlan(ws, args[1:], stdout)
	case "execute", "run":
		return runTaskExecute(ctx, ws, args[1:], stdout, stderr)
	case "verify", "review", "test":
		return runTaskVerify(ctx, ws, args[1:], stdout, stderr)
	case "done":
		return runTaskDone(ws, args[1:], stdout)
	case "reopen":
		return runTaskReopen(ws, args[1:], stdout)
	default:
		return errors.New("usage: thanos task create|split|plan|execute|verify|done|reopen")
	}
}

func runTaskList(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "--json") {
		return errors.New("usage: thanos task list [--json]")
	}
	tasks, err := ws.ListTasks()
	if err != nil {
		return err
	}
	if len(args) == 1 {
		return json.NewEncoder(stdout).Encode(tasks)
	}
	rows := make([][]string, 0, len(tasks))
	for _, task := range tasks {
		rows = append(rows, []string{task.ID, string(task.Status), task.Priority, task.AssignedAgent, task.Title})
	}
	ui.Block(stdout, ui.Table([]string{"ID", "STATUS", "PRIORITY", "AGENT", "TITLE"}, rows))
	return nil
}

func runTaskShow(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	if len(args) < 1 || len(args) > 2 || (len(args) == 2 && args[1] != "--json") {
		return errors.New("usage: thanos task show TASK_ID [--json]")
	}
	task, err := ws.LoadTask(args[0])
	if err != nil {
		return err
	}
	if len(args) == 2 {
		return json.NewEncoder(stdout).Encode(task)
	}
	ui.Block(stdout, ui.Section("Task", []ui.Field{
		{Label: "id", Value: task.ID},
		{Label: "status", Value: string(task.Status)},
		{Label: "priority", Value: task.Priority},
		{Label: "agent", Value: emptyDash(task.AssignedAgent)},
		{Label: "plan", Value: task.PlanPath},
		{Label: "log", Value: task.LogPath},
		{Label: "review", Value: task.ReviewPath},
		{Label: "tests", Value: task.TestResultPath},
		{Label: "title", Value: task.Title},
	}))
	return nil
}

func runTaskCreate(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("task create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	description := flags.String("description", "", "task description")
	priority := flags.String("priority", "medium", "priority")
	agent := flags.String("agent", "", "assigned agent profile")
	parent := flags.String("parent", "", "parent task id")
	parsed, err := intersperseFlags(args, map[string]bool{
		"--description": true, "--priority": true, "--agent": true, "--parent": true,
	})
	if err != nil {
		return err
	}
	if err := flags.Parse(parsed); err != nil {
		return err
	}
	title := strings.TrimSpace(strings.Join(flags.Args(), " "))
	if title == "" {
		return errors.New("usage: thanos task create \"Task title\" [--description TEXT] [--priority P] [--agent NAME]")
	}
	task, err := ws.NewTask(title, *description, *priority, *agent, *parent)
	if err != nil {
		return err
	}
	if err := ws.SaveTask(task); err != nil {
		return err
	}
	if task.ParentTaskID != "" {
		parentTask, err := ws.LoadTask(task.ParentTaskID)
		if err != nil {
			return err
		}
		parentTask.Subtasks = appendUnique(parentTask.Subtasks, task.ID)
		parentTask.UpdatedAt = time.Now().UTC()
		if err := ws.SaveTask(parentTask); err != nil {
			return err
		}
	}
	printExecLog(stdout, ui.ExecLogEntry{Type: "write", Path: ws.TaskPath(task.ID), Message: "Created task " + task.ID, Status: ui.Completed})
	return nil
}

func runTaskSplit(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: thanos task split TASK_ID")
	}
	parent, err := ws.LoadTask(args[0])
	if err != nil {
		return err
	}
	templates := []struct {
		suffix      string
		description string
		agentRole   string
	}{
		{"Plan", "Understand the task once and produce reusable implementation artifacts.", "planning"},
		{"Execute", "Implement the saved plan in an isolated worktree and record changed files.", "implementation"},
		{"Verify", "Review the execution summary and run validation checks.", "verification"},
	}
	for _, tmpl := range templates {
		child, err := ws.NewTask(parent.Title+" - "+tmpl.suffix, tmpl.description+"\n\nParent: "+parent.ID+"\n"+parent.Description, parent.Priority, agentForRole(ws, tmpl.agentRole), parent.ID)
		if err != nil {
			return err
		}
		if err := ws.SaveTask(child); err != nil {
			return err
		}
		parent.Subtasks = appendUnique(parent.Subtasks, child.ID)
		printExecLog(stdout, ui.ExecLogEntry{Type: "write", Path: ws.TaskPath(child.ID), Message: "Created subtask " + child.ID, Status: ui.Completed})
	}
	parent.UpdatedAt = time.Now().UTC()
	return ws.SaveTask(parent)
}

func runTaskPlan(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: thanos task plan TASK_ID")
	}
	task, err := ws.LoadTask(args[0])
	if err != nil {
		return err
	}
	if task.Status == model.TaskBacklog {
		if task, err = taskworkflow.Transition(task, model.TaskPlan); err != nil {
			return err
		}
	}
	if _, err := os.Stat(ws.TaskPlanPath(task.ID)); err == nil {
		task.PlanPath = filepath.ToSlash(filepath.Join(".thanos", "plans", task.ID+".md"))
		task.UpdatedAt = time.Now().UTC()
		if saveErr := ws.SaveTask(task); saveErr != nil {
			return saveErr
		}
		printTaskStatus(stdout, task, "Plan already exists; reused saved plan")
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	memory := relatedFeatureMemory(ws, task)
	plan := renderTaskPlan(task, memory)
	if err := os.MkdirAll(filepath.Dir(ws.TaskPlanPath(task.ID)), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(ws.TaskPlanPath(task.ID), []byte(plan), 0o644); err != nil {
		return err
	}
	task.PlanPath = filepath.ToSlash(filepath.Join(".thanos", "plans", task.ID+".md"))
	task.UpdatedAt = time.Now().UTC()
	if err := ws.SaveTask(task); err != nil {
		return err
	}
	printTaskStatus(stdout, task, "Plan generated")
	printExecLog(stdout, ui.ExecLogEntry{Type: "write", Path: ws.TaskPlanPath(task.ID), Message: "Created approved-plan candidate", Status: ui.Completed})
	return nil
}

func runTaskExecute(ctx context.Context, ws *workspace.Workspace, args []string, stdout, stderr io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: thanos task execute TASK_ID")
	}
	task, err := ws.LoadTask(args[0])
	if err != nil {
		return err
	}
	if _, err := os.Stat(ws.TaskPlanPath(task.ID)); err != nil {
		return fmt.Errorf("plan is required before Execute: run 'thanos task plan %s'", task.ID)
	}
	if task.Status != model.TaskExecute {
		task, err = taskworkflow.Transition(task, model.TaskExecute)
		if err != nil {
			return err
		}
	}
	if err := ensureTaskWorktree(ctx, ws, task, stdout, stderr); err != nil {
		return err
	}
	if err := runTaskAgent(ctx, ws, task, stdout, stderr); err != nil {
		task.UpdatedAt = time.Now().UTC()
		_ = ws.SaveTask(task)
		return err
	}
	if err := writeExecutionSummary(ctx, ws, task); err != nil {
		return err
	}
	task, err = taskworkflow.Transition(task, model.TaskVerify)
	if err != nil {
		return err
	}
	if err := ws.SaveTask(task); err != nil {
		return err
	}
	printTaskStatus(stdout, task, "Paused at Verify")
	return printTaskVerifyGate(ws, task, stdout)
}

func runTaskVerify(ctx context.Context, ws *workspace.Workspace, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: thanos task verify TASK_ID [approve|request-changes|rerun-agent|reopen-plan]")
	}
	task, err := ws.LoadTask(args[0])
	if err != nil {
		return err
	}
	if len(args) > 1 {
		switch args[1] {
		case "approve":
			task.ReviewApproved = true
			task.UpdatedAt = time.Now().UTC()
			if err := ws.SaveTask(task); err != nil {
				return err
			}
		case "request-changes", "rerun-agent":
			task.ReviewApproved = false
			task, err = taskworkflow.Transition(task, model.TaskExecute)
			if err != nil {
				return err
			}
			if err := ws.SaveTask(task); err != nil {
				return err
			}
		case "reopen-plan":
			task.ReviewApproved = false
			task, err = taskworkflow.Transition(task, model.TaskPlan)
			if err != nil {
				return err
			}
			if err := ws.SaveTask(task); err != nil {
				return err
			}
		default:
			return errors.New("usage: thanos task verify TASK_ID [approve|request-changes|rerun-agent|reopen-plan]")
		}
		return printTaskVerifyGate(ws, task, stdout)
	}
	if task.Status == model.TaskExecute {
		task, err = taskworkflow.Transition(task, model.TaskVerify)
		if err != nil {
			return err
		}
		if err := ws.SaveTask(task); err != nil {
			return err
		}
	}
	if err := writeReviewReport(ws, task); err != nil {
		return err
	}
	if err := runTaskTests(ctx, ws, task, stdout, stderr); err != nil {
		task.TestsPassed = false
		task.UpdatedAt = time.Now().UTC()
		_ = ws.SaveTask(task)
		return err
	}
	task.TestsPassed = true
	task.UpdatedAt = time.Now().UTC()
	if err := ws.SaveTask(task); err != nil {
		return err
	}
	return printTaskVerifyGate(ws, task, stdout)
}

func printTaskVerifyGate(ws *workspace.Workspace, task model.Task, stdout io.Writer) error {
	printTaskStatus(stdout, task, "Review gate")
	ui.Block(stdout, ui.Section("Plan Checklist", []ui.Field{{Label: "plan", Value: task.PlanPath}, {Label: "approved", Value: fmt.Sprint(task.ReviewApproved)}, {Label: "tests", Value: fmt.Sprint(task.TestsPassed)}}))
	ui.Block(stdout, ui.Section("Changed Files", []ui.Field{{Label: "worktree", Value: task.WorktreePath}}))
	if summary, err := os.ReadFile(ws.TaskLogPath(task.ID)); err == nil {
		ui.Raw(stdout, string(summary))
	} else {
		ui.Info(stdout, "No execution summary yet.")
	}
	if review, err := os.ReadFile(ws.TaskReviewPath(task.ID)); err == nil {
		ui.Raw(stdout, string(review))
	} else {
		ui.Info(stdout, "No review result yet.")
	}
	if tests, err := os.ReadFile(ws.TaskTestResultPath(task.ID)); err == nil {
		ui.Raw(stdout, string(tests))
	} else {
		ui.Info(stdout, "No test result yet.")
	}
	ui.Block(stdout, ui.Table([]string{"ACTION", "COMMAND"}, [][]string{
		{"approve", "thanos task verify " + task.ID + " approve"},
		{"request changes", "thanos task verify " + task.ID + " request-changes"},
		{"rerun agent", "thanos task verify " + task.ID + " rerun-agent"},
		{"reopen plan", "thanos task verify " + task.ID + " reopen-plan"},
	}))
	return nil
}

func runTaskTests(ctx context.Context, ws *workspace.Workspace, task model.Task, stdout, stderr io.Writer) error {
	config, err := ws.ReadConfig()
	if err != nil {
		return err
	}
	commands := config.Project.Test
	if len(commands) == 0 {
		commands = []string{"go test ./..."}
	}
	root := ws.Root
	if task.WorktreePath != "" {
		root = filepath.Join(ws.Root, filepath.FromSlash(task.WorktreePath))
	}
	report := &strings.Builder{}
	report.WriteString("# Test Result\n\n")
	passed := true
	for _, commandText := range commands {
		report.WriteString("## `" + commandText + "`\n\n")
		if err := runShellCommand(ctx, root, commandText, stdout, stderr, report); err != nil {
			passed = false
			report.WriteString("\nVERDICT: FAIL\n")
			report.WriteString(err.Error() + "\n")
			break
		}
		report.WriteString("\nVERDICT: PASS\n")
	}
	if err := os.MkdirAll(filepath.Dir(ws.TaskTestResultPath(task.ID)), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(ws.TaskTestResultPath(task.ID), []byte(report.String()), 0o644); err != nil {
		return err
	}
	if passed {
		return nil
	}
	printTaskStatus(stderr, task, "Tests failed")
	return fmt.Errorf("task %s tests failed", task.ID)
}

func runTaskDone(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: thanos task done TASK_ID")
	}
	task, err := ws.LoadTask(args[0])
	if err != nil {
		return err
	}
	if task.Status != model.TaskVerify {
		return fmt.Errorf("task %s must be in Verify before Done", task.ID)
	}
	task, err = taskworkflow.Transition(task, model.TaskDone)
	if err != nil {
		return err
	}
	if err := ws.SaveTask(task); err != nil {
		return err
	}
	printTaskStatus(stdout, task, "Task completed")
	return nil
}

func runTaskReopen(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: thanos task reopen TASK_ID")
	}
	task, err := ws.LoadTask(args[0])
	if err != nil {
		return err
	}
	task.Status = model.TaskBacklog
	task.ReviewApproved = false
	task.TestsPassed = false
	task.UpdatedAt = time.Now().UTC()
	if err := ws.SaveTask(task); err != nil {
		return err
	}
	printTaskStatus(stdout, task, "Task reopened")
	return nil
}

func printTaskStatus(stdout io.Writer, task model.Task, message string) {
	ui.Block(stdout, ui.Section("Task", []ui.Field{
		{Label: "id", Value: task.ID},
		{Label: "status", Value: string(task.Status)},
		{Label: "agent", Value: emptyDash(task.AssignedAgent)},
		{Label: "branch", Value: emptyDash(task.BranchName)},
		{Label: "worktree", Value: emptyDash(task.WorktreePath)},
		{Label: "message", Value: message},
	}))
}

func renderTaskPlan(task model.Task, memory string) string {
	if strings.TrimSpace(memory) == "" {
		memory = "No related feature memory found."
	}
	return strings.TrimSpace(fmt.Sprintf(`# Plan: %s

## Requirement Summary
%s

## Acceptance Criteria
- Confirm the implementation satisfies the task request.
- Keep review and test evidence under .thanos/.

## Related Feature Memory
%s

## Affected Modules
- TBD by planner before execution.

## Task Checklist
- Confirm assumptions and acceptance criteria.
- Make the smallest coherent implementation change in the task worktree.
- Update or add focused tests.
- Produce an execution summary for Verify.

## Risks
- Scope may be broader than the initial task description.
- Existing feature memory may be incomplete.

## Test Strategy
- Run the configured project smoke or unit test command.
- Record skipped tests with a reason.

## Rollback Plan
- Keep all edits isolated in the task worktree and branch.
- Remove the worktree or discard the task branch if review fails.

## Optional Mermaid Diagram
`+"```mermaid"+`
flowchart LR
  Backlog --> Plan --> Execute --> Verify --> Done
  Verify --> Execute
  Verify --> Plan
`+"```"+`
`, task.Title, task.Description, memory)) + "\n"
}

func writeTaskLog(ws *workspace.Workspace, task model.Task, content string) {
	path := ws.TaskLogPath(task.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	fmt.Fprintln(file, content)
}

func relatedFeatureMemory(ws *workspace.Workspace, task model.Task) string {
	features, err := filepath.Glob(filepath.Join(ws.DotDir(), "plan-graph", "features", "*.md"))
	if err != nil {
		return ""
	}
	taskText := strings.ToLower(task.Title + "\n" + task.Description)
	var matches []string
	for _, path := range features {
		name := strings.TrimSuffix(filepath.Base(path), ".md")
		if !strings.Contains(taskText, strings.ToLower(strings.ReplaceAll(name, "-", " "))) && !strings.Contains(taskText, strings.ToLower(name)) {
			continue
		}
		data, err := os.ReadFile(path)
		if err == nil {
			matches = append(matches, "### "+name+"\n"+string(data))
		}
	}
	return strings.Join(matches, "\n\n")
}

func updateTaskFeatureMemory(ws *workspace.Workspace, task model.Task) error {
	name := task.Title
	if task.ParentTaskID != "" {
		name = task.ParentTaskID
	}
	path := ws.PlanGraphFeaturePath(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	executionSummary := readOptional(ws.TaskLogPath(task.ID))
	testResult := readOptional(ws.TaskTestResultPath(task.ID))
	content := fmt.Sprintf(`# %s

## Last Updated
%s

## Changed Behavior
Task %s: %s

## Important Files
%s

## Decisions
- Review approved: %t
- Tests passed: %t

## Test Notes
%s

## Future Risks
- Revisit this memory if follow-up review finds regressions.
`, name, time.Now().UTC().Format(time.RFC3339), task.ID, task.Title, strings.TrimSpace(executionSummary), task.ReviewApproved, task.TestsPassed, strings.TrimSpace(testResult))
	return os.WriteFile(path, []byte(content), 0o644)
}

func ensureTaskWorktree(ctx context.Context, ws *workspace.Workspace, task model.Task, stdout, stderr io.Writer) error {
	target := filepath.Join(ws.Root, filepath.FromSlash(task.WorktreePath))
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		return nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := runExternal(ctx, ws.Root, stdout, stderr, "git", "worktree", "add", "-b", task.BranchName, target); err != nil {
		return fmt.Errorf("create task worktree: %w", err)
	}
	return nil
}

func runTaskAgent(ctx context.Context, ws *workspace.Workspace, task model.Task, stdout, stderr io.Writer) error {
	profile, ok, err := taskAgent(ws, task, model.TaskExecute)
	if err != nil {
		return err
	}
	if !ok || profile.Command == "" {
		writeTaskLog(ws, task, executePrompt(ws, task))
		ui.Info(stdout, "No executable agent profile configured; wrote Execute prompt to "+task.LogPath)
		return nil
	}
	worktree := filepath.Join(ws.Root, filepath.FromSlash(task.WorktreePath))
	prompt := executePrompt(ws, task)
	commandText := strings.TrimSpace(profile.Command + " " + strings.Join(profile.Args, " "))
	printExecLog(stdout, ui.ExecLogEntry{Type: "exec", Command: commandText, Workdir: worktree, Status: ui.Running})
	started := time.Now()
	cmd := exec.CommandContext(ctx, profile.Command, profile.Args...)
	cmd.Dir = worktree
	cmd.Stdin = strings.NewReader(prompt)
	logPath := ws.TaskLogPath(task.ID)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	cmd.Stdout = io.MultiWriter(stdout, logFile)
	cmd.Stderr = io.MultiWriter(stderr, logFile)
	cmd.Env = append(os.Environ(), "THANOS_TASK_ID="+task.ID, "THANOS_PROJECT_ROOT="+ws.Root)
	for key, value := range profile.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	if err := cmd.Run(); err != nil {
		printExecLog(stderr, ui.ExecLogEntry{Status: ui.Failed, DurationMs: time.Since(started).Milliseconds()})
		return err
	}
	printExecLog(stdout, ui.ExecLogEntry{Status: ui.Succeeded, DurationMs: time.Since(started).Milliseconds()})
	return nil
}

// roleForStep maps a task workflow step to the agent-profile role that should
// run it, so the per-role Agent Profiles matrix (planner/coder/reviewer/tester)
// drives which agent executes each step.
func roleForStep(step model.TaskStatus) string {
	switch step {
	case model.TaskPlan:
		return "planner"
	case model.TaskExecute:
		return "coder"
	case model.TaskVerify:
		return "reviewer"
	default:
		return ""
	}
}

func taskAgent(ws *workspace.Workspace, task model.Task, step model.TaskStatus) (model.AgentProfile, bool, error) {
	agents, err := ws.ReadAgents()
	if err != nil {
		return model.AgentProfile{}, false, err
	}
	// An explicit per-task assignment always wins.
	if name := task.AssignedAgent; name != "" {
		for _, profile := range agents.Agents {
			if profile.Name == name && agentAllows(profile, step) {
				return profile, true, nil
			}
		}
		return model.AgentProfile{}, false, fmt.Errorf("agent profile %q does not allow %s", name, step)
	}
	// Otherwise prefer the profile assigned to this step's role.
	if role := roleForStep(step); role != "" {
		for _, profile := range agents.Agents {
			if strings.EqualFold(profile.Role, role) && profile.Command != "" && agentAllows(profile, step) {
				return profile, true, nil
			}
		}
	}
	// Fall back to the first profile that allows the step.
	for _, profile := range agents.Agents {
		if agentAllows(profile, step) {
			return profile, true, nil
		}
	}
	return model.AgentProfile{}, false, nil
}

func agentAllows(profile model.AgentProfile, step model.TaskStatus) bool {
	if len(profile.AllowedSteps) == 0 {
		return true
	}
	for _, allowed := range profile.AllowedSteps {
		if allowed == step {
			return true
		}
	}
	return false
}

func agentForRole(ws *workspace.Workspace, role string) string {
	agents, err := ws.ReadAgents()
	if err != nil {
		return ""
	}
	for _, profile := range agents.Agents {
		if strings.EqualFold(profile.Role, role) {
			return profile.Name
		}
	}
	return ""
}

func executePrompt(ws *workspace.Workspace, task model.Task) string {
	plan := readOptional(ws.TaskPlanPath(task.ID))
	return strings.TrimSpace(fmt.Sprintf(`# Execute task

Read and follow only the saved plan before editing code. Do not re-analyze the original ticket. Keep all work in this worktree. Do not merge.

Task: %s
Status: %s

Saved plan:
%s

After execution, leave the repository ready for Verify. Thanos will stop at the human approval gate.
`, task.ID, task.Status, plan)) + "\n"
}

func writeExecutionSummary(ctx context.Context, ws *workspace.Workspace, task model.Task) error {
	root := filepath.Join(ws.Root, filepath.FromSlash(task.WorktreePath))
	out, err := exec.CommandContext(ctx, "git", "-C", root, "diff", "--stat").CombinedOutput()
	if err != nil {
		return fmt.Errorf("git diff --stat: %w: %s", err, strings.TrimSpace(string(out)))
	}
	files, err := exec.CommandContext(ctx, "git", "-C", root, "diff", "--name-only").CombinedOutput()
	if err != nil {
		return fmt.Errorf("git diff --name-only: %w: %s", err, strings.TrimSpace(string(files)))
	}
	content := fmt.Sprintf("# Execution Summary\n\n## Changed Files\n%s\n\n## Completed Checklist\n- Review the saved plan checklist manually.\n\n## Diff Stat\n%s\n\n## Execution Notes\n- Verify reads this summary and the saved plan; the original ticket is not needed.\n", string(files), string(out))
	if err := os.MkdirAll(filepath.Dir(ws.TaskLogPath(task.ID)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(ws.TaskLogPath(task.ID), []byte(content), 0o644)
}

func writeReviewReport(ws *workspace.Workspace, task model.Task) error {
	plan := readOptional(ws.TaskPlanPath(task.ID))
	execution := readOptional(ws.TaskLogPath(task.ID))
	content := fmt.Sprintf("# Review\n\n## Inputs\n- Plan: %s\n- Execution summary: %s\n\n## Self Review\n- Verify implementation against the saved plan.\n- Confirm tests, lint, and type checks are recorded when available.\n\n## Remaining Risks\n- Human approval is still required before Done.\n\n## Plan Excerpt\n%s\n\n## Execution Summary\n%s\n", task.PlanPath, task.LogPath, plan, execution)
	if err := os.MkdirAll(filepath.Dir(ws.TaskReviewPath(task.ID)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(ws.TaskReviewPath(task.ID), []byte(content), 0o644)
}

func runShellCommand(ctx context.Context, root, commandText string, stdout, stderr io.Writer, report io.Writer) error {
	printExecLog(stdout, ui.ExecLogEntry{Type: "exec", Command: commandText, Workdir: root, Status: ui.Running})
	started := time.Now()
	cmd := shellCommand(ctx, commandText)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	fmt.Fprintln(report, "```")
	fmt.Fprint(report, string(output))
	fmt.Fprintln(report, "```")
	if len(output) > 0 {
		fmt.Fprint(stdout, string(output))
	}
	if err != nil {
		fmt.Fprint(stderr, string(output))
		printExecLog(stderr, ui.ExecLogEntry{Status: ui.Failed, DurationMs: time.Since(started).Milliseconds()})
		return err
	}
	printExecLog(stdout, ui.ExecLogEntry{Status: ui.Succeeded, DurationMs: time.Since(started).Milliseconds()})
	return nil
}

func readOptional(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func appendUnique(items []string, item string) []string {
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
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
	config, err := ws.ReadConfig()
	if err != nil {
		return err
	}
	current, err := ws.EnsureRuntime(feature, config)
	if err != nil {
		return err
	}
	if err := ws.ValidateCompletionEvidence(feature, current); err != nil {
		return err
	}
	next, err := state.Complete(current)
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

// runAsk sends a single ad-hoc prompt straight to the configured runner and
// streams its output — a headless one-off, like `crush run`, with no pipeline.
func runAsk(ctx context.Context, ws *workspace.Workspace, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("ask", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	runnerName := flags.String("runner", "", "runner override")
	args, err := intersperseFlags(args, map[string]bool{"--runner": true})
	if err != nil {
		return err
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	prompt := strings.TrimSpace(strings.Join(flags.Args(), " "))
	if prompt == "" {
		return errors.New("usage: thanos ask \"<prompt>\" [--runner name]")
	}
	config, err := ws.ReadConfig()
	if err != nil {
		return err
	}
	name := *runnerName
	if name == "" {
		name = config.DefaultRunner
	}
	runnerConfig, ok := config.Runners[name]
	if !ok {
		return fmt.Errorf("runner %q is not configured", name)
	}
	return runner.Subprocess{}.Run(ctx, ws.Root, runnerConfig, prompt, stdout, stderr)
}

// runPlan lists, adds, or removes execution chunks (ECs) of a feature's plan.
func runPlan(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: thanos plan ls|add|rm FEATURE_ID [args]")
	}
	sub := args[0]
	feature, err := ws.LoadFeature(args[1])
	if err != nil {
		return err
	}
	plan, err := ws.ReadPlan(feature.ID)
	if err != nil {
		return err
	}
	switch sub {
	case "ls":
		active := plan.ActiveChunks()
		if len(active) == 0 {
			ui.Info(stdout, "No execution plan yet. Run the feature to let the planner create one.")
			return nil
		}
		rows := make([][]string, 0, len(active))
		for _, c := range active {
			status := c.Status
			if status == "" {
				status = "todo"
			}
			rows = append(rows, []string{strconv.Itoa(c.Index), c.ID, status, c.Title})
		}
		ui.Block(stdout, ui.Table([]string{"#", "ID", "STATUS", "TITLE"}, rows))
		return nil
	case "rm":
		if len(args) != 3 {
			return errors.New("usage: thanos plan rm FEATURE_ID INDEX")
		}
		index, convErr := strconv.Atoi(args[2])
		if convErr != nil {
			return fmt.Errorf("invalid EC index %q", args[2])
		}
		found := false
		for i := range plan.Chunks {
			if plan.Chunks[i].Index == index {
				plan.Chunks[i].Status = "removed"
				found = true
			}
		}
		if !found {
			return fmt.Errorf("no EC with index %d", index)
		}
		if err := ws.WritePlan(feature.ID, plan); err != nil {
			return err
		}
		printExecLog(stdout, ui.ExecLogEntry{
			Type: "write", Path: ws.PlanPath(feature.ID),
			Message: fmt.Sprintf("Removed EC-%d from %s", index, feature.ID), Status: ui.Completed,
		})
		return nil
	case "add":
		title := strings.TrimSpace(strings.Join(args[2:], " "))
		if title == "" {
			return errors.New("usage: thanos plan add FEATURE_ID \"<title>\"")
		}
		next := 0
		for _, c := range plan.Chunks {
			if c.Index > next {
				next = c.Index
			}
		}
		next++
		plan.Chunks = append(plan.Chunks, model.ExecutionChunk{
			Index: next, ID: fmt.Sprintf("%s-ec%d", feature.ID, next), Title: title, Status: "todo",
		})
		if err := ws.WritePlan(feature.ID, plan); err != nil {
			return err
		}
		printExecLog(stdout, ui.ExecLogEntry{
			Type: "write", Path: ws.PlanPath(feature.ID),
			Message: fmt.Sprintf("Added EC-%d to %s", next, feature.ID), Status: ui.Completed,
		})
		return nil
	default:
		return errors.New("usage: thanos plan ls|add|rm FEATURE_ID [args]")
	}
}

// runClarify answers a paused clarification and resumes the run. The engine
// pauses (Active=false, Reason "needs clarification") when a role writes a
// clarify.json; this writes the answer the role reads on resume.
func runClarify(ctx context.Context, ws *workspace.Workspace, args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: thanos clarify FEATURE_ID <answer>")
	}
	feature, err := ws.LoadFeature(args[0])
	if err != nil {
		return err
	}
	answer := strings.TrimSpace(strings.Join(args[1:], " "))
	current, err := ws.ReadState(feature.ID)
	if err != nil {
		return err
	}
	name := clarifyAnswerName(current)
	if err := ws.WriteArtifact(feature.ID, name, answer); err != nil {
		return err
	}
	current.Active = true
	current.Reason = ""
	if err := ws.WriteState(current); err != nil {
		return err
	}
	printExecLog(stdout, ui.ExecLogEntry{
		Type: "write", Path: filepath.Join(ws.RuntimeDir(feature.ID), name),
		Message: "Recorded clarification; resuming", Status: ui.Completed,
	})
	orch := orchestrator.Orchestrator{Workspace: ws, Runner: runner.Subprocess{}, Stdout: stdout, Stderr: stderr}
	return orch.Run(ctx, feature.ID, "")
}

// clarifyAnswerName is the EC-scoped path of the clarification answer artifact.
func clarifyAnswerName(s model.State) string {
	if s.ECTotal > 1 && s.ECIndex >= 1 {
		return fmt.Sprintf("ec-%d/clarify-answer.md", s.ECIndex)
	}
	return "clarify-answer.md"
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
			if info.Mode()&os.ModeSymlink != 0 {
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
			} else if runtime.GOOS != "windows" {
				// On non-Windows a real (non-symlink) entry is unexpected and
				// must not be clobbered. On Windows this is a prior copy we refresh.
				return fmt.Errorf("cannot sync skill %s: %s already exists and is not a symlink", skill.Name, target)
			}
			if err := os.RemoveAll(target); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		relative, err := filepath.Rel(filepath.Dir(target), sourceDir)
		if err != nil {
			return err
		}
		if err := linkOrCopyDir(sourceDir, target, relative); err != nil {
			return fmt.Errorf("link skill %s into %s: %w", skill.Name, targetRoot, err)
		}
	}
	return nil
}

// shellCommand runs commandText through the platform shell (sh on Unix, cmd on
// Windows) so project-configured test/build commands work cross-platform.
func shellCommand(ctx context.Context, commandText string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", commandText)
	}
	return exec.CommandContext(ctx, "sh", "-c", commandText)
}

// linkOrCopyDir symlinks target -> sourceDir (via the relative linkName), and
// falls back to a recursive copy on Windows where symlinks require privilege.
func linkOrCopyDir(sourceDir, target, linkName string) error {
	if err := os.Symlink(linkName, target); err == nil {
		return nil
	} else if runtime.GOOS != "windows" {
		return err
	}
	return copyDir(sourceDir, target)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, info.Mode().Perm())
	})
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
		string(model.RolePlanner):        true,
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
	case model.RolePlanner:
		return model.PhasePlan
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
		return model.PhaseOverview
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
  thanos status
  thanos board
  thanos task create "Task title" [--description TEXT] [--priority P] [--agent NAME]
  thanos task list [--json]
  thanos task show TASK_ID [--json]
  thanos task split TASK_ID
  thanos task plan TASK_ID
  thanos task execute TASK_ID
  thanos task verify TASK_ID [approve|request-changes|rerun-agent|reopen-plan]
  thanos task done TASK_ID
  thanos task reopen TASK_ID
  thanos prompt FEATURE_ID planner|coder|reviewer|tester
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
  thanos ask "<prompt>" [--runner NAME]
  thanos plan ls|add|rm FEATURE_ID [args]
  thanos clarify FEATURE_ID "<answer>"
  thanos scan
  thanos version

Initialization is network-free. Skills are installed explicitly through the
open npx skills CLI; Claude plugins use Claude Code's native plugin CLI.`)
}
