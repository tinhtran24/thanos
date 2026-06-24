package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/tinhtran/thanos/internal/model"
	"golang.org/x/mod/modfile"
)

type packageJSON struct {
	Name           string                     `json:"name"`
	PackageManager string                     `json:"packageManager"`
	Scripts        map[string]json.RawMessage `json:"scripts"`
	Workspaces     json.RawMessage            `json:"workspaces"`
}

var ignoredDirs = map[string]bool{
	".git": true, ".thanos": true, ".sense": true, "node_modules": true,
	"vendor": true, "dist": true, "build": true, ".build": true, "coverage": true,
	".next": true, ".nuxt": true, ".output": true, ".cache": true, ".turbo": true,
	".nestjs": true, ".medusa": true, ".svelte-kit": true, "__pycache__": true,
	"target": true, "out": true, "bin": true, "obj": true,
}

var (
	readFrameworkFile = os.ReadFile
	readFrameworkDir  = os.ReadDir
	statFrameworkPath = os.Stat
	pythonNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*`)
	pythonNormalize   = regexp.MustCompile(`[._-]+`)
	pythonExtraName   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	pythonVersion     = regexp.MustCompile(`^(===|~=|==|!=|<=|>=|<|>)\s*[^,;\s()<>!=~]+`)
	echoModulePattern = regexp.MustCompile(`^github\.com/labstack/echo/v[0-9]+$`)
)

var pythonMarkerVariables = map[string]bool{
	"extra":                          true,
	"implementation_name":            true,
	"implementation_version":         true,
	"os_name":                        true,
	"platform_machine":               true,
	"platform_python_implementation": true,
	"platform_release":               true,
	"platform_system":                true,
	"platform_version":               true,
	"python_full_version":            true,
	"python_version":                 true,
	"sys_platform":                   true,
}

func DetectFramework(root, language string) (string, error) {
	var (
		matches map[string]bool
		err     error
	)
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "php":
		matches, err = detectPHPFramework(root)
	case "typescript":
		matches, err = detectTypeScriptFramework(root)
	case "go":
		matches, err = detectGoFramework(root)
	case "python":
		matches, err = detectPythonFramework(root)
	case "rust":
		matches, err = detectRustFramework(root)
	default:
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if len(matches) != 1 {
		return "", nil
	}
	for match := range matches {
		return match, nil
	}
	return "", nil
}

func detectPHPFramework(root string) (map[string]bool, error) {
	matches := map[string]bool{}
	path := filepath.Join(root, "composer.json")
	data, ok, err := readOptionalFrameworkFile(path)
	if err != nil {
		return nil, err
	}
	if ok {
		var manifest struct {
			Require    map[string]json.RawMessage `json:"require"`
			RequireDev map[string]json.RawMessage `json:"require-dev"`
		}
		if json.Unmarshal(data, &manifest) == nil {
			for _, dependencies := range []map[string]json.RawMessage{manifest.Require, manifest.RequireDev} {
				for name := range dependencies {
					if strings.EqualFold(strings.TrimSpace(name), "laravel/framework") {
						matches["laravel"] = true
					}
				}
			}
		}
	}
	artisan, err := frameworkMarker(filepath.Join(root, "artisan"), false)
	if err != nil {
		return nil, err
	}
	bootstrap, err := frameworkMarker(filepath.Join(root, "bootstrap", "app.php"), false)
	if err != nil {
		return nil, err
	}
	if artisan && bootstrap {
		matches["laravel"] = true
	}
	wordpress := true
	for _, name := range []string{"wp-admin", "wp-includes", "wp-content"} {
		marker, err := frameworkMarker(filepath.Join(root, name), true)
		if err != nil {
			return nil, err
		}
		wordpress = wordpress && marker
	}
	if wordpress {
		matches["wordpress"] = true
	}
	return matches, nil
}

func detectTypeScriptFramework(root string) (map[string]bool, error) {
	matches := map[string]bool{}
	path := filepath.Join(root, "package.json")
	data, ok, err := readOptionalFrameworkFile(path)
	if err != nil || !ok {
		return matches, err
	}
	var manifest struct {
		Dependencies    map[string]json.RawMessage `json:"dependencies"`
		DevDependencies map[string]json.RawMessage `json:"devDependencies"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return matches, nil
	}
	frameworks := map[string]string{
		"next":          "nextjs",
		"@nestjs/core":  "nestjs",
		"@angular/core": "angular",
		"nuxt":          "nuxt",
	}
	for _, dependencies := range []map[string]json.RawMessage{manifest.Dependencies, manifest.DevDependencies} {
		for name := range dependencies {
			if framework := frameworks[name]; framework != "" {
				matches[framework] = true
			}
		}
	}
	return matches, nil
}

