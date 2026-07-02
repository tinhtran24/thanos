package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type CommandRunner interface {
	Run(ctx context.Context, dir string, command string, args ...string) error
}

type WorktreeSpec struct {
	TaskID string
	Branch string
	Path   string
	Base   string
}

type WorktreeManager struct {
	Workspace *Workspace
	Runner    CommandRunner
}

func (m WorktreeManager) Prepare(ctx context.Context, spec WorktreeSpec) (WorktreeSpec, error) {
	if m.Workspace == nil {
		return spec, fmt.Errorf("workspace is required")
	}
	if m.Runner == nil {
		return spec, fmt.Errorf("command runner is required")
	}
	if strings.TrimSpace(spec.TaskID) == "" {
		return spec, fmt.Errorf("task id is required")
	}
	if strings.TrimSpace(spec.Branch) == "" {
		return spec, fmt.Errorf("branch is required")
	}
	if isMainBranch(spec.Branch) {
		return spec, fmt.Errorf("refusing to use protected branch %q for task worktree", spec.Branch)
	}
	if strings.TrimSpace(spec.Base) == "" {
		spec.Base = "HEAD"
	}
	if strings.TrimSpace(spec.Path) == "" {
		spec.Path = m.Workspace.TaskWorktreePath(spec.TaskID)
	}
	if err := os.MkdirAll(filepath.Dir(spec.Path), 0o755); err != nil {
		return spec, err
	}
	if _, err := os.Stat(spec.Path); err == nil {
		return spec, nil
	} else if !os.IsNotExist(err) {
		return spec, err
	}
	if err := m.Runner.Run(ctx, m.Workspace.Root, "git", "rev-parse", "--is-inside-work-tree"); err != nil {
		return spec, fmt.Errorf("workspace is not a git repository: %w", err)
	}
	if err := m.Runner.Run(ctx, m.Workspace.Root, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+spec.Branch); err == nil {
		return spec, m.Runner.Run(ctx, m.Workspace.Root, "git", "worktree", "add", spec.Path, spec.Branch)
	}
	return spec, m.Runner.Run(ctx, m.Workspace.Root, "git", "worktree", "add", "-b", spec.Branch, spec.Path, spec.Base)
}

func isMainBranch(branch string) bool {
	switch strings.TrimSpace(branch) {
	case "main", "master", "trunk":
		return true
	default:
		return false
	}
}
