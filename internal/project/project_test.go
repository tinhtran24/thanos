package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestDetectFrameworkPHP(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string
		dirs      []string
		framework string
	}{
		{"composer require", map[string]string{"composer.json": `{"require":{"laravel/framework":"^11"}}`}, nil, "laravel"},
		{"composer require dev normalized key", map[string]string{"composer.json": `{"require-dev":{" Laravel/Framework ":"*" }}`}, nil, "laravel"},
		{"laravel markers", map[string]string{"artisan": "", "bootstrap/app.php": "<?php"}, nil, "laravel"},
		{"wordpress markers", nil, []string{"wp-admin", "wp-includes", "wp-content"}, "wordpress"},
		{"partial markers", map[string]string{"artisan": ""}, nil, ""},
		{"wrong marker types", map[string]string{"wp-admin": "", "wp-includes": "", "wp-content": ""}, nil, ""},
		{"similar dependency", map[string]string{"composer.json": `{"require":{"laravel/framework-extra":"*" }}`}, nil, ""},
		{"value is not evidence", map[string]string{"composer.json": `{"require":{"vendor/package":"laravel/framework" }}`}, nil, ""},
		{"ambiguous", map[string]string{"composer.json": `{"require":{"laravel/framework":"*" }}`}, []string{"wp-admin", "wp-includes", "wp-content"}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := frameworkRoot(t, test.files, test.dirs)
			assertFramework(t, root, "php", test.framework)
		})
	}
}

func TestDetectFrameworkMarkerTypes(t *testing.T) {
	root := frameworkRoot(t, map[string]string{
		"artisan":           "",
		"bootstrap/app.php": "",
	}, nil)
	if err := os.Remove(filepath.Join(root, "artisan")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "artisan"), 0o755); err != nil {
		t.Fatal(err)
	}
	assertFramework(t, root, "php", "")
}

func TestDetectFrameworkTypeScript(t *testing.T) {
	tests := []struct {
		name, manifest, framework string
	}{
		{"next dependency", `{"dependencies":{"next":"15"}}`, "nextjs"},
		{"nest dev dependency", `{"devDependencies":{"@nestjs/core":"11"}}`, "nestjs"},
		{"angular", `{"dependencies":{"@angular/core":"20"}}`, "angular"},
		{"nuxt", `{"dependencies":{"nuxt":"4"}}`, "nuxt"},
		{"case sensitive", `{"dependencies":{"Next":"15"}}`, ""},
		{"similar name", `{"dependencies":{"next-auth":"5"}}`, ""},
		{"script false positive", `{"scripts":{"build":"next build"}}`, ""},
		{"value false positive", `{"dependencies":{"other":"next"}}`, ""},
		{"same source ambiguity", `{"dependencies":{"next":"15","nuxt":"4"}}`, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertFramework(t, frameworkRoot(t, map[string]string{"package.json": test.manifest}, nil), "typescript", test.framework)
		})
	}
	t.Run("root only", func(t *testing.T) {
		root := frameworkRoot(t, map[string]string{
			"package-lock.json":     `{"packages":{"":{"dependencies":{"next":"15"}}}}`,
			"apps/web/package.json": `{"dependencies":{"next":"15"}}`,
			"src/index.ts":          `import next from "next"`,
		}, nil)
		assertFramework(t, root, "typescript", "")
	})
}

func TestDetectFrameworkGo(t *testing.T) {
	tests := []struct {
		name, module, framework string
	}{
		{"gin", "module example.com/app\n\ngo 1.20\nrequire github.com/gin-gonic/gin v1.10.0\n", "gin"},
		{"echo unversioned", "module example.com/app\n\ngo 1.20\nrequire github.com/labstack/echo v3.3.10+incompatible\n", "echo"},
		{"echo versioned grouped", "module example.com/app\n\ngo 1.20\nrequire (\n github.com/labstack/echo/v4 v4.13.0\n)\n", "echo"},
		{"nondecimal echo", "module example.com/app\n\ngo 1.20\nrequire github.com/labstack/echo/vx v1.0.0\n", ""},
		{"similar gin", "module example.com/app\n\ngo 1.20\nrequire github.com/gin-gonic/gin-contrib v1.0.0\n", ""},
		{"replace only", "module example.com/app\n\ngo 1.20\nreplace example.com/other => github.com/gin-gonic/gin v1.10.0\n", ""},
		{"comment only", "module example.com/app\n\ngo 1.20\n// require github.com/gin-gonic/gin v1.10.0\n", ""},
		{"ambiguity", "module example.com/app\n\ngo 1.20\nrequire (\n github.com/gin-gonic/gin v1.10.0\n github.com/labstack/echo/v4 v4.13.0\n)\n", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertFramework(t, frameworkRoot(t, map[string]string{"go.mod": test.module}, nil), "go", test.framework)
		})
	}
	t.Run("other files ignored", func(t *testing.T) {
		root := frameworkRoot(t, map[string]string{
			"go.mod":        "module example.com/app\n\ngo 1.20\n",
			"go.sum":        "github.com/gin-gonic/gin v1.10.0 h1:test\n",
			"main.go":       `package main; import _ "github.com/gin-gonic/gin"`,
			"nested/go.mod": "module nested\n\ngo 1.20\nrequire github.com/gin-gonic/gin v1.10.0\n",
		}, nil)
		assertFramework(t, root, "go", "")
	})
}