func detectGoFramework(root string) (map[string]bool, error) {
	matches := map[string]bool{}
	path := filepath.Join(root, "go.mod")
	data, ok, err := readOptionalFrameworkFile(path)
	if err != nil || !ok {
		return matches, err
	}
	parsed, err := modfile.Parse(path, data, nil)
	if err != nil {
		return matches, nil
	}
	for _, requirement := range parsed.Require {
		switch {
		case requirement.Mod.Path == "github.com/gin-gonic/gin":
			matches["gin"] = true
		case requirement.Mod.Path == "github.com/labstack/echo" || echoModulePattern.MatchString(requirement.Mod.Path):
			matches["echo"] = true
		}
	}
	return matches, nil
}

func detectPythonFramework(root string) (map[string]bool, error) {
	matches := map[string]bool{}
	path := filepath.Join(root, "pyproject.toml")
	data, ok, err := readOptionalFrameworkFile(path)
	if err != nil {
		return nil, err
	}
	if ok {
		var document map[string]any
		if toml.Unmarshal(data, &document) == nil {
			collectPythonTOML(matches, document)
		}
	}
	entries, err := readFrameworkDir(root)
	if err != nil {
		return nil, fmt.Errorf("read framework directory %s: %w", root, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "requirements") || !strings.HasSuffix(name, ".txt") {
			continue
		}
		path := filepath.Join(root, name)
		info, err := statFrameworkPath(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("stat framework evidence %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		data, err := readFrameworkFile(path)
		if err != nil {
			return nil, fmt.Errorf("read framework evidence %s: %w", path, err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			addPythonRequirement(matches, line)
		}
	}
	return matches, nil
}

func collectPythonTOML(matches map[string]bool, document map[string]any) {
	project, _ := document["project"].(map[string]any)
	addPythonRequirements(matches, project["dependencies"])
	if optional, ok := project["optional-dependencies"].(map[string]any); ok {
		for _, requirements := range optional {
			addPythonRequirements(matches, requirements)
		}
	}
	tool, _ := document["tool"].(map[string]any)
	poetry, _ := tool["poetry"].(map[string]any)
	addPoetryDependencies(matches, poetry["dependencies"])
	addPoetryDependencies(matches, poetry["dev-dependencies"])
	if groups, ok := poetry["group"].(map[string]any); ok {
		for _, rawGroup := range groups {
			if group, ok := rawGroup.(map[string]any); ok {
				addPoetryDependencies(matches, group["dependencies"])
			}
		}
	}
	if groups, ok := document["dependency-groups"].(map[string]any); ok {
		for _, requirements := range groups {
			addPythonRequirements(matches, requirements)
		}
	}
}

func addPythonRequirements(matches map[string]bool, value any) {
	list, ok := value.([]any)
	if !ok {
		return
	}
	for _, raw := range list {
		if requirement, ok := raw.(string); ok {
			addPythonRequirement(matches, requirement)
		}
	}
}

func addPoetryDependencies(matches map[string]bool, value any) {
	dependencies, ok := value.(map[string]any)
	if !ok {
		return
	}
	for name := range dependencies {
		if strings.EqualFold(name, "python") {
			continue
		}
		addPythonDistribution(matches, name)
	}
}

func addPythonRequirement(matches map[string]bool, requirement string) {
	requirement = strings.TrimSpace(stripPythonRequirementComment(requirement))
	if requirement == "" || strings.HasPrefix(requirement, "#") || strings.HasPrefix(requirement, "-") ||
		strings.HasPrefix(requirement, ".") || strings.HasPrefix(requirement, "/") {
		return
	}
	name := pythonNamePattern.FindString(requirement)
	if name == "" {
		return
	}
	if !validPythonRequirementSuffix(requirement[len(name):]) {
		return
	}
	addPythonDistribution(matches, name)
}

func stripPythonRequirementComment(requirement string) string {
	for index := 1; index < len(requirement); index++ {
		if requirement[index] == '#' && (requirement[index-1] == ' ' || requirement[index-1] == '\t') {
			return requirement[:index]
		}
	}
	return requirement
}

func validPythonRequirementSuffix(suffix string) bool {
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		return true
	}
	if strings.HasPrefix(suffix, "[") {
		end := strings.IndexByte(suffix, ']')
		if end < 0 || !validPythonExtras(suffix[1:end]) {
			return false
		}
		suffix = strings.TrimSpace(suffix[end+1:])
		if suffix == "" {
			return true
		}
	}
	switch suffix[0] {
	case ';':
		return validPythonMarker(suffix[1:])
	case '@':
		return validPythonDirectReference(suffix[1:])
	case '(':
		end := strings.IndexByte(suffix, ')')
		if end < 0 || !validPythonVersionSpecifiers(suffix[1:end]) {
			return false
		}
		return validPythonMarkerSuffix(suffix[end+1:])
	default:
		return validPythonVersionAndMarker(suffix)
	}
}

func validPythonExtras(extras string) bool {
	parts := strings.Split(extras, ",")
	if len(parts) == 0 {
		return false
	}
	for _, extra := range parts {
		if !pythonExtraName.MatchString(strings.TrimSpace(extra)) {
			return false
		}
	}
	return true
}

func validPythonDirectReference(reference string) bool {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return false
	}
	url, marker, hasMarker := splitPythonDirectReference(reference)
	if url == "" || strings.ContainsAny(url, " \t") {
		return false
	}
	return !hasMarker || validPythonMarker(marker)
}

func splitPythonDirectReference(reference string) (string, string, bool) {
	for index := 0; index < len(reference); index++ {
		if reference[index] != ';' {
			continue
		}
		if index == 0 || (reference[index-1] != ' ' && reference[index-1] != '\t') {
			continue
		}
		return strings.TrimSpace(reference[:index]), reference[index+1:], true
	}
	return strings.TrimSpace(reference), "", false
}

func validPythonVersionAndMarker(suffix string) bool {
	version, marker, hasMarker := strings.Cut(suffix, ";")
	if !validPythonVersionSpecifiers(version) {
		return false
	}
	return !hasMarker || validPythonMarker(marker)
}

func validPythonVersionSpecifiers(specifiers string) bool {
	specifiers = strings.TrimSpace(specifiers)
	if specifiers == "" {
		return false
	}
	for specifiers != "" {
		match := pythonVersion.FindString(specifiers)
		if match == "" {
			return false
		}
		specifiers = strings.TrimSpace(specifiers[len(match):])
		if specifiers == "" {
			return true
		}
		if specifiers[0] != ',' {
			return false
		}
		specifiers = strings.TrimSpace(specifiers[1:])
	}
	return false
}

func validPythonMarkerSuffix(suffix string) bool {
	suffix = strings.TrimSpace(suffix)
	return suffix == "" || (suffix[0] == ';' && validPythonMarker(suffix[1:]))
}

type pythonMarkerParser struct {
	input string
	index int
}

func validPythonMarker(marker string) bool {
	parser := pythonMarkerParser{input: marker}
	if !parser.parseOr() {
		return false
	}
	parser.skipSpace()
	return parser.index == len(parser.input)
}

func (parser *pythonMarkerParser) parseOr() bool {
	if !parser.parseAnd() {
		return false
	}
	for {
		start := parser.index
		if !parser.consumeWord("or") {
			parser.index = start
			return true
		}
		if !parser.parseAnd() {
			return false
		}
	}
}

func (parser *pythonMarkerParser) parseAnd() bool {
	if !parser.parseExpression() {
		return false
	}
	for {
		start := parser.index
		if !parser.consumeWord("and") {
			parser.index = start
			return true
		}
		if !parser.parseExpression() {
			return false
		}
	}
}

func (parser *pythonMarkerParser) parseExpression() bool {
	parser.skipSpace()
	if parser.consumeByte('(') {
		if !parser.parseOr() {
			return false
		}
		parser.skipSpace()
		return parser.consumeByte(')')
	}
	if !parser.parseOperand() || !parser.parseOperator() {
		return false
	}
	return parser.parseOperand()
}

func (parser *pythonMarkerParser) parseOperand() bool {
	parser.skipSpace()
	if parser.index >= len(parser.input) {
		return false
	}
	if parser.input[parser.index] == '\'' || parser.input[parser.index] == '"' {
		return parser.parseQuotedString()
	}
	start := parser.index
	for parser.index < len(parser.input) && isPythonMarkerIdentifierByte(parser.input[parser.index]) {
		parser.index++
	}
	return parser.index > start && pythonMarkerVariables[parser.input[start:parser.index]]
}

func (parser *pythonMarkerParser) parseQuotedString() bool {
	quote := parser.input[parser.index]
	parser.index++
	for parser.index < len(parser.input) {
		switch parser.input[parser.index] {
		case '\\':
			parser.index++
			if parser.index >= len(parser.input) {
				return false
			}
			parser.index++
		case quote:
			parser.index++
			return true
		default:
			parser.index++
		}
	}
	return false
}

func (parser *pythonMarkerParser) parseOperator() bool {
	parser.skipSpace()
	for _, operator := range []string{"===", "~=", "==", "!=", "<=", ">=", "<", ">"} {
		if strings.HasPrefix(parser.input[parser.index:], operator) {
			parser.index += len(operator)
			return true
		}
	}
	start := parser.index
	if parser.consumeWord("in") {
		return true
	}
	parser.index = start
	if !parser.consumeWord("not") {
		return false
	}
	return parser.consumeWord("in")
}

func (parser *pythonMarkerParser) consumeWord(word string) bool {
	parser.skipSpace()
	end := parser.index + len(word)
	if end > len(parser.input) || parser.input[parser.index:end] != word {
		return false
	}
	if end < len(parser.input) && isPythonMarkerIdentifierByte(parser.input[end]) {
		return false
	}
	parser.index = end
	return true
}

func (parser *pythonMarkerParser) consumeByte(value byte) bool {
	if parser.index >= len(parser.input) || parser.input[parser.index] != value {
		return false
	}
	parser.index++
	return true
}

func (parser *pythonMarkerParser) skipSpace() {
	for parser.index < len(parser.input) && (parser.input[parser.index] == ' ' || parser.input[parser.index] == '\t') {
		parser.index++
	}
}

func isPythonMarkerIdentifierByte(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9'
}

func addPythonDistribution(matches map[string]bool, name string) {
	normalized := pythonNormalize.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	switch normalized {
	case "django", "flask", "fastapi":
		matches[normalized] = true
	}
}

func detectRustFramework(root string) (map[string]bool, error) {
	matches := map[string]bool{}
	path := filepath.Join(root, "Cargo.toml")
	data, ok, err := readOptionalFrameworkFile(path)
	if err != nil || !ok {
		return matches, err
	}
	var document map[string]any
	if toml.Unmarshal(data, &document) != nil {
		return matches, nil
	}
	for _, key := range []string{"dependencies", "dev-dependencies", "build-dependencies"} {
		addCargoDependencies(matches, document[key])
	}
	if workspace, ok := document["workspace"].(map[string]any); ok {
		addCargoDependencies(matches, workspace["dependencies"])
	}
	if targets, ok := document["target"].(map[string]any); ok {
		for _, rawTarget := range targets {
			if target, ok := rawTarget.(map[string]any); ok {
				for _, key := range []string{"dependencies", "dev-dependencies", "build-dependencies"} {
					addCargoDependencies(matches, target[key])
				}
			}
		}
	}
	return matches, nil
}

func addCargoDependencies(matches map[string]bool, value any) {
	dependencies, ok := value.(map[string]any)
	if !ok {
		return
	}
	for name, declaration := range dependencies {
		switch table := declaration.(type) {
		case string:
		case map[string]any:
			if packageName, ok := table["package"].(string); ok {
				name = packageName
			}
		default:
			continue
		}
		normalized := strings.ReplaceAll(strings.ToLower(name), "_", "-")
		switch normalized {
		case "actix-web", "axum", "rocket":
			matches[normalized] = true
		}
	}
}

func readOptionalFrameworkFile(path string) ([]byte, bool, error) {
	data, err := readFrameworkFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read framework evidence %s: %w", path, err)
	}
	return data, true, nil
}

