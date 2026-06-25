package featuregraph

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tinhtran/thanos/internal/codegraph"
	"github.com/tinhtran/thanos/internal/model"
)

const Version = 1

type Path struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Source string `json:"source"`
}

type Feature struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	Parent       string    `json:"parent,omitempty"`
	Rules        []string  `json:"rules,omitempty"`
	Decisions    []string  `json:"decisions,omitempty"`
	Acceptance   []string  `json:"acceptance,omitempty"`
	Scope        []string  `json:"scope,omitempty"`
	Dependencies []string  `json:"dependencies,omitempty"`
	Related      []string  `json:"related,omitempty"`
	Paths        []Path    `json:"paths,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type Graph struct {
	Version     int       `json:"version"`
	GeneratedAt time.Time `json:"generated_at"`
	Features    []Feature `json:"features"`
	Edges       []Edge    `json:"edges"`
}

type Context struct {
	Target    Feature
	Features  []Feature
	Paths     []Path
	Rules     []string
	Decisions []string
}

func PathFor(dotDir string) string {
	return filepath.Join(dotDir, "memory", "feature-graph.json")
}

func SummaryPath(dotDir string) string {
	return filepath.Join(dotDir, "memory", "feature-graph.md")
}

func Load(dotDir string) (Graph, error) {
	var graph Graph
	file, err := os.Open(PathFor(dotDir))
	if errors.Is(err, os.ErrNotExist) {
		return Graph{Version: Version}, nil
	}
	if err != nil {
		return graph, err
	}
	defer file.Close()
	if err := json.NewDecoder(bufio.NewReader(file)).Decode(&graph); err != nil {
		return graph, err
	}
	if graph.Version == 0 {
		graph.Version = Version
	}
	return graph, nil
}

func Save(dotDir string, graph Graph) error {
	graph.Version = Version
	graph.GeneratedAt = time.Now().UTC()
	sort.Slice(graph.Features, func(i, j int) bool { return graph.Features[i].ID < graph.Features[j].ID })
	sort.Slice(graph.Edges, func(i, j int) bool {
		if graph.Edges[i].From != graph.Edges[j].From {
			return graph.Edges[i].From < graph.Edges[j].From
		}
		if graph.Edges[i].Kind != graph.Edges[j].Kind {
			return graph.Edges[i].Kind < graph.Edges[j].Kind
		}
		return graph.Edges[i].To < graph.Edges[j].To
	})
	dir := filepath.Dir(PathFor(dotDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(PathFor(dotDir), data, 0o644); err != nil {
		return err
	}
	return os.WriteFile(SummaryPath(dotDir), []byte(Summary(graph)), 0o644)
}

func Sync(dotDir string, feature model.Feature) error {
	graph, err := Load(dotDir)
	if err != nil {
		return err
	}
	node := nodeFromFeature(feature)
	if existing, ok := findFeature(graph, feature.ID); ok {
		node.Paths = mergePaths(existing.Paths, node.Paths)
		node.Rules = mergeStrings(existing.Rules, node.Rules)
		node.Decisions = mergeStrings(existing.Decisions, node.Decisions)
	}
	upsertFeature(&graph, node)
	rebuildEdges(&graph)
	return Save(dotDir, graph)
}

func Rebuild(dotDir string, features []model.Feature) error {
	existing, err := Load(dotDir)
	if err != nil {
		return err
	}
	existingByID := make(map[string]Feature, len(existing.Features))
	for _, item := range existing.Features {
		existingByID[item.ID] = item
	}
	graph := Graph{Version: Version}
	for _, feature := range features {
		node := nodeFromFeature(feature)
		if previous, ok := existingByID[feature.ID]; ok {
			node.Paths = mergePaths(previous.Paths, node.Paths)
			node.Rules = mergeStrings(previous.Rules, node.Rules)
			node.Decisions = mergeStrings(previous.Decisions, node.Decisions)
		}
		graph.Features = append(graph.Features, node)
	}
	rebuildEdges(&graph)
	return Save(dotDir, graph)
}

func UpdateFromArtifacts(dotDir string, feature model.Feature) error {
	graph, err := Load(dotDir)
	if err != nil {
		return err
	}
	node := nodeFromFeature(feature)
	if existing, ok := findFeature(graph, feature.ID); ok {
		node.Paths = mergePaths(existing.Paths, node.Paths)
		node.Rules = mergeStrings(existing.Rules, node.Rules)
		node.Decisions = mergeStrings(existing.Decisions, node.Decisions)
	}
	for _, path := range feature.Scope {
		if looksLikePath(path) {
			node.Paths = appendUniquePath(node.Paths, Path{
				Path: filepath.ToSlash(filepath.Clean(path)), Kind: classifyPath(path), Source: "declared-scope",
			})
		}
	}
	memoryPath := filepath.Join(dotDir, feature.ID, "feature-memory.json")
	if data, readErr := os.ReadFile(memoryPath); readErr == nil {
		var learned struct {
			BusinessRules          []string `json:"business_rules"`
			ArchitecturalDecisions []string `json:"architectural_decisions"`
			AffectedPaths          []string `json:"affected_paths"`
		}
		if err := json.Unmarshal(data, &learned); err != nil {
			return fmt.Errorf("%s: %w", memoryPath, err)
		}
		for _, rule := range learned.BusinessRules {
			node.Rules = appendUniqueString(node.Rules, rule)
		}
		for _, decision := range learned.ArchitecturalDecisions {
			node.Decisions = appendUniqueString(node.Decisions, decision)
		}
		for _, path := range learned.AffectedPaths {
			if looksLikePath(path) {
				node.Paths = appendUniquePath(node.Paths, Path{
					Path: path, Kind: classifyPath(path), Source: "feature-memory",
				})
			}
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	reports, err := filepath.Glob(filepath.Join(dotDir, feature.ID, "*", "implementation-note.md"))
	if err != nil {
		return err
	}
	if rootReport := filepath.Join(dotDir, feature.ID, "implementation-note.md"); fileExists(rootReport) {
		reports = append(reports, rootReport)
	}
	for _, report := range reports {
		paths, readErr := pathsFromCoderReport(report)
		if readErr != nil {
			return readErr
		}
		for _, path := range paths {
			node.Paths = appendUniquePath(node.Paths, Path{
				Path: path, Kind: classifyPath(path), Source: "implementation-note",
			})
		}
	}
	upsertFeature(&graph, node)
	rebuildEdges(&graph)
	return Save(dotDir, graph)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func Resolve(dotDir, featureID string) (Context, error) {
	graph, err := Load(dotDir)
	if err != nil {
		return Context{}, err
	}
	target, ok := findFeature(graph, featureID)
	if !ok {
		return Context{}, fmt.Errorf("feature memory %q not found", featureID)
	}
	selected := map[string]bool{target.ID: true}
	var queue []string
	if target.Parent != "" {
		queue = append(queue, target.Parent)
	}
	queue = append(queue, target.Dependencies...)
	queue = append(queue, target.Related...)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if selected[id] {
			continue
		}
		item, exists := findFeature(graph, id)
		if !exists {
			continue
		}
		selected[id] = true
		if item.Parent != "" {
			queue = append(queue, item.Parent)
		}
		queue = append(queue, item.Dependencies...)
		queue = append(queue, item.Related...)
	}
	context := Context{Target: target}
	for _, item := range graph.Features {
		if !selected[item.ID] {
			continue
		}
		context.Features = append(context.Features, item)
		for _, rule := range append(append([]string{}, item.Rules...), item.Acceptance...) {
			context.Rules = appendUniqueString(context.Rules, rule)
		}
		for _, decision := range item.Decisions {
			context.Decisions = appendUniqueString(context.Decisions, decision)
		}
		for _, path := range item.Paths {
			context.Paths = appendUniquePath(context.Paths, path)
		}
	}
	context.Paths = expandCodeImpact(dotDir, context.Paths)
	sort.Slice(context.Features, func(i, j int) bool { return context.Features[i].ID < context.Features[j].ID })
	sort.Slice(context.Paths, func(i, j int) bool { return context.Paths[i].Path < context.Paths[j].Path })
	sort.Strings(context.Rules)
	sort.Strings(context.Decisions)
	return context, nil
}

func ContextMarkdown(dotDir, featureID string) string {
	context, err := Resolve(dotDir, featureID)
	if err != nil {
		return "No stored feature memory is available yet."
	}
	var output strings.Builder
	fmt.Fprintf(&output, "Target: %s — %s (%s)\n", context.Target.ID, context.Target.Title, context.Target.Type)
	if context.Target.Parent != "" {
		fmt.Fprintf(&output, "Bugfix parent: %s\n", context.Target.Parent)
	}
	if len(context.Features) > 1 {
		output.WriteString("Related feature context:\n")
		for _, feature := range context.Features {
			if feature.ID != context.Target.ID {
				fmt.Fprintf(&output, "- %s — %s [%s]\n", feature.ID, feature.Title, feature.Status)
			}
		}
	}
	if len(context.Rules) > 0 {
		output.WriteString("Business rules and acceptance invariants:\n")
		for _, rule := range context.Rules {
			fmt.Fprintf(&output, "- %s\n", rule)
		}
	}
	if len(context.Decisions) > 0 {
		output.WriteString("Architectural decisions:\n")
		for _, decision := range context.Decisions {
			fmt.Fprintf(&output, "- %s\n", decision)
		}
	}
	if len(context.Paths) > 0 {
		output.WriteString("Known and inferred impact paths:\n")
		for _, path := range context.Paths {
			fmt.Fprintf(&output, "- %s [%s; %s]\n", path.Path, path.Kind, path.Source)
		}
	}
	return strings.TrimSpace(output.String())
}

func Summary(graph Graph) string {
	var output strings.Builder
	output.WriteString("# Thanos Feature Memory\n\n")
	fmt.Fprintf(&output, "Generated: %s\n\n", graph.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&output, "- Features: %d\n- Relationships: %d\n\n", len(graph.Features), len(graph.Edges))
	for _, feature := range graph.Features {
		fmt.Fprintf(&output, "## %s — %s\n\n", feature.ID, feature.Title)
		fmt.Fprintf(&output, "- Type: %s\n- Status: %s\n", feature.Type, feature.Status)
		if feature.Parent != "" {
			fmt.Fprintf(&output, "- Fixes: %s\n", feature.Parent)
		}
		if len(feature.Rules) > 0 {
			output.WriteString("- Rules:\n")
			for _, rule := range feature.Rules {
				fmt.Fprintf(&output, "  - %s\n", rule)
			}
		}
		if len(feature.Decisions) > 0 {
			output.WriteString("- Architectural decisions:\n")
			for _, decision := range feature.Decisions {
				fmt.Fprintf(&output, "  - %s\n", decision)
			}
		}
		if len(feature.Paths) > 0 {
			output.WriteString("- Affected paths:\n")
			for _, path := range feature.Paths {
				fmt.Fprintf(&output, "  - `%s` (%s)\n", path.Path, path.Kind)
			}
		}
		output.WriteString("\n")
	}
	return output.String()
}

func nodeFromFeature(feature model.Feature) Feature {
	kind := feature.Type
	if kind == "" {
		kind = "feature"
	}
	node := Feature{
		ID: feature.ID, Title: feature.Title, Type: kind, Status: feature.Status,
		Parent: feature.Parent, Rules: uniqueStrings(feature.Rules),
		Decisions:  uniqueStrings(feature.Decisions),
		Acceptance: uniqueStrings(feature.Acceptance), Scope: uniqueStrings(feature.Scope),
		Dependencies: uniqueStrings(feature.Dependencies), Related: uniqueStrings(feature.Related),
		UpdatedAt: time.Now().UTC(),
	}
	for _, path := range feature.Scope {
		if looksLikePath(path) {
			node.Paths = appendUniquePath(node.Paths, Path{
				Path: path, Kind: classifyPath(path), Source: "declared-scope",
			})
		}
	}
	return node
}

func rebuildEdges(graph *Graph) {
	graph.Edges = nil
	for _, feature := range graph.Features {
		if feature.Parent != "" {
			graph.Edges = append(graph.Edges, Edge{From: feature.ID, To: feature.Parent, Kind: "fixes"})
		}
		for _, dependency := range feature.Dependencies {
			graph.Edges = append(graph.Edges, Edge{From: feature.ID, To: dependency, Kind: "depends-on"})
		}
		for _, related := range feature.Related {
			graph.Edges = append(graph.Edges, Edge{From: feature.ID, To: related, Kind: "related-to"})
		}
	}
}

func findFeature(graph Graph, id string) (Feature, bool) {
	for _, feature := range graph.Features {
		if feature.ID == id || strings.HasPrefix(feature.ID, id) {
			return feature, true
		}
	}
	return Feature{}, false
}

func upsertFeature(graph *Graph, feature Feature) {
	for index := range graph.Features {
		if graph.Features[index].ID == feature.ID {
			graph.Features[index] = feature
			return
		}
	}
	graph.Features = append(graph.Features, feature)
}

func pathsFromCoderReport(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var paths []string
	inFiles := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "## files changed") {
			inFiles = true
			continue
		}
		if inFiles && strings.HasPrefix(trimmed, "## ") {
			break
		}
		if !inFiles || (!strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "* ")) {
			continue
		}
		value := strings.TrimSpace(trimmed[2:])
		for _, separator := range []string{" — ", " - ", ": "} {
			if index := strings.Index(value, separator); index > 0 {
				value = value[:index]
				break
			}
		}
		value = strings.Trim(strings.TrimSpace(value), "`")
		if looksLikePath(value) {
			paths = appendUniqueString(paths, filepath.ToSlash(filepath.Clean(value)))
		}
	}
	return paths, nil
}