func TestDetectFrameworkPython(t *testing.T) {
	tests := []struct {
		name, manifest, requirement, framework string
	}{
		{"pep621 dependencies", "[project]\ndependencies = [\"Django[postgres]>=5; python_version > '3.10'\"]\n", "", "django"},
		{"pep621 optional", "[project.optional-dependencies]\ntest = [\"Flask~=3\"]\n", "", "flask"},
		{"poetry dependencies", "[tool.poetry.dependencies]\npython = \"^3.12\"\nFastAPI = \"^0.115\"\n", "", "fastapi"},
		{"poetry dev", "[tool.poetry.dev-dependencies]\nDjango = \"^5\"\n", "", "django"},
		{"poetry group", "[tool.poetry.group.api.dependencies]\nflask = \"^3\"\n", "", "flask"},
		{"pep735", "[dependency-groups]\napi = [\"fastapi>=0.115\", {include-group = \"lint\"}]\n", "", "fastapi"},
		{"requirements normalized", "", "fast_api==1\nFlask==3 # web\nDjango @ https://example.test/django.whl\n", ""},
		{"requirements single", "", "fastapi[standard]>=0.115\n", "fastapi"},
		{"parenthesized version", "", "Django(>=5)\n", "django"},
		{"version list and marker", "", "Flask >= 3, < 4 ; python_version >= '3.10'\n", "flask"},
		{"direct reference marker", "", "FastAPI @ https://example.test/fastapi.whl ; python_version >= '3.10'\n", "fastapi"},
		{"compound marker", "", "Django; python_version >= '3.10' and (sys_platform == 'linux' or os_name == \"posix\")\n", "django"},
		{"membership marker", "", "Flask; platform_system not in 'Windows,Darwin'\n", "flask"},
		{"direct reference URL semicolon", "", "FastAPI @ https://example.test/packages;v=1/fastapi.whl\n", "fastapi"},
		{"invalid trailing prose", "", "django garbage\n", ""},
		{"invalid extras", "", "flask[api,]\n", ""},
		{"invalid version", "", "fastapi>=\n", ""},
		{"invalid bare marker prose", "", "django ; garbage\n", ""},
		{"invalid bare marker missing operand", "", "flask ; python_version\n", ""},
		{"invalid bare marker missing value", "", "fastapi ; python_version >=\n", ""},
		{"invalid version marker", "", "django>=5 ; python_version >=\n", ""},
		{"invalid parenthesized marker", "", "flask(>=3) ; os_name ==\n", ""},
		{"invalid direct reference marker", "", "fastapi @ https://example.test/fastapi.whl ; sys_platform\n", ""},
		{"invalid marker variable", "", "django ; unknown_name == 'value'\n", ""},
		{"invalid marker connective", "", "flask ; python_version >= '3.10' and\n", ""},
		{"invalid marker parentheses", "", "fastapi ; (python_version >= '3.10'\n", ""},
		{"unsupported lines", "", "-r other.txt\n-e ./local\n./Django\ndjango/local\nhttps://example/Django\n# flask\n", ""},
		{"poetry alias value ignored", "[tool.poetry.dependencies]\nweb = { version = \"*\", package = \"django\" }\n", "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{}
			if test.manifest != "" {
				files["pyproject.toml"] = test.manifest
			}
			if test.requirement != "" {
				files["requirements-dev.txt"] = test.requirement
			}
			assertFramework(t, frameworkRoot(t, files, nil), "python", test.framework)
		})
	}
	t.Run("cross source ambiguity", func(t *testing.T) {
		root := frameworkRoot(t, map[string]string{
			"pyproject.toml":   "[project]\ndependencies=[\"django\"]\n",
			"requirements.txt": "flask\n",
		}, nil)
		assertFramework(t, root, "python", "")
	})
}