func frameworkMarker(path string, directory bool) (bool, error) {
	info, err := statFrameworkPath(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat framework marker %s: %w", path, err)
	}
	if directory {
		return info.IsDir(), nil
	}
	return info.Mode().IsRegular(), nil
}

func Detect(root string, languages map[string]int) (model.Project, error) {
	detected := model.Project{
		Name:     filepath.Base(root),
		Language: primaryLanguage(root, languages),
	}

	manifests, err := packageManifests(root)
	if err != nil {
		return detected, err
	}
	packageRoots, err := detectPackageRoots(root)
	if err != nil {
		return detected, err
	}
	if len(manifests) > 0 {
		rootManifest, hasRoot := manifests["."]
		detected.PackageManager = packageManager(root, rootManifest.PackageManager)
		if hasRoot {
			if rootManifest.Name != "" {
				detected.Name = rootManifest.Name
			}
			detected.Build = scriptCommand(detected.PackageManager, rootManifest.Scripts, "build")
			detected.Test = scriptCommand(detected.PackageManager, rootManifest.Scripts, "test")
			detected.Lint = scriptCommand(detected.PackageManager, rootManifest.Scripts, "lint")
		}
		detected.MultiPackage = len(packageRoots) > 1 || (hasRoot && hasWorkspaces(rootManifest.Workspaces))
		if detected.MultiPackage {
			if len(detected.Build) == 0 {
				detected.Build = packageScriptCommands(detected.PackageManager, manifests, "build")
			}
			if len(detected.Test) == 0 {
				detected.Test = packageScriptCommands(detected.PackageManager, manifests, "test")
			}
			if len(detected.Lint) == 0 {
				detected.Lint = packageScriptCommands(detected.PackageManager, manifests, "lint")
			}
		}
	}
	detected.MultiPackage = detected.MultiPackage || len(packageRoots) > 1 || workspaceMarkerExists(root)
	if detected.MultiPackage {
		detected.Packages = packageRoots
	}

	if len(detected.Build)+len(detected.Test)+len(detected.Lint) == 0 {
		applyLanguageCommands(root, &detected)
	}
	return detected, nil
}

