package codegraph

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Node struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Language string `json:"language,omitempty"`
	Package  string `json:"package,omitempty"`
	Test     bool   `json:"test,omitempty"`
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type Convention struct {
	Name     string `json:"name"`
	Evidence string `json:"evidence"`
}

type Graph struct {
	Version     int            `json:"version"`
	Root        string         `json:"root"`
	GeneratedAt time.Time      `json:"generated_at"`
	Files       int            `json:"files"`
	Symbols     int            `json:"symbols"`
	Nodes       []Node         `json:"nodes"`
	Edges       []Edge         `json:"edges"`
	Languages   map[string]int `json:"languages"`
	Conventions []Convention   `json:"conventions"`
	Ignored     []string       `json:"ignored"`
}

var (
	jsFunction = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)|^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?\(`)
	classDecl  = regexp.MustCompile(`(?m)^\s*(?:export\s+)?class\s+([A-Za-z_$][\w$]*)`)
	pyFunction = regexp.MustCompile(`(?m)^\s*(?:async\s+)?def\s+([A-Za-z_]\w*)\s*\(`)
	pyClass    = regexp.MustCompile(`(?m)^\s*class\s+([A-Za-z_]\w*)`)
	callToken  = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
)

var ignoredDirs = map[string]bool{
	".git": true, ".thanos": true, ".sense": true, "node_modules": true,
	"vendor": true, "dist": true, "build": true, ".build": true, "coverage": true,
	".next": true, ".nuxt": true, ".output": true, ".cache": true, ".turbo": true,
	".nestjs": true, ".medusa": true, ".svelte-kit": true, "__pycache__": true,
	"target": true, "out": true, "bin": true, "obj": true,
}

var ignoredDirNames = func() []string {
	names := make([]string, 0, len(ignoredDirs))
	for name := range ignoredDirs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}()

func HasSource(root string) bool {
	found := false
	_ = walkSource(root, func(string, string) error {
		found = true
		return filepath.SkipAll
	})
	return found
}

func Build(root string) (Graph, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Graph{}, err
	}
	graph := Graph{
		Version: 1, Root: absolute, GeneratedAt: time.Now().UTC(),
		Languages: map[string]int{},
		Ignored:   append([]string(nil), ignoredDirNames...),
	}
	var pending []pendingCall
	symbolsByName := map[string][]string{}

	err = walkSource(absolute, func(path, language string) error {
		relative, err := filepath.Rel(absolute, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fileID := "file:" + relative
		testFile := isTestFile(relative)
		graph.Nodes = append(graph.Nodes, Node{
			ID: fileID, Kind: "file", Name: filepath.Base(relative), File: relative,
			Language: language, Test: testFile,
		})
		graph.Files++
		graph.Languages[language]++

		if language == "go" {
			nodes, edges, calls := parseGo(relative, data)
			for _, node := range nodes {
				graph.Nodes = append(graph.Nodes, node)
				graph.Symbols++
				symbolsByName[node.Name] = append(symbolsByName[node.Name], node.ID)
				graph.Edges = append(graph.Edges, Edge{From: fileID, To: node.ID, Kind: "defines"})
			}
			graph.Edges = append(graph.Edges, edges...)
			pending = append(pending, calls...)
			return nil
		}

		nodes := parseGeneric(relative, language, data)
		for _, node := range nodes {
			graph.Nodes = append(graph.Nodes, node)
			graph.Symbols++
			symbolsByName[node.Name] = append(symbolsByName[node.Name], node.ID)
			graph.Edges = append(graph.Edges, Edge{From: fileID, To: node.ID, Kind: "defines"})
			for _, match := range callToken.FindAllStringSubmatch(string(data), -1) {
				pending = append(pending, pendingCall{from: node.ID, name: match[1]})
			}
		}
		return nil
	})
	if err != nil {
		return Graph{}, err
	}

	for _, call := range pending {
		targets := symbolsByName[call.name]
		if len(targets) == 1 && targets[0] != call.from {
			graph.Edges = append(graph.Edges, Edge{From: call.from, To: targets[0], Kind: "calls"})
		}
	}
	graph.Conventions = detectConventions(graph)
	sort.Slice(graph.Nodes, func(i, j int) bool { return graph.Nodes[i].ID < graph.Nodes[j].ID })
	sort.Slice(graph.Edges, func(i, j int) bool {
		if graph.Edges[i].From != graph.Edges[j].From {
			return graph.Edges[i].From < graph.Edges[j].From
		}
		if graph.Edges[i].Kind != graph.Edges[j].Kind {
			return graph.Edges[i].Kind < graph.Edges[j].Kind
		}
		return graph.Edges[i].To < graph.Edges[j].To
	})
	return graph, nil
}

func Save(graph Graph, dotDir string) error {
	dir := filepath.Join(dotDir, "codebase")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(dir, "graph.json"), data, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "summary.md"), []byte(Summary(graph)), 0o644)
}

func Summary(graph Graph) string {
	var output strings.Builder
	output.WriteString("# Thanos Codebase Graph\n\n")
	fmt.Fprintf(&output, "Generated: %s\n\n", graph.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&output, "- Files: %d\n- Symbols: %d\n- Relationships: %d\n", graph.Files, graph.Symbols, len(graph.Edges))
	output.WriteString("\n## Languages\n\n")
	type languageCount struct {
		name  string
		count int
	}
	var languages []languageCount
	for name, count := range graph.Languages {
		languages = append(languages, languageCount{name, count})
	}
	sort.Slice(languages, func(i, j int) bool {
		if languages[i].count != languages[j].count {
			return languages[i].count > languages[j].count
		}
		return languages[i].name < languages[j].name
	})
	for _, item := range languages {
		fmt.Fprintf(&output, "- %s: %d files\n", item.name, item.count)
	}

	output.WriteString("\n## Key Symbols\n\n")
	symbolCount := 0
	for _, node := range graph.Nodes {
		if node.Kind == "file" || node.Test {
			continue
		}
		fmt.Fprintf(&output, "- `%s` (%s) — %s:%d\n", node.Name, node.Kind, node.File, node.Line)
		symbolCount++
		if symbolCount == 20 {
			break
		}
	}
	if symbolCount == 0 {
		output.WriteString("- No symbols detected.\n")
	}

	incoming := map[string]int{}
	for _, edge := range graph.Edges {
		if edge.Kind == "calls" || edge.Kind == "imports" {
			incoming[edge.To]++
		}
	}
	nodes := map[string]Node{}
	for _, node := range graph.Nodes {
		nodes[node.ID] = node
	}
	type hub struct {
		id    string
		count int
	}
	var hubs []hub
	for id, count := range incoming {
		if _, exists := nodes[id]; !exists {
			continue
		}
		hubs = append(hubs, hub{id, count})
	}
	sort.Slice(hubs, func(i, j int) bool { return hubs[i].count > hubs[j].count })
	output.WriteString("\n## Hub Symbols\n\n")
	if len(hubs) == 0 {
		output.WriteString("- No resolved call hubs detected.\n")
	}
	for index, item := range hubs {
		if index == 10 {
			break
		}
		node := nodes[item.id]
		fmt.Fprintf(&output, "- `%s` — %d incoming relationships (%s:%d)\n", node.Name, item.count, node.File, node.Line)
	}
	output.WriteString("\n## Detected Conventions\n\n")
	for _, convention := range graph.Conventions {
		fmt.Fprintf(&output, "- **%s:** %s\n", convention.Name, convention.Evidence)
	}
	output.WriteString("\nFull machine-readable graph: `.thanos/codebase/graph.json`.\n")
	return output.String()
}

func walkSource(root string, visit func(path, language string) error) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != root && ignoredDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		language := languageFor(path)
		if language == "" || info.Size() > 2<<20 {
			return nil
		}
		return visit(path, language)
	})
}

func languageFor(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".kt", ".kts":
		return "kotlin"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".c", ".h":
		return "c"
	case ".cc", ".cpp", ".cxx", ".hpp":
		return "cpp"
	default:
		return ""
	}
}

func isTestFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return strings.Contains(base, "_test.") || strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") || strings.HasPrefix(base, "test_")
}

func parseGo(path string, data []byte) ([]Node, []Edge, []pendingCall) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, data, 0)
	if err != nil {
		return nil, nil, nil
	}
	testFile := isTestFile(path)
	var nodes []Node
	var edges []Edge
	var calls []pendingCall
	fileID := "file:" + path
	for _, spec := range file.Imports {
		target := strings.Trim(spec.Path.Value, `"`)
		edges = append(edges, Edge{From: fileID, To: "package:" + target, Kind: "imports"})
	}
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.FuncDecl:
			name := value.Name.Name
			if value.Recv != nil && len(value.Recv.List) > 0 {
				name = receiverName(value.Recv.List[0].Type) + "." + name
			}
			position := set.Position(value.Pos())
			id := fmt.Sprintf("symbol:%s:%d:%s", path, position.Line, name)
			nodes = append(nodes, Node{
				ID: id, Kind: "function", Name: value.Name.Name, File: path,
				Line: position.Line, Language: "go", Package: file.Name.Name, Test: testFile,
			})
			if value.Body != nil {
				ast.Inspect(value.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					switch target := call.Fun.(type) {
					case *ast.Ident:
						calls = append(calls, pendingCall{from: id, name: target.Name})
					case *ast.SelectorExpr:
						calls = append(calls, pendingCall{from: id, name: target.Sel.Name})
					}
					return true
				})
			}
		case *ast.GenDecl:
			for _, spec := range value.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				position := set.Position(typeSpec.Pos())
				kind := "type"
				switch typeSpec.Type.(type) {
				case *ast.InterfaceType:
					kind = "interface"
				case *ast.StructType:
					kind = "struct"
				}
				nodes = append(nodes, Node{
					ID:   fmt.Sprintf("symbol:%s:%d:%s", path, position.Line, typeSpec.Name.Name),
					Kind: kind, Name: typeSpec.Name.Name, File: path, Line: position.Line,
					Language: "go", Package: file.Name.Name, Test: testFile,
				})
			}
		}
	}
	return nodes, edges, calls
}

