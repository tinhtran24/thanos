package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinhtran/thanos/internal/model"
)

func TestFeatureRoundTripAndShortID(t *testing.T) {
	ws := Open(t.TempDir())
	if err := ws.Init(model.Config{Project: model.Project{Name: "test"}}); err != nil {
		t.Fatal(err)
	}
	feature := model.Feature{ID: "F001-user-auth", Title: "User auth", Status: "todo"}
	if err := ws.SaveFeature(feature); err != nil {
		t.Fatal(err)
	}
	got, err := ws.LoadFeature("F001")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != feature.Title {
		t.Fatalf("title = %q", got.Title)
	}
}

func TestEnsureRuntimeMigratesLatestLegacyRoundArtifacts(t *testing.T) {
	ws := Open(t.TempDir())
	config := model.Config{Project: model.Project{Name: "test"}}
	if err := ws.Init(config); err != nil {
		t.Fatal(err)
	}
	feature := model.Feature{ID: "F001-user-auth", Title: "User auth", Status: "todo"}
	if err := ws.SaveFeature(feature); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ws.RuntimeDir(feature.ID), "rounds", "round-2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(ws.RuntimeDir(feature.ID), "rounds", "round-2", "coder-report.md"),
		[]byte("latest implementation evidence"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := ws.WriteState(model.State{FeatureID: feature.ID, Phase: model.PhaseReview}); err != nil {
		t.Fatal(err)
	}

	if _, err := ws.EnsureRuntime(feature, config); err != nil {
		t.Fatal(err)
	}
	got, err := ws.ReadArtifact(feature.ID, "implementation-note.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "latest implementation evidence" {
		t.Fatalf("migrated artifact = %q", got)
	}
}

func TestValidateCompletionEvidence(t *testing.T) {
	ws := Open(t.TempDir())
	if err := ws.Init(model.Config{Project: model.Project{Name: "test"}}); err != nil {
		t.Fatal(err)
	}
	feature := model.Feature{ID: "F001-user-auth", Title: "User auth", Status: "ready-for-review"}
	if err := ws.SaveFeature(feature); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"implementation-note.md": "# Implementation Note",
		"review-report.md":       "## Decision\nAPPROVED\n## Verdict\nPASS",
		"test-report.md":         "## Verdict\nPASS",
	} {
		if err := ws.WriteArtifact(feature.ID, name, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := ws.ValidateCompletionEvidence(feature, model.State{Phase: model.PhaseReview}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCompletionEvidenceExplainsPendingReview(t *testing.T) {
	ws := Open(t.TempDir())
	if err := ws.Init(model.Config{Project: model.Project{Name: "test"}}); err != nil {
		t.Fatal(err)
	}
	feature := model.Feature{ID: "F001-user-auth", Title: "User auth", Status: "ready-for-review"}
	for name, content := range map[string]string{
		"implementation-note.md": "# Implementation Note",
		"review-report.md":       "## Decision\nPENDING INDEPENDENT REVIEW\n## Verdict\nNOT RUN",
		"test-report.md":         "## Verdict\nPASS",
	} {
		if err := ws.WriteArtifact(feature.ID, name, content); err != nil {
			t.Fatal(err)
		}
	}
	err := ws.ValidateCompletionEvidence(feature, model.State{Phase: model.PhaseReview})
	if err == nil || !strings.Contains(err.Error(), "review is not approved") {
		t.Fatalf("error = %v", err)
	}
}
