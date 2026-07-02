package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recordingRunner struct {
	failShowRef bool
	calls       []string
}

func (r *recordingRunner) Run(_ context.Context, _ string, command string, args ...string) error {
	r.calls = append(r.calls, command+" "+strings.Join(args, " "))
	if command == "git" && len(args) >= 1 && args[0] == "show-ref" && r.failShowRef {
		return os.ErrNotExist
	}
	return nil
}

func TestWorktreeManagerCreatesBranchWorktreeForTask(t *testing.T) {
	root := t.TempDir()
	runner := &recordingRunner{failShowRef: true}
	manager := WorktreeManager{Workspace: Open(root), Runner: runner}

	spec, err := manager.Prepare(context.Background(), WorktreeSpec{
		TaskID: "T-106",
		Branch: "thanos/T-106-cart",
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Path != filepath.Join(root, ".thanos", "worktrees", "T-106") {
		t.Fatalf("path = %s", spec.Path)
	}
	want := "git worktree add -b thanos/T-106-cart " + spec.Path + " HEAD"
	if got := runner.calls[len(runner.calls)-1]; got != want {
		t.Fatalf("last call = %q, want %q", got, want)
	}
}

func TestWorktreeManagerRefusesProtectedBranch(t *testing.T) {
	manager := WorktreeManager{Workspace: Open(t.TempDir()), Runner: &recordingRunner{}}
	_, err := manager.Prepare(context.Background(), WorktreeSpec{TaskID: "T-1", Branch: "main"})
	if err == nil || !strings.Contains(err.Error(), "protected branch") {
		t.Fatalf("expected protected branch error, got %v", err)
	}
}