type pendingCall struct {
	from string
	name string
}

func receiverName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return receiverName(value.X)
	case *ast.IndexExpr:
		return receiverName(value.X)
	default:
		return "receiver"
	}
}

func parseGeneric(path, language string, data []byte) []Node {
	source := string(data)
	testFile := isTestFile(path)
	var patterns []*regexp.Regexp
	switch language {
	case "typescript", "javascript":
		patterns = []*regexp.Regexp{jsFunction, classDecl}
	case "python":
		patterns = []*regexp.Regexp{pyFunction, pyClass}
	default:
		return nil
	}
	var nodes []Node
	for _, pattern := range patterns {
		for _, match := range pattern.FindAllStringSubmatchIndex(source, -1) {
			name := ""
			start := -1
			for index := 2; index+1 < len(match); index += 2 {
				if match[index] >= 0 {
					name = source[match[index]:match[index+1]]
					start = match[index]
					break
				}
			}
			if name == "" {
				continue
			}
			line := 1 + strings.Count(source[:start], "\n")
			nodes = append(nodes, Node{
				ID:   fmt.Sprintf("symbol:%s:%d:%s", path, line, name),
				Kind: "symbol", Name: name, File: path, Line: line,
				Language: language, Test: testFile,
			})
		}
	}
	return nodes
}

func detectConventions(graph Graph) []Convention {
	var conventions []Convention
	if graph.Languages["go"] > 0 {
		conventions = append(conventions, Convention{
			Name:     "Go formatting and tests",
			Evidence: "Go source is present; use gofmt and keep tests in *_test.go files beside the package.",
		})
	}
	testCount := 0
	internalCount := 0
	for _, node := range graph.Nodes {
		if node.Kind != "file" {
			continue
		}
		if node.Test {
			testCount++
		}
		if strings.HasPrefix(node.File, "internal/") {
			internalCount++
		}
	}
	if testCount > 0 {
		conventions = append(conventions, Convention{
			Name:     "Test organization",
			Evidence: fmt.Sprintf("%d test files use language-native test naming.", testCount),
		})
	}
	if internalCount > 0 {
		conventions = append(conventions, Convention{
			Name:     "Internal package boundary",
			Evidence: fmt.Sprintf("%d files live under internal/; keep non-public implementation there.", internalCount),
		})
	}
	if len(conventions) == 0 {
		conventions = append(conventions, Convention{
			Name:     "Repository structure",
			Evidence: "Follow naming and directory patterns in adjacent source files.",
		})
	}
	return conventions
}
