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
	"sort"
	"strings"

	"github.com/tinhtran/thanos/internal/codegraph"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/orchestrator"
	"github.com/tinhtran/thanos/internal/project"
	"github.com/tinhtran/thanos/internal/prompts"
	"github.com/tinhtran/thanos/internal/runner"
	"github.com/tinhtran/thanos/internal/state"
	"github.com/tinhtran/thanos/internal/workspace"
)

var runExternal = runExternalCommand

func Execute(ctx context.Context, args []string, version string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printHelp(stdout)
		return nil
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	ws := workspace.Open(root)
	switch args[0] {
	case "help", "-h", "--help":
		printHelp(stdout)
		return nil
	case "version", "--version":
		fmt.Fprintln(stdout, version)
		return nil
	case "init":
		return runInit(ctx, ws, args[1:], stdout)
	case "new":
		return runNew(ws, args[1:], stdout)
	case "status":
		return runStatus(ws, stdout)
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
	default:
		return fmt.Errorf("unknown command %q; run 'thanos help'", args[0])
	}
}

func runInit(_ context.Context, ws *workspace.Workspace, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	name := flags.String("name", "", "project name override")
	language := flags.String("language", "", "project language override")
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
	}
	if err := ws.Init(config); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Initialized Thanos workspace at %s\n", ws.DotDir())
	if graph.present {
		if err := codegraph.Save(graph.value, ws.DotDir()); err != nil {
			return fmt.Errorf("save initial codebase graph: %w", err)
		}
		fmt.Fprintf(stdout, "Indexed codebase: %d files, %d symbols, %d relationships\n",
			graph.value.Files, graph.value.Symbols, len(graph.value.Edges))
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
	fmt.Fprintf(stdout, "Indexed codebase: %d files, %d symbols, %d relationships\n",
		graph.Files, graph.Symbols, len(graph.Edges))
	fmt.Fprintf(stdout, "Graph: %s\nSummary: %s\n",
		filepath.Join(ws.DotDir(), "codebase", "graph.json"),
		filepath.Join(ws.DotDir(), "codebase", "summary.md"))
	return nil
}

func runNew(ws *workspace.Workspace, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("new", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	description := flags.String("description", "", "feature description")
	acceptance := flags.String("acceptance", "", "semicolon-separated acceptance criteria")
	priority := flags.String("priority", "medium", "priority")
	args, err := intersperseFlags(args, map[string]bool{
		"--description": true,
		"--acceptance":  true,
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
	id, err := ws.NextFeatureID(title)
	if err != nil {
		return err
	}
	criteria := splitList(*acceptance)
	feature := model.Feature{
		ID: id, Title: title, Description: *description, Acceptance: criteria,
		Priority: *priority, Status: "todo",
	}
	if err := ws.SaveFeature(feature); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Created %s\n", id)
	return nil
}

func runStatus(ws *workspace.Workspace, stdout io.Writer) error {
	features, err := ws.ListFeatures()
	if err != nil {
		return err
	}
	if len(features) == 0 {
		fmt.Fprintln(stdout, "No features.")
		return nil
	}
	fmt.Fprintln(stdout, "ID\tSTATUS\tPHASE\tROUND\tTITLE")
	for _, feature := range features {
		phase, round := "-", 0
		if current, err := ws.ReadState(feature.ID); err == nil {
			phase, round = string(current.Phase), current.Round
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%d\t%s\n", feature.ID, feature.Status, phase, round, feature.Title)
	}
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
	fmt.Fprint(stdout, prompt)
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
	fmt.Fprintf(stdout, "%s: %s -> %s\n", feature.ID, current.Phase, next.Phase)
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
	fmt.Fprintf(stdout, "%s marked done\n", feature.ID)
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
	data, _ := json.MarshalIndent(checks, "", "  ")
	fmt.Fprintln(stdout, string(data))
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
		fmt.Fprintf(stdout, "Registered skill %s (%s) for %s\n", skill.Name, skill.Path, scope)
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
	fmt.Fprintf(stdout, "Added runner %s; synchronized %d configured skills into %s\n", name, len(config.Skills), runnerConfig.SkillsDir)
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
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = root
	command.Stdout = stdout
	command.Stderr = stderr
	command.Stdin = os.Stdin
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
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
	fmt.Fprintln(output, `Thanos — multi-role AI development framework

Usage:
  thanos init [--name NAME] [--runner codex] [--runner-command codex]
  thanos new "Feature title" [--description TEXT] [--acceptance "A; B"]
  thanos run FEATURE_ID [--runner NAME]
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
  thanos scan
  thanos version

Initialization is network-free. Skills are installed explicitly through the
open npx skills CLI; Claude plugins use Claude Code's native plugin CLI.`)
}