func TestDetectFrameworkRust(t *testing.T) {
	tests := []struct {
		name, manifest, framework string
	}{
		{"root dependency", "[dependencies]\nactix_web = \"4\"\n", "actix-web"},
		{"dev dependency", "[dev-dependencies]\naxum = \"0.8\"\n", "axum"},
		{"build dependency", "[build-dependencies]\nrocket = { version = \"0.5\" }\n", "rocket"},
		{"renamed dependency", "[dependencies]\nweb = { package = \"actix-web\", version = \"4\" }\n", "actix-web"},
		{"workspace dependency", "[workspace.dependencies]\naxum = \"0.8\"\n", "axum"},
		{"workspace inherited key", "[dependencies]\nrocket = { workspace = true }\n", "rocket"},
		{"target dependency", "[target.'cfg(unix)'.dependencies]\naxum = \"0.8\"\n", "axum"},
		{"value ignored", "[dependencies]\nweb = \"actix-web\"\n", ""},
		{"boolean value ignored", "[dependencies]\naxum = true\n", ""},
		{"numeric value ignored", "[dependencies]\nrocket = 1\n", ""},
		{"array value ignored", "[dependencies]\nactix_web = [\"4\"]\n", ""},
		{"ambiguity", "[dependencies]\naxum = \"0.8\"\nrocket = \"0.5\"\n", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertFramework(t, frameworkRoot(t, map[string]string{"Cargo.toml": test.manifest}, nil), "rust", test.framework)
		})
	}
	t.Run("root only", func(t *testing.T) {
		root := frameworkRoot(t, map[string]string{
			"Cargo.lock":        `name = "axum"`,
			"src/main.rs":       `use axum::Router;`,
			"nested/Cargo.toml": "[dependencies]\naxum=\"0.8\"\n",
		}, nil)
		assertFramework(t, root, "rust", "")
	})
}

func TestDetectFrameworkLanguageSelection(t *testing.T) {
	root := frameworkRoot(t, map[string]string{"package.json": `{"dependencies":{"next":"15"}}`}, nil)
	for _, language := range []string{"typescript", " TypeScript ", "TYPESCRIPT"} {
		assertFramework(t, root, language, "nextjs")
	}
	assertFramework(t, root, "javascript", "")
	assertFramework(t, root, "", "")
}

func TestDetectFrameworkAmbiguity(t *testing.T) {
	root := frameworkRoot(t, map[string]string{
		"requirements.txt":     "django\n",
		"requirements-dev.txt": "flask\n",
	}, nil)
	assertFramework(t, root, "python", "")
}

func TestDetectFrameworkMalformedEvidence(t *testing.T) {
	tests := []struct {
		language, filename, malformed, validFile, valid, framework string
		extraFiles                                                 map[string]string
	}{
		{"php", "composer.json", "{", "artisan", "", "laravel", map[string]string{"bootstrap/app.php": ""}},
		{"typescript", "package.json", "{", "", "", "", nil},
		{"go", "go.mod", "not a module", "", "", "", nil},
		{"python", "pyproject.toml", "[", "requirements.txt", "django\n", "django", nil},
		{"rust", "Cargo.toml", "[", "", "", "", nil},
	}
	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			files := map[string]string{test.filename: test.malformed}
			if test.validFile != "" {
				files[test.validFile] = test.valid
			}
			for path, content := range test.extraFiles {
				files[path] = content
			}
			assertFramework(t, frameworkRoot(t, files, nil), test.language, test.framework)
		})
	}
}

func TestDetectFrameworkFilesystemErrors(t *testing.T) {
	t.Run("manifest read", func(t *testing.T) {
		original := readFrameworkFile
		readFrameworkFile = func(string) ([]byte, error) { return nil, fs.ErrPermission }
		defer func() { readFrameworkFile = original }()
		_, err := DetectFramework(t.TempDir(), "typescript")
		if err == nil || !strings.Contains(err.Error(), "package.json") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("root directory read", func(t *testing.T) {
		original := readFrameworkDir
		readFrameworkDir = func(string) ([]os.DirEntry, error) { return nil, fs.ErrPermission }
		defer func() { readFrameworkDir = original }()
		_, err := DetectFramework(t.TempDir(), "python")
		if err == nil || !strings.Contains(err.Error(), "framework directory") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("marker stat", func(t *testing.T) {
		original := statFrameworkPath
		statFrameworkPath = func(string) (os.FileInfo, error) { return nil, fs.ErrPermission }
		defer func() { statFrameworkPath = original }()
		_, err := DetectFramework(t.TempDir(), "php")
		if err == nil || !strings.Contains(err.Error(), "framework marker") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("not found is ignored", func(t *testing.T) {
		original := readFrameworkFile
		readFrameworkFile = func(string) ([]byte, error) { return nil, os.ErrNotExist }
		defer func() { readFrameworkFile = original }()
		got, err := DetectFramework(t.TempDir(), "typescript")
		if err != nil || got != "" {
			t.Fatalf("framework = %q, error = %v", got, err)
		}
	})
}

func frameworkRoot(t *testing.T, files map[string]string, dirs []string) string {
	t.Helper()
	root := t.TempDir()
	for path, content := range files {
		writeFile(t, filepath.Join(root, path), content)
	}
	for _, path := range dirs {
		if err := os.MkdirAll(filepath.Join(root, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func assertFramework(t *testing.T, root, language, want string) {
	t.Helper()
	got, err := DetectFramework(root, language)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("DetectFramework(%q) = %q, want %q", language, got, want)
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