func expandCodeImpact(dotDir string, paths []Path) []Path {
	data, err := os.ReadFile(filepath.Join(dotDir, "codebase", "graph.json"))
	if err != nil {
		return paths
	}
	var graph codegraph.Graph
	if err := json.Unmarshal(data, &graph); err != nil {
		return paths
	}
	known := map[string]bool{}
	for _, path := range paths {
		known[filepath.ToSlash(filepath.Clean(path.Path))] = true
	}
	fileForNode := make(map[string]string, len(graph.Nodes))
	for _, node := range graph.Nodes {
		fileForNode[node.ID] = node.File
	}
	for _, edge := range graph.Edges {
		fromFile, fromOK := fileForNode[edge.From]
		toFile, toOK := fileForNode[edge.To]
		if !fromOK || !toOK || fromFile == "" || toFile == "" || fromFile == toFile {
			continue
		}
		fromKnown, toKnown := known[fromFile], known[toFile]
		if fromKnown && !toKnown {
			paths = appendUniquePath(paths, Path{Path: toFile, Kind: classifyPath(toFile), Source: "code-graph-neighbor"})
		}
		if toKnown && !fromKnown {
			paths = appendUniquePath(paths, Path{Path: fromFile, Kind: classifyPath(fromFile), Source: "code-graph-neighbor"})
		}
	}
	return paths
}

