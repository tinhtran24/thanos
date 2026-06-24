package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tinhtran/thanos/internal/featuregraph"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/workspace"
)

func TestRunInitIsNetworkFree(t *testing.T) {
	ws := workspace.Open(t.TempDir())
	var output strings.Builder
	if err := runInit(context.Background(), ws, nil, &output); err != nil {
		t.Fatal(err)
	}
	config, err := ws.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Skills) != 0 || len(config.Plugins) != 0 {
		t.Fatalf("init configured extensions unexpectedly: %+v", config)
	}
}

func TestRunInitScansExistingProject(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := workspace.Open(root)
	var output strings.Builder
	if err := runInit(context.Background(), ws, nil, &output); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".thanos", "codebase", "graph.json")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Indexed codebase") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestRunInitDetectsTypeScriptCommandsAndWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{
  "workspaces": ["apps/*"],
  "scripts": {"build": "turbo build", "test": "turbo test", "lint": "turbo lint"}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "package.json"), []byte(`{"name":"web"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "index.ts"), []byte("export const app = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := workspace.Open(root)
	if err := runInit(context.Background(), ws, nil, io.Discard); err != nil {
		t.Fatal(err)
	}
	config, err := ws.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Project.Language != "typescript" || !config.Project.MultiPackage {
		t.Fatalf("project = %+v", config.Project)
	}
	if !reflect.DeepEqual(config.Project.Build, []string{"npm run build"}) ||
		!reflect.DeepEqual(config.Project.Test, []string{"npm test"}) ||
		!reflect.DeepEqual(config.Project.Lint, []string{"npm run lint"}) {
		t.Fatalf("commands = %+v", config.Project)
	}
}

func TestRunInitAutoDetectsFramework(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"dependencies":{"next":"15"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.ts"), []byte("export const app = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := workspace.Open(root)
	if err := runInit(context.Background(), ws, nil, io.Discard); err != nil {
		t.Fatal(err)
	}
	config, err := ws.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Project.Framework != "nextjs" {
		t.Fatalf("framework = %q", config.Project.Framework)
	}
}

func TestRunInitLanguageOverrideSelectsFramework(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"dependencies":{"next":"15"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := workspace.Open(root)
	if err := runInit(context.Background(), ws, []string{"--language", "typescript"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	config, err := ws.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Project.Language != "typescript" || config.Project.Framework != "nextjs" {
		t.Fatalf("project = %+v", config.Project)
	}
}

func TestRunInitFrameworkOverrideIsTrimmed(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"dependencies":{"next":"15"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := workspace.Open(root)
	if err := runInit(context.Background(), ws, []string{"--language", "typescript", "--framework", " custom-web "}, io.Discard); err != nil {
		t.Fatal(err)
	}
	config, err := ws.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Project.Framework != "custom-web" {
		t.Fatalf("framework = %q", config.Project.Framework)
	}
}

func TestRunInitWhitespaceFrameworkRetainsDetection(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"dependencies":{"nuxt":"4"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := workspace.Open(root)
	if err := runInit(context.Background(), ws, []string{"--language", "typescript", "--framework", " \t "}, io.Discard); err != nil {
		t.Fatal(err)
	}
	config, err := ws.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Project.Framework != "nuxt" {
		t.Fatalf("framework = %q", config.Project.Framework)
	}
}

func TestRunInitEmptyFrameworkIsOmitted(t *testing.T) {
	ws := workspace.Open(t.TempDir())
	if err := runInit(context.Background(), ws, nil, io.Discard); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(ws.DotDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"framework"`) {
		t.Fatalf("settings contain empty framework: %s", data)
	}
}

func TestRunInitFrameworkErrorPreventsSettings(t *testing.T) {
	original := detectFramework
	detectFramework = func(root, language string) (string, error) {
		return "", errors.New("evidence unavailable")
	}
	defer func() { detectFramework = original }()

	ws := workspace.Open(t.TempDir())
	err := runInit(context.Background(), ws, nil, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "detect framework: evidence unavailable") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(ws.DotDir(), "settings.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("settings stat error = %v", statErr)
	}
}

