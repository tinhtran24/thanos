package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

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