func detectPackageRoots(root string) ([]string, error) {
	markers := map[string]bool{
		"package.json": true, "go.mod": true, "Cargo.toml": true,
		"pyproject.toml": true, "pom.xml": true, "build.gradle": true,
		"build.gradle.kts": true,
	}
	roots := map[string]bool{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != root && ignoredDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !markers[info.Name()] {
			return nil
		}
		dir, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		roots[filepath.ToSlash(dir)] = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(roots))
	for path := range roots {
		result = append(result, path)
	}
	sort.Strings(result)
	return result, nil
}

func workspaceMarkerExists(root string) bool {
	for _, name := range []string{"pnpm-workspace.yaml", "go.work", "lerna.json"} {
		if fileExists(filepath.Join(root, name)) {
			return true
		}
	}
	return false
}

func packageManifests(root string) (map[string]packageJSON, error) {
	result := map[string]packageJSON{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != root && ignoredDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() != "package.json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var manifest packageJSON
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil
		}
		dir, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		result[filepath.ToSlash(dir)] = manifest
		return nil
	})
	return result, err
}

func primaryLanguage(root string, languages map[string]int) string {
	if fileExists(filepath.Join(root, "tsconfig.json")) && languages["typescript"] > 0 {
		return "typescript"
	}
	best := ""
	bestCount := 0
	for language, count := range languages {
		if count > bestCount || count == bestCount && language < best {
			best, bestCount = language, count
		}
	}
	if best != "" {
		return best
	}
	switch {
	case fileExists(filepath.Join(root, "go.mod")):
		return "go"
	case fileExists(filepath.Join(root, "Cargo.toml")):
		return "rust"
	case fileExists(filepath.Join(root, "pyproject.toml")):
		return "python"
	case fileExists(filepath.Join(root, "package.json")):
		return "javascript"
	default:
		return "unknown"
	}
}

