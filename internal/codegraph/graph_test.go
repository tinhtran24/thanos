package codegraph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildGoCallGraphAndSummary(t *testing.T) {
	root := t.TempDir()
	source := `package sample

func Serve() { Load() }
func Load() {}
`
	testSource := `package sample
import "testing"
func TestServe(t *testing.T) { Serve() }
`
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sample_test.go"), []byte(testSource), 0o644); err != nil {
		t.Fatal(err)
	}
	graph, err := Build(root)
	if err != nil {
		t.Fatal(err)
	}
	if graph.Files != 2 || graph.Symbols != 3 {
		t.Fatalf("graph counts = files:%d symbols:%d", graph.Files, graph.Symbols)
	}
	callEdges := 0
	for _, edge := range graph.Edges {
		if edge.Kind == "calls" {
			callEdges++
		}
	}
	if callEdges != 2 {
		t.Fatalf("call edges = %d, want 2", callEdges)
	}
	summary := Summary(graph)
	if !strings.Contains(summary, "Hub Symbols") || !strings.Contains(summary, "Detected Conventions") {
		t.Fatalf("summary missing graph context:\n%s", summary)
	}
}

func TestSaveWritesLocalGraph(t *testing.T) {
	root := t.TempDir()
	dotDir := filepath.Join(root, ".thanos")
	graph := Graph{Version: 1, Languages: map[string]int{"go": 1}}
	if err := Save(graph, dotDir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"graph.json", "summary.md"} {
		if _, err := os.Stat(filepath.Join(dotDir, "codebase", name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHasSourceIgnoresGeneratedDirectories(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "node_modules", "package", "index.js")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("function hidden() {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if HasSource(root) {
		t.Fatal("node_modules source should be ignored")
	}
}