func TestRunInitFrameworkDetectionDoesNotExecuteCommands(t *testing.T) {
	restore := stubExternal(t, func(root, name string, args []string) error {
		return errors.New("unexpected subprocess")
	})
	defer restore()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"dependencies":{"next":"15"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runInit(context.Background(), workspace.Open(root), []string{"--language", "typescript"}, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestInitHelpIncludesFramework(t *testing.T) {
	var output strings.Builder
	printHelp(&output)
	if !strings.Contains(output.String(), "--framework") {
		t.Fatalf("help = %q", output.String())
	}
}

func TestContinueHelpIsDocumented(t *testing.T) {
	var output strings.Builder
	printHelp(&output)
	if !strings.Contains(output.String(), "thanos continue FEATURE_ID") {
		t.Fatalf("help = %q", output.String())
	}
}

func TestPrepareContinueResumesLatestFailedRound(t *testing.T) {
	ws := initializedWorkspace(t)
	feature := model.Feature{ID: "F002-test", Title: "Test", Status: "todo"}
	if err := ws.SaveFeature(feature); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ws.RuntimeDir(feature.ID), "rounds", "round-3"), 0o755); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(ws.RuntimeDir(feature.ID), "rounds", "round-3", "review-report.md")
	if err := os.WriteFile(reportPath, []byte("## Verdict\nFAIL — 12/13 criteria met"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ws.WriteState(model.State{
		FeatureID: feature.ID,
		Phase:     model.PhaseAttention,
		Round:     5,
		MaxRounds: 10,
		Reason:    "maximum amendment rounds reached",
	}); err != nil {
		t.Fatal(err)
	}

	var output strings.Builder
	id, err := prepareContinue(ws, "F002", &output)
	if err != nil {
		t.Fatal(err)
	}
	if id != feature.ID {
		t.Fatalf("feature ID = %q", id)
	}
	current, err := ws.ReadState(feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Phase != model.PhaseAmend || current.Role != model.RoleCoder || current.Round != 3 {
		t.Fatalf("state = %+v", current)
	}
	if current.Reason != "" || !current.Active {
		t.Fatalf("state metadata = %+v", current)
	}
	if !strings.Contains(output.String(), "Continuing failed round 3") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestPrepareContinueRequiresFailedReport(t *testing.T) {
	ws := initializedWorkspace(t)
	feature := model.Feature{ID: "F002-test", Title: "Test", Status: "todo"}
	if err := ws.SaveFeature(feature); err != nil {
		t.Fatal(err)
	}
	if err := ws.WriteState(model.State{
		FeatureID: feature.ID,
		Phase:     model.PhaseAttention,
		Round:     3,
		MaxRounds: 3,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareContinue(ws, feature.ID, io.Discard); err == nil {
		t.Fatal("expected missing failed report error")
	}
}

func TestFrameworkDocumentation(t *testing.T) {
	required := []string{
		"project.framework", "--framework",
		"wordpress", "laravel", "nextjs", "nestjs", "angular", "nuxt",
		"gin", "echo", "django", "flask", "fastapi", "actix-web", "axum", "rocket",
		"composer.json", "artisan", "bootstrap/app.php", "wp-admin", "wp-includes", "wp-content",
		"package.json", "go.mod", "pyproject.toml", "requirements*.txt", "Cargo.toml",
		"final", "--language", "ambiguous", "omitted", "local", "read-only", "network-free",
		"package manager", "project command",
	}
	for _, document := range []string{"README.md", "Technical.md"} {
		t.Run(document, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", document))
			if err != nil {
				t.Fatal(err)
			}
			content := string(data)
			for _, token := range required {
				if !strings.Contains(content, token) {
					t.Errorf("%s missing %q", document, token)
				}
			}
		})
	}
}

func TestRunScanRefreshesGraph(t *testing.T) {
	ws := initializedWorkspace(t)
	if err := os.WriteFile(filepath.Join(ws.Root, "service.go"), []byte("package service\nfunc Run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runScan(ws, io.Discard); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(ws.DotDir(), "codebase", "summary.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Run") {
		t.Fatalf("summary = %s", data)
	}
}

func TestRunSkillAddExecutesNpxAndRegistersDiscoveredSkill(t *testing.T) {
	ws := initializedWorkspace(t)
	restore := stubExternal(t, func(root, name string, args []string) error {
		want := []string{"--yes", "skills", "add", "abc/skill", "--agent", "universal", "--yes", "--copy"}
		if name != "npx" || !reflect.DeepEqual(args, want) {
			t.Fatalf("command = %s %v, want npx %v", name, args, want)
		}
		path := filepath.Join(root, ".agents", "skills", "abc-skill", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("---\nname: abc-skill\n---"), 0o644)
	})
	defer restore()

	var output strings.Builder
	if err := runSkill(context.Background(), ws, []string{"add", "abc/skill", "--roles", "coder,tester"}, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	config, err := ws.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Skills) != 1 || config.Skills[0].Source != "abc/skill" {
		t.Fatalf("skills = %+v", config.Skills)
	}
	if got := strings.Join(config.Skills[0].Roles, ","); got != "coder,tester" {
		t.Fatalf("roles = %q", got)
	}
}

func TestRunSkillFindDelegatesToNpx(t *testing.T) {
	ws := initializedWorkspace(t)
	restore := stubExternal(t, func(_ string, name string, args []string) error {
		want := []string{"--yes", "skills", "find", "golang"}
		if name != "npx" || !reflect.DeepEqual(args, want) {
			t.Fatalf("command = %s %v", name, args)
		}
		return nil
	})
	defer restore()
	if err := runSkill(context.Background(), ws, []string{"find", "golang"}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestClaudeMarketplaceAndPluginInstall(t *testing.T) {
	ws := initializedWorkspace(t)
	var commands [][]string
	restore := stubExternal(t, func(_ string, name string, args []string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	})
	defer restore()

	if err := runPlugin(context.Background(), ws, []string{"marketplace", "add", "claude", "abc/plugins"}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := runPlugin(context.Background(), ws, []string{"install", "claude", "review@abc", "--scope", "project"}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"claude", "plugin", "marketplace", "add", "abc/plugins"},
		{"claude", "plugin", "install", "review@abc", "--scope", "project"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %v, want %v", commands, want)
	}
	config, err := ws.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(config.PluginMarketplaces) != 1 || len(config.Plugins) != 1 {
		t.Fatalf("plugin config = %+v / %+v", config.PluginMarketplaces, config.Plugins)
	}
}

func TestRunRunnerAddSynchronizesSkillsWithSymlinks(t *testing.T) {
	ws := initializedWorkspace(t)
	source := filepath.Join(ws.Root, ".agents", "skills", "testing", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("# Testing"), 0o644); err != nil {
		t.Fatal(err)
	}
	config, err := ws.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	config.Skills = []model.Skill{{
		Name: "testing", Path: ".agents/skills/testing/SKILL.md", Source: "abc/skills",
	}}
	if err := ws.WriteConfig(config); err != nil {
		t.Fatal(err)
	}

	if err := runRunner(ws, []string{"add", "claude", "--command", "claude"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(ws.Root, ".claude", "skills", "testing")
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is not a symlink", target)
	}
	config, err = ws.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Runners["claude"].Agent != "claude-code" {
		t.Fatalf("runner = %+v", config.Runners["claude"])
	}
}

func TestSkillAddSynchronizesExistingRunner(t *testing.T) {
	ws := initializedWorkspace(t)
	config, err := ws.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	config.Runners = map[string]model.Runner{
		"claude": {Command: "claude", Agent: "claude-code", SkillsDir: ".claude/skills"},
	}
	if err := ws.WriteConfig(config); err != nil {
		t.Fatal(err)
	}
	restore := stubExternal(t, func(root, _ string, _ []string) error {
		path := filepath.Join(root, ".agents", "skills", "testing", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("# Testing"), 0o644)
	})
	defer restore()
	if err := runSkill(context.Background(), ws, []string{"add", "abc/skills"}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(ws.Root, ".claude", "skills", "testing")
	if info, err := os.Lstat(target); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("synced target: info=%v err=%v", info, err)
	}
}

func TestLSPAndMCPConfiguration(t *testing.T) {
	ws := initializedWorkspace(t)
	if err := runLSP(ws, []string{"add", "go", "--command", "gopls"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := runMCP(ws, []string{
		"add", "github", "--type", "http", "--url", "https://example.test/mcp",
	}, io.Discard); err != nil {
		t.Fatal(err)
	}
	config, err := ws.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.LSP["go"].Command != "gopls" {
		t.Fatalf("lsp = %+v", config.LSP)
	}
	if config.MCP["github"].Type != "http" || config.MCP["github"].URL != "https://example.test/mcp" {
		t.Fatalf("mcp = %+v", config.MCP)
	}
}

func TestBugfixMapsToParentFeatureMemory(t *testing.T) {
	ws := initializedWorkspace(t)
	parent := model.Feature{
		ID: "F001-password-policy", Title: "Password policy", Type: "feature",
		Status: "done", Rules: []string{"Minimum length is 12"},
	}
	if err := ws.SaveFeature(parent); err != nil {
		t.Fatal(err)
	}
	if err := featuregraph.Sync(ws.DotDir(), parent); err != nil {
		t.Fatal(err)
	}
	if err := runBugfix(ws, []string{
		"F001", "Fix reset validation", "--acceptance", "Reset uses the same policy",
	}, io.Discard); err != nil {
		t.Fatal(err)
	}
	features, err := ws.ListFeatures()
	if err != nil {
		t.Fatal(err)
	}
	if len(features) != 2 || features[1].Type != "bugfix" || features[1].Parent != parent.ID {
		t.Fatalf("features = %+v", features)
	}
	memory := featuregraph.ContextMarkdown(ws.DotDir(), features[1].ID)
	if !strings.Contains(memory, "Minimum length is 12") {
		t.Fatalf("memory = %s", memory)
	}
}

func initializedWorkspace(t *testing.T) *workspace.Workspace {
	t.Helper()
	ws := workspace.Open(t.TempDir())
	if err := ws.Init(model.Config{Project: model.Project{Name: "test"}}); err != nil {
		t.Fatal(err)
	}
	return ws
}

func stubExternal(t *testing.T, fn func(root, name string, args []string) error) func() {
	t.Helper()
	original := runExternal
	runExternal = func(_ context.Context, root string, _, _ io.Writer, name string, args ...string) error {
		return fn(root, name, args)
	}
	return func() { runExternal = original }
}