func packageManager(root, declared string) string {
	if declared != "" {
		return strings.SplitN(declared, "@", 2)[0]
	}
	for _, item := range []struct {
		file string
		name string
	}{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"bun.lockb", "bun"},
		{"bun.lock", "bun"},
		{"package-lock.json", "npm"},
		{"npm-shrinkwrap.json", "npm"},
	} {
		if fileExists(filepath.Join(root, item.file)) {
			return item.name
		}
	}
	return "npm"
}

func scriptCommand(manager string, scripts map[string]json.RawMessage, name string) []string {
	if _, ok := scripts[name]; !ok {
		return nil
	}
	switch manager {
	case "yarn":
		return []string{"yarn " + name}
	case "pnpm":
		if name == "test" {
			return []string{"pnpm test"}
		}
		return []string{"pnpm run " + name}
	case "bun":
		return []string{"bun run " + name}
	default:
		if name == "test" {
			return []string{"npm test"}
		}
		return []string{"npm run " + name}
	}
}

func packageScriptCommands(manager string, manifests map[string]packageJSON, name string) []string {
	var paths []string
	for path, manifest := range manifests {
		if path == "." {
			continue
		}
		if _, ok := manifest.Scripts[name]; ok {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	commands := make([]string, 0, len(paths))
	for _, path := range paths {
		switch manager {
		case "yarn":
			commands = append(commands, "yarn --cwd "+path+" "+name)
		case "pnpm":
			commands = append(commands, "pnpm --dir "+path+" "+scriptInvocation(manager, name))
		case "bun":
			commands = append(commands, "bun --cwd "+path+" run "+name)
		default:
			commands = append(commands, "npm --prefix "+path+" "+scriptInvocation(manager, name))
		}
	}
	return commands
}

func scriptInvocation(manager, name string) string {
	if name == "test" && manager != "bun" {
		return "test"
	}
	return "run " + name
}

func hasWorkspaces(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return len(list) > 0
	}
	var object struct {
		Packages []string `json:"packages"`
	}
	return json.Unmarshal(raw, &object) == nil && len(object.Packages) > 0
}

func applyLanguageCommands(root string, detected *model.Project) {
	switch detected.Language {
	case "go":
		detected.Build = []string{"go build ./..."}
		detected.Test = []string{"go test ./..."}
		detected.Lint = []string{"go vet ./..."}
	case "rust":
		detected.Build = []string{"cargo build"}
		detected.Test = []string{"cargo test"}
		detected.Lint = []string{"cargo clippy --all-targets --all-features"}
	case "python":
		if fileExists(filepath.Join(root, "pyproject.toml")) {
			detected.Test = []string{"python -m pytest"}
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !errors.Is(err, os.ErrNotExist)
}
