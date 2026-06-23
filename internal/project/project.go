package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tinhtran/thanos/internal/model"
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
