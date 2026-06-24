package orchestrator

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tinhtran/thanos/internal/codegraph"
	"github.com/tinhtran/thanos/internal/featuregraph"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/prompts"
	"github.com/tinhtran/thanos/internal/runner"
	"github.com/tinhtran/thanos/internal/state"
	"github.com/tinhtran/thanos/internal/ui"
	"github.com/tinhtran/thanos/internal/workspace"
)

type Orchestrator struct {
	Workspace *workspace.Workspace
	Runner    runner.Runner
	Stdout    io.Writer
	Stderr    io.Writer
}

func (o *Orchestrator) Run(ctx context.Context, featureID, runnerOverride string) error {
	feature, err := o.Workspace.LoadFeature(featureID)
	if err != nil {
		return err
	}
	features, err := o.Workspace.ListFeatures()
	if err != nil {
		return err
	}
	if err := featuregraph.Rebuild(o.Workspace.DotDir(), features); err != nil {
		return fmt.Errorf("refresh feature memory: %w", err)
	}
	for _, known := range features {
		if err := featuregraph.UpdateFromArtifacts(o.Workspace.DotDir(), known); err != nil {
			return fmt.Errorf("learn feature memory for %s: %w", known.ID, err)
		}
	}
	config, err := o.Workspace.ReadConfig()
	if err != nil {
		return err
	}
	if err := o.checkDependencies(feature); err != nil {
		return err
	}
	current, err := o.Workspace.EnsureRuntime(feature, config)
	if err != nil {
		return err
	}
	runnerName := runnerOverride
	if runnerName == "" {
		runnerName = feature.Runner
	}
	if runnerName == "" {
		runnerName = config.DefaultRunner
	}
	runnerConfig, ok := config.Runners[runnerName]
	if !ok {
		return fmt.Errorf("runner %q is not configured", runnerName)
	}
	current.Runner = runnerName

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		switch current.Phase {
		case model.PhaseInit:
			current, err = o.transition(current, model.PhaseDesign)
		case model.PhaseDesign:
			err = o.executeRole(ctx, feature, config, current, runnerConfig)
			if err == nil {
				err = requireArtifacts(o.Workspace, feature.ID, "task-brief.md", "acceptance-criteria.md", "test-strategy.yaml")
			}
			if err == nil {
				current, err = o.transition(current, model.PhaseDesignReview)
			}
		case model.PhaseDesignReview:
			err = o.executeRole(ctx, feature, config, current, runnerConfig)
			if err == nil {
				if verdictPass(o.Workspace, feature.ID, "design-review-report.md") {
					current, err = o.transition(current, model.PhaseCode)
				} else {
					current, err = o.transition(current, model.PhaseDesign)
				}
			}
		case model.PhaseCode, model.PhaseAmend:
			err = o.executeRole(ctx, feature, config, current, runnerConfig)
			if err == nil {
				report := filepath.Join("rounds", fmt.Sprintf("round-%d", current.Round), "coder-report.md")
				err = requireArtifacts(o.Workspace, feature.ID, report)
			}
			if err == nil {
				err = featuregraph.UpdateFromArtifacts(o.Workspace.DotDir(), feature)
			}
			if err == nil {
				current, err = o.transition(current, model.PhaseReview)
			}
		case model.PhaseReview:
			err = o.executeRole(ctx, feature, config, current, runnerConfig)
			if err == nil {
				report := filepath.Join("rounds", fmt.Sprintf("round-%d", current.Round), "review-report.md")
				if verdictPass(o.Workspace, feature.ID, report) {
					current, err = o.transition(current, model.PhaseTest)
				} else {
					current, err = o.transition(current, model.PhaseAmend)
				}
			}
		case model.PhaseTest:
			err = o.executeRole(ctx, feature, config, current, runnerConfig)
			if err == nil {
				report := filepath.Join("rounds", fmt.Sprintf("round-%d", current.Round), "test-report.md")
				if verdictPass(o.Workspace, feature.ID, report) {
					current, err = o.transition(current, model.PhaseDeepReview)
				} else {
					current, err = o.transition(current, model.PhaseAmend)
				}
			}
		case model.PhaseDeepReview:
			err = o.executeRole(ctx, feature, config, current, runnerConfig)
			if err == nil {
				report := filepath.Join("rounds", fmt.Sprintf("round-%d", current.Round), "deep-review-report.md")
				if verdictPass(o.Workspace, feature.ID, report) {
					current, err = o.transition(current, model.PhaseAccept)
				} else {
					current, err = o.transition(current, model.PhaseAmend)
				}
			}
		case model.PhaseAccept:
			err = o.executeRole(ctx, feature, config, current, runnerConfig)
			if err == nil {
				err = requireArtifacts(o.Workspace, feature.ID, "final-report.md", "retro-learnings.json", "feature-memory.json")
			}
			if err == nil {
				err = featuregraph.UpdateFromArtifacts(o.Workspace.DotDir(), feature)
			}
			if err == nil {
				if graph, scanErr := codegraph.Build(o.Workspace.Root); scanErr != nil {
					err = fmt.Errorf("refresh codebase graph: %w", scanErr)
				} else if scanErr = codegraph.Save(graph, o.Workspace.DotDir()); scanErr != nil {
					err = fmt.Errorf("save refreshed codebase graph: %w", scanErr)
				}
			}
			if err == nil {
				current, err = o.transition(current, model.PhasePending)
				if err == nil {
					feature.Status = "pending-review"
					err = o.Workspace.SaveFeature(feature)
					if err == nil {
						err = featuregraph.Sync(o.Workspace.DotDir(), feature)
					}
				}
			}
		case model.PhasePending:
			ui.Block(o.Stdout, ui.ExecLog(ui.ExecLogEntry{
				Type:    "read",
				Path:    filepath.Join(o.Workspace.DotDir(), feature.ID, "state.json"),
				Message: fmt.Sprintf("%s is pending human review. Run: thanos done %s", feature.ID, feature.ID),
				Status:  ui.Completed,
			}))
			return nil
		case model.PhaseDone:
			return nil
		case model.PhaseAttention, model.PhaseBlocked:
			return fmt.Errorf("%s stopped in %s: %s", feature.ID, current.Phase, current.Reason)
		default:
			return fmt.Errorf("unsupported phase %q", current.Phase)
		}
		if err != nil {
			current.Active = false
			current.Reason = err.Error()
			current.UpdatedAt = time.Now().UTC()
			_ = o.Workspace.WriteState(current)
			return err
		}
	}
}

