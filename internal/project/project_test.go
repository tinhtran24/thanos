package project

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDetectTypeScriptWorkspaceAndPackageScripts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{
  "name": "store",
  "packageManager": "pnpm@9.0.0",
  "workspaces": ["apps/*"]
}`)
	writeFile(t, filepath.Join(root, "pnpm-lock.yaml"), "lockfileVersion: '9.0'\n")
	writeFile(t, filepath.Join(root, "apps", "backend", "package.json"), `{
  "scripts": {"build": "nest build", "test": "jest", "lint": "eslint ."}
}`)
	writeFile(t, filepath.Join(root, "apps", "frontend", "package.json"), `{
  "scripts": {"build": "next build", "test": "vitest"}
}`)
	writeFile(t, filepath.Join(root, "apps", "frontend", "page.tsx"), "export default function Page() {}\n")

	detected, err := Detect(root, map[string]int{"typescript": 1})
	if err != nil {
		t.Fatal(err)
	}
	if detected.Name != "store" || detected.Language != "typescript" || detected.PackageManager != "pnpm" {
		t.Fatalf("detected project = %+v", detected)
	}
	if !detected.MultiPackage {
		t.Fatal("workspace should be detected as multiple packages")
	}
	wantPackages := []string{".", "apps/backend", "apps/frontend"}
	if !reflect.DeepEqual(detected.Packages, wantPackages) {
		t.Fatalf("packages = %v, want %v", detected.Packages, wantPackages)
	}
	wantBuild := []string{
		"pnpm --dir apps/backend run build",
		"pnpm --dir apps/frontend run build",
	}
	if !reflect.DeepEqual(detected.Build, wantBuild) {
		t.Fatalf("build = %v, want %v", detected.Build, wantBuild)
	}
	wantTest := []string{
		"pnpm --dir apps/backend test",
		"pnpm --dir apps/frontend test",
	}
	if !reflect.DeepEqual(detected.Test, wantTest) {
		t.Fatalf("test = %v, want %v", detected.Test, wantTest)
	}
	if !reflect.DeepEqual(detected.Lint, []string{"pnpm --dir apps/backend run lint"}) {
		t.Fatalf("lint = %v", detected.Lint)
	}
}

func TestDetectRootPackageScripts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{
  "scripts": {"build": "tsc", "test": "vitest", "lint": "eslint ."}
}`)
	writeFile(t, filepath.Join(root, "package-lock.json"), "{}")
	writeFile(t, filepath.Join(root, "src", "index.ts"), "export const value = 1\n")

	detected, err := Detect(root, map[string]int{"typescript": 1})
	if err != nil {
		t.Fatal(err)
	}
	if detected.MultiPackage {
		t.Fatal("single package project detected as multiple packages")
	}
	if !reflect.DeepEqual(detected.Build, []string{"npm run build"}) ||
		!reflect.DeepEqual(detected.Test, []string{"npm test"}) ||
		!reflect.DeepEqual(detected.Lint, []string{"npm run lint"}) {
		t.Fatalf("commands = build:%v test:%v lint:%v", detected.Build, detected.Test, detected.Lint)
	}
}

func TestDetectMixedBackendFrontendPackages(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "backend", "go.mod"), "module example.com/backend\n\ngo 1.20\n")
	writeFile(t, filepath.Join(root, "backend", "main.go"), "package main\n")
	writeFile(t, filepath.Join(root, "frontend", "package.json"), `{"scripts":{"build":"next build"}}`)
	writeFile(t, filepath.Join(root, "frontend", "page.tsx"), "export default function Page() {}\n")

	detected, err := Detect(root, map[string]int{"go": 1, "typescript": 1})
	if err != nil {
		t.Fatal(err)
	}
	if !detected.MultiPackage {
		t.Fatal("mixed backend/frontend repository should be detected as multiple packages")
	}
	want := []string{"backend", "frontend"}
	if !reflect.DeepEqual(detected.Packages, want) {
		t.Fatalf("packages = %v, want %v", detected.Packages, want)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
