package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPlan(t *testing.T) {
	got := Plan([]PlanStep{
		{Title: "Check git status", State: Done},
		{Title: "Apply patch", State: Done},
		{Title: "Run tests", State: Pending},
	})
	want := "✓ Check git status\n✓ Apply patch\n• Run tests"
	if got != want {
		t.Fatalf("Plan() = %q, want %q", got, want)
	}
}

func TestExecLogCommand(t *testing.T) {
	got := ExecLog(ExecLogEntry{
		Type:       "exec",
		Command:    "/bin/zsh -lc 'git status --short'",
		Workdir:    "/Users/tinhtran/Coding/thanos",
		Status:     Succeeded,
		DurationMs: 0,
	})
	want := "exec\n/bin/zsh -lc 'git status --short' in /Users/tinhtran/Coding/thanos\n✓ succeeded in 0ms:"
	if got != want {
		t.Fatalf("ExecLog() = %q, want %q", got, want)
	}
}

func TestExecLogMessage(t *testing.T) {
	got := ExecLog(ExecLogEntry{
		Type:    "apply patch",
		Message: "patch: completed",
		Status:  Completed,
	})
	want := "apply patch\n✓ patch: completed"
	if got != want {
		t.Fatalf("ExecLog() = %q, want %q", got, want)
	}
}

func TestLogger(t *testing.T) {
	var output strings.Builder
	Success(&output, "Build succeeded")
	Error(&output, "Deployment failed")
	Warning(&output, "Configuration missing")
	Info(&output, "Loading project")
	Debug(&output, "using cached metadata")

	want := "✓ Build succeeded\n" +
		"✗ Deployment failed\n" +
		"⚠ Configuration missing\n" +
		"ℹ Loading project\n" +
		"• using cached metadata\n"
	if output.String() != want {
		t.Fatalf("logger output = %q, want %q", output.String(), want)
	}
}

func TestStatusRenderers(t *testing.T) {
	section := Section("Project Information", []Field{
		{Label: "Name", Value: "thanos"},
		{Label: "Version", Value: "1.0.0"},
	})
	for _, value := range []string{"Project Information", "Name:", "thanos", "Version:", "1.0.0"} {
		if !strings.Contains(section, value) {
			t.Fatalf("section missing %q: %q", value, section)
		}
	}

	table := Table(
		[]string{"TARGET", "STATUS"},
		[][]string{{"darwin-arm64", "Success"}, {"linux-amd64", "Pending"}},
	)
	for _, value := range []string{"TARGET", "STATUS", "darwin-arm64", "linux-amd64"} {
		if !strings.Contains(table, value) {
			t.Fatalf("table missing %q: %q", value, table)
		}
	}

	panel := Panel("Release Published", []Field{
		{Label: "Version", Value: "v1.2.0"},
		{Label: "Assets", Value: "12"},
		{Label: "Status", Value: "Success"},
	}, true)
	if !strings.Contains(panel, "Release Published") || !strings.Contains(panel, "v1.2.0") {
		t.Fatalf("panel = %q", panel)
	}

	failure := Failure(ErrorDetails{
		Title:      "Git Push Failed",
		Reason:     "remote rejected update",
		Suggestion: "Update the local branch.",
		Command:    "git pull --rebase",
	})
	for _, value := range []string{"Git Push Failed", "Reason:", "remote rejected update", "Suggestion:", "git pull --rebase"} {
		if !strings.Contains(failure, value) {
			t.Fatalf("failure missing %q: %q", value, failure)
		}
	}
}

func TestProgress(t *testing.T) {
	rendered := Progress("Uploading artifacts", 0.65, 25)
	if !strings.Contains(rendered, "Uploading artifacts") || !strings.Contains(rendered, "65%") {
		t.Fatalf("progress = %q", rendered)
	}
}

func TestSpinnerFallsBackForNonTerminalOutput(t *testing.T) {
	var output strings.Builder
	if err := Build(context.Background(), &output, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); !strings.Contains(got, "Building binaries...") || !strings.Contains(got, "completed") {
		t.Fatalf("spinner output = %q", got)
	}

	output.Reset()
	wantErr := errors.New("build failed")
	if err := Build(context.Background(), &output, func() error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("spinner error = %v", err)
	}
	if !strings.Contains(output.String(), "failed") {
		t.Fatalf("spinner failure output = %q", output.String())
	}
}