func (o *Orchestrator) executeRole(ctx context.Context, feature model.Feature, config model.Config, current model.State, runnerConfig model.Runner) error {
	prompt, err := prompts.Render(current.Role, prompts.Data{
		Feature: feature, Config: config, State: current, Root: o.Workspace.Root,
	})
	if err != nil {
		return err
	}
	promptPath := filepath.Join(o.Workspace.DotDir(), "prompts", fmt.Sprintf("%s-%s-round-%d.md", feature.ID, current.Role, current.Round))
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return err
	}
	_ = o.Workspace.AppendEvent(model.Event{
		Type: "role-start", FeatureID: feature.ID, Timestamp: time.Now().UTC(),
		Phase: current.Phase, Role: current.Role, Round: current.Round,
	})
	command := strings.TrimSpace(runnerConfig.Command + " " + strings.Join(runnerConfig.Args, " "))
	started := time.Now()
	ui.Block(o.Stdout, ui.ExecLog(ui.ExecLogEntry{
		Type: "exec", Command: command, Workdir: o.Workspace.Root, Status: ui.Running,
		Message: fmt.Sprintf("%s %s (round %d) running", feature.ID, current.Role, current.Round),
	}))
	err = o.Runner.Run(ctx, o.Workspace.Root, runnerConfig, prompt, o.Stdout, o.Stderr)
	status := ui.Succeeded
	output := o.Stdout
	if err != nil {
		status = ui.Failed
		output = o.Stderr
	}
	ui.Block(output, ui.ExecLog(ui.ExecLogEntry{
		Status: status, DurationMs: time.Since(started).Milliseconds(),
	}))
	data := map[string]any{"success": err == nil}
	if err != nil {
		data["error"] = err.Error()
	}
	_ = o.Workspace.AppendEvent(model.Event{
		Type: "role-end", FeatureID: feature.ID, Timestamp: time.Now().UTC(),
		Phase: current.Phase, Role: current.Role, Round: current.Round, Data: data,
	})
	return err
}

func (o *Orchestrator) transition(current model.State, to model.Phase) (model.State, error) {
	next, err := state.Transition(current, to)
	if err != nil {
		return current, err
	}
	if err := o.Workspace.WriteState(next); err != nil {
		return current, err
	}
	_ = o.Workspace.AppendEvent(model.Event{
		Type: "transition", FeatureID: current.FeatureID, Timestamp: time.Now().UTC(),
		Phase: next.Phase, Role: next.Role, Round: next.Round,
		Data: map[string]any{"from": current.Phase, "to": next.Phase},
	})
	return next, nil
}

func (o *Orchestrator) checkDependencies(feature model.Feature) error {
	for _, dependency := range feature.Dependencies {
		dep, err := o.Workspace.LoadFeature(dependency)
		if err != nil {
			return fmt.Errorf("dependency %s: %w", dependency, err)
		}
		if dep.Status != "done" {
			return fmt.Errorf("dependency %s is %s, want done", dep.ID, dep.Status)
		}
	}
	return nil
}

func requireArtifacts(ws *workspace.Workspace, id string, names ...string) error {
	for _, name := range names {
		if !ws.ArtifactExists(id, name) {
			return fmt.Errorf("agent did not create required artifact .thanos/%s/%s", id, name)
		}
	}
	return nil
}

func verdictPass(ws *workspace.Workspace, id, name string) bool {
	content, err := ws.ReadArtifact(id, name)
	if err != nil {
		return false
	}
	upper := strings.ToUpper(content)
	index := strings.LastIndex(upper, "VERDICT")
	if index >= 0 {
		upper = upper[index:]
	}
	return strings.Contains(upper, "PASS") && !strings.Contains(upper, "FAIL")
}