func classifyPath(path string) string {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := strings.ToLower(filepath.Base(lower))
	switch {
	case strings.Contains(base, "_test.") || strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") || strings.HasPrefix(base, "test_"):
		return "test"
	case strings.HasSuffix(lower, ".md") || strings.Contains(lower, "/docs/") || strings.HasPrefix(lower, "docs/"):
		return "documentation"
	case strings.Contains(lower, "schema") || strings.Contains(lower, "contract") ||
		strings.Contains(lower, "openapi") || strings.Contains(lower, "graphql"):
		return "contract"
	case strings.Contains(lower, "/ui/") || strings.HasPrefix(lower, "ui/") ||
		strings.Contains(lower, "/web/") || strings.HasPrefix(lower, "web/") ||
		strings.Contains(lower, "/frontend/") || strings.HasPrefix(lower, "frontend/") ||
		strings.HasSuffix(lower, ".tsx") ||
		strings.HasSuffix(lower, ".jsx"):
		return "frontend"
	default:
		return "code"
	}
}

func looksLikePath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "\n") || strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return false
	}
	return strings.Contains(value, "/") || filepath.Ext(value) != ""
}

func appendUniquePath(items []Path, item Path) []Path {
	item.Path = filepath.ToSlash(filepath.Clean(item.Path))
	for index, existing := range items {
		if existing.Path != item.Path {
			continue
		}
		if existing.Source == item.Source || sourceRank(item.Source) > sourceRank(existing.Source) {
			items[index] = item
		}
		return items
	}
	return append(items, item)
}

func sourceRank(source string) int {
	switch source {
	case "feature-memory":
		return 3
	case "implementation-note", "coder-report":
		return 2
	case "declared-scope":
		return 1
	default:
		return 0
	}
}

func appendUniqueString(items []string, item string) []string {
	item = strings.TrimSpace(item)
	if item == "" {
		return items
	}
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

func uniqueStrings(items []string) []string {
	var result []string
	for _, item := range items {
		result = appendUniqueString(result, item)
	}
	sort.Strings(result)
	return result
}

func mergeStrings(groups ...[]string) []string {
	var result []string
	for _, group := range groups {
		for _, item := range group {
			result = appendUniqueString(result, item)
		}
	}
	sort.Strings(result)
	return result
}

func mergePaths(groups ...[]Path) []Path {
	var result []Path
	for _, group := range groups {
		for _, item := range group {
			result = appendUniquePath(result, item)
		}
	}
	return result
}
