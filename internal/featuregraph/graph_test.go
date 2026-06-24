package featuregraph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinhtran/thanos/internal/codegraph"
	"github.com/tinhtran/thanos/internal/model"
)

func TestBugfixResolvesParentRulesAndImpactPaths(t *testing.T) {
	root := t.TempDir()
	dotDir := filepath.Join(root, ".thanos")
	source := `package auth
func ValidatePassword(value string) bool { return len(value) >= 12 }
`
	caller := `package auth
func Register(password string) bool { return ValidatePassword(password) }
`
	if err := os.WriteFile(filepath.Join(root, "validator.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "registration.go"), []byte(caller), 0o644); err != nil {
		t.Fatal(err)
	}
	code, err := codegraph.Build(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := codegraph.Save(code, dotDir); err != nil {
		t.Fatal(err)
	}

	parent := model.Feature{
		ID: "F001-password-policy", Title: "Password policy", Type: "feature", Status: "done",
		Rules: []string{"Passwords require at least 12 characters"},
	}
	bugfix := model.Feature{
		ID: "F002-password-message", Title: "Fix password message", Type: "bugfix",
		Parent: parent.ID, Status: "todo",
	}
	if err := Rebuild(dotDir, []model.Feature{parent, bugfix}); err != nil {
		t.Fatal(err)
	}
	runtime := filepath.Join(dotDir, parent.ID)
	if err := os.MkdirAll(runtime, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime, "feature-memory.json"), []byte(`{
  "business_rules": ["Registration and reset use the same password policy"],
  "architectural_decisions": ["Password policy remains centralized in ValidatePassword"],
  "affected_paths": ["validator.go"]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UpdateFromArtifacts(dotDir, parent); err != nil {
		t.Fatal(err)
	}
	parent.Status = "pending-review"
	if err := Sync(dotDir, parent); err != nil {
		t.Fatal(err)
	}

	context, err := Resolve(dotDir, bugfix.ID)
	if err != nil {
		t.Fatal(err)
	}
	memory := ContextMarkdown(dotDir, bugfix.ID)
	for _, want := range []string{
		"Passwords require at least 12 characters",
		"Registration and reset use the same password policy",
		"Password policy remains centralized",
		"validator.go",
		"registration.go",
	} {
		if !strings.Contains(memory, want) {
			t.Fatalf("memory missing %q:\n%s", want, memory)
		}
	}
	if len(context.Features) != 2 {
		t.Fatalf("features = %+v", context.Features)
	}
}

func TestUpdateFromCoderReportLearnsChangedPaths(t *testing.T) {
	dotDir := filepath.Join(t.TempDir(), ".thanos")
	feature := model.Feature{ID: "F001-login", Title: "Login", Status: "todo"}
	if err := Rebuild(dotDir, []model.Feature{feature}); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(dotDir, feature.ID, "rounds", "round-1", "coder-report.md")
	if err := os.MkdirAll(filepath.Dir(report), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# Coder Report\n## Files Changed\n- `internal/auth/login.go` — implementation\n- web/login.test.ts - tests\n## Verification\n"
	if err := os.WriteFile(report, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UpdateFromArtifacts(dotDir, feature); err != nil {
		t.Fatal(err)
	}
	context, err := Resolve(dotDir, feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(context.Paths) != 2 {
		t.Fatalf("paths = %+v", context.Paths)
	}
}
