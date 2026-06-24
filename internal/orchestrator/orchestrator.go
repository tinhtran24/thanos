package orchestrator

import (
	"context"
	"errors"
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
	codingStyle := o.readCodingStyle()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		// Resolve the active chunk + path prefix for the current EC.
		plan, _ := o.Workspace.ReadPlan(feature.ID)
		chunks := plan.ActiveChunks()
		ec := ecContext{prefix: ecPrefix(current), coding: codingStyle, plan: chunks, chunk: currentChunk(chunks, current.ECIndex)}

		switch current.Phase {
		case model.PhaseInit:
			current, err = o.transition(current, model.PhasePlan)
		case model.PhasePlan:
			err = o.executeRole(ctx, feature, config, current, runnerConfig, ec)
			if err == nil {
				current, err = o.beginExecutionPlan(feature, current)
			}
		case model.PhaseDesign:
			o.ensureChunkDir(feature.ID, current)
			err = o.executeRole(ctx, feature, config, current, runnerConfig, ec)
			if err == nil {
				err = requireArtifacts(o.Workspace, feature.ID, ecJoin(current, "task-brief.md"), ecJoin(current, "acceptance-criteria.md"), ecJoin(current, "test-strategy.yaml"))
			}
			if err == nil {
				current, err = o.transition(current, model.PhaseDesignReview)
			}
		case model.PhaseDesignReview:
			err = o.executeRole(ctx, feature, config, current, runnerConfig, ec)
			if err == nil {
				if verdictPass(o.Workspace, feature.ID, ecJoin(current, "design-review-report.md")) {
					current, err = o.transition(current, model.PhaseCode)
				} else {
					current, err = o.transition(current, model.PhaseDesign)
				}
			}
		case model.PhaseCode, model.PhaseAmend:
			o.ensureRoundDir(feature.ID, current)
			err = o.executeRole(ctx, feature, config, current, runnerConfig, ec)
			if err == nil {
				report := ecJoin(current, "rounds", fmt.Sprintf("round-%d", current.Round), "coder-report.md")
				err = requireArtifacts(o.Workspace, feature.ID, report)
			}
			if err == nil {
				err = featuregraph.UpdateFromArtifacts(o.Workspace.DotDir(), feature)
			}
			if err == nil {
				current, err = o.transition(current, model.PhaseReview)
			}
		case model.PhaseReview:
			err = o.executeRole(ctx, feature, config, current, runnerConfig, ec)
			if err == nil {
				report := ecJoin(current, "rounds", fmt.Sprintf("round-%d", current.Round), "review-report.md")
				if verdictPass(o.Workspace, feature.ID, report) {
					current, err = o.transition(current, model.PhaseTest)
				} else {
					current, err = o.transition(current, model.PhaseAmend)
				}
			}
		case model.PhaseTest:
			err = o.executeRole(ctx, feature, config, current, runnerConfig, ec)
			if err == nil {
				report := ecJoin(current, "rounds", fmt.Sprintf("round-%d", current.Round), "test-report.md")
				if verdictPass(o.Workspace, feature.ID, report) {
					current, err = o.transition(current, model.PhaseDeepReview)
				} else {
					current, err = o.transition(current, model.PhaseAmend)
				}
			}
		case model.PhaseDeepReview:
			err = o.executeRole(ctx, feature, config, current, runnerConfig, ec)
			if err == nil {
				report := ecJoin(current, "rounds", fmt.Sprintf("round-%d", current.Round), "deep-review-report.md")
				if verdictPass(o.Workspace, feature.ID, report) {
					current, err = o.finishChunkOrFeature(feature, current)
				} else {
					current, err = o.transition(current, model.PhaseAmend)
				}
			}
		case model.PhaseAccept:
			err = o.executeRole(ctx, feature, config, current, runnerConfig, ec)
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
			if errors.Is(err, errClarifyPending) {
				// The role asked a question instead of finishing. Pause cleanly
				// (keep the phase so the role re-runs once answered) and stop.
				current.Active = false
				current.Reason = "needs clarification"
				current.UpdatedAt = time.Now().UTC()
				_ = o.Workspace.WriteState(current)
				_ = o.Workspace.AppendEvent(model.Event{
					Type: "clarify", FeatureID: feature.ID, Timestamp: time.Now().UTC(),
					Phase: current.Phase, Role: current.Role, Round: current.Round,
				})
				ui.Block(o.Stdout, ui.ExecLog(ui.ExecLogEntry{
					Type: "read", Path: filepath.Join(o.Workspace.DotDir(), feature.ID, ecJoin(current, "clarify.json")),
					Message: fmt.Sprintf("%s needs clarification. Answer: thanos clarify %s \"<answer>\"", feature.ID, feature.ID),
					Status:  ui.Warned,
				}))
				return nil
			}
			current.Active = false
			current.Reason = err.Error()
			current.UpdatedAt = time.Now().UTC()
			_ = o.Workspace.WriteState(current)
			return err
		}
	}
}

// ecContext carries the per-execution-chunk scope handed to a role.
type ecContext struct {
	chunk  *model.ExecutionChunk
	prefix string                 // artifact path prefix ("" or "ec-<i>/")
	coding string                 // project coding-style doc
	plan   []model.ExecutionChunk // active chunks
}

func (o *Orchestrator) executeRole(ctx context.Context, feature model.Feature, config model.Config, current model.State, runnerConfig model.Runner, ec ecContext) error {
	prompt, err := prompts.Render(current.Role, prompts.Data{
		Feature: feature, Config: config, State: current, Root: o.Workspace.Root,
		ExecutionChunk: ec.chunk, ECPrefix: ec.prefix, CodingStyle: ec.coding, Plan: ec.plan,
	})
	if err != nil {
		return err
	}
	promptPath := filepath.Join(o.Workspace.DotDir(), "prompts", fmt.Sprintf("%s-ec%d-%s-round-%d.md", feature.ID, current.ECIndex, current.Role, current.Round))
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return err
	}
	_ = o.Workspace.AppendEvent(model.Event{
		Type: "role-start", FeatureID: feature.ID, Timestamp: time.Now().UTC(),
		Phase: current.Phase, Role: current.Role, Round: current.Round,
	})
	command := strings.TrimSpace(runnerConfig.Command + " " + strings.Join(runnerConfig.Args, " "))
	started := time.Now()
	label := fmt.Sprintf("%s %s (round %d) running", feature.ID, current.Role, current.Round)
	if ec.chunk != nil && current.ECTotal > 1 {
		label = fmt.Sprintf("%s EC-%d/%d %s (round %d) running", feature.ID, current.ECIndex, current.ECTotal, current.Role, current.Round)
	}
	ui.Block(o.Stdout, ui.ExecLog(ui.ExecLogEntry{
		Type: "exec", Command: command, Workdir: o.Workspace.Root, Status: ui.Running,
		Message: label,
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
	if err == nil && o.clarifyPending(feature.ID, current) {
		return errClarifyPending
	}
	return err
}

// errClarifyPending signals that a role wrote a clarify.json question instead of
// finishing; the run pauses until `thanos clarify` records an answer.
var errClarifyPending = errors.New("needs clarification")

// clarifyPending reports whether an unanswered clarify.json exists for the
// current chunk (a clarify.json newer than its answer, or with no answer yet).
func (o *Orchestrator) clarifyPending(id string, current model.State) bool {
	question := filepath.Join(o.Workspace.RuntimeDir(id), ecJoin(current, "clarify.json"))
	qi, err := os.Stat(question)
	if err != nil {
		return false
	}
	answer := filepath.Join(o.Workspace.RuntimeDir(id), ecJoin(current, "clarify-answer.md"))
	ai, err := os.Stat(answer)
	if err != nil {
		return true // question with no answer
	}
	return qi.ModTime().After(ai.ModTime()) // re-asked after the last answer
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

// --- execution-chunk helpers -------------------------------------------------

// ecDir is the artifact subdirectory for the current chunk: "" for a single
// implicit chunk (feature-root layout, back-compatible) or "ec-<i>" otherwise.
func ecDir(s model.State) string {
	if s.ECTotal > 1 && s.ECIndex >= 1 {
		return fmt.Sprintf("ec-%d", s.ECIndex)
	}
	return ""
}

func ecPrefix(s model.State) string {
	if d := ecDir(s); d != "" {
		return d + "/"
	}
	return ""
}

// ecJoin builds an artifact path scoped to the current chunk.
func ecJoin(s model.State, parts ...string) string {
	return filepath.Join(append([]string{ecDir(s)}, parts...)...)
}

func currentChunk(chunks []model.ExecutionChunk, idx int) *model.ExecutionChunk {
	if idx >= 1 && idx <= len(chunks) {
		c := chunks[idx-1]
		return &c
	}
	return nil
}

func (o *Orchestrator) ensureChunkDir(id string, s model.State) {
	if d := ecDir(s); d != "" {
		_ = o.Workspace.EnsureArtifactDir(id, d)
	}
}

func (o *Orchestrator) ensureRoundDir(id string, s model.State) {
	_ = o.Workspace.EnsureArtifactDir(id, ecJoin(s, "rounds", fmt.Sprintf("round-%d", s.Round)))
}

func (o *Orchestrator) readCodingStyle() string {
	data, err := os.ReadFile(filepath.Join(o.Workspace.DotDir(), "coding-style.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

func (o *Orchestrator) markChunkStatus(id, chunkID, status string) {
	plan, err := o.Workspace.ReadPlan(id)
	if err != nil {
		return
	}
	changed := false
	for i := range plan.Chunks {
		if plan.Chunks[i].ID == chunkID {
			plan.Chunks[i].Status = status
			changed = true
		}
	}
	if changed {
		_ = o.Workspace.WritePlan(id, plan)
	}
}

// beginExecutionPlan loads the planner's chunks (synthesizing a single implicit
// chunk if none were written) and starts the first chunk's design phase.
func (o *Orchestrator) beginExecutionPlan(feature model.Feature, current model.State) (model.State, error) {
	plan, err := o.Workspace.ReadPlan(feature.ID)
	if err != nil {
		return current, err
	}
	active := plan.ActiveChunks()
	if len(active) == 0 {
		active = []model.ExecutionChunk{{
			Index: 1, ID: feature.ID + "-ec1", Title: feature.Title,
			Description: feature.Description, Acceptance: feature.Acceptance,
			Scope: feature.Scope, Status: "todo",
		}}
		plan.Chunks = active
		if err := o.Workspace.WritePlan(feature.ID, plan); err != nil {
			return current, err
		}
	}
	current.ECTotal = len(active)
	current.ECIndex = 1
	current.ECID = active[0].ID
	o.markChunkStatus(feature.ID, current.ECID, "active")
	_ = o.Workspace.AppendEvent(model.Event{
		Type: "ec-start", FeatureID: feature.ID, Timestamp: time.Now().UTC(),
		Data: map[string]any{"ec_index": 1, "ec_total": current.ECTotal, "ec_id": current.ECID, "title": active[0].Title},
	})
	return o.transition(current, model.PhaseDesign)
}

// finishChunkOrFeature marks the current chunk done and either advances to the
// next chunk's design phase or, for the last chunk, moves to feature accept.
func (o *Orchestrator) finishChunkOrFeature(feature model.Feature, current model.State) (model.State, error) {
	o.markChunkStatus(feature.ID, current.ECID, "done")
	_ = o.Workspace.AppendEvent(model.Event{
		Type: "ec-end", FeatureID: feature.ID, Timestamp: time.Now().UTC(),
		Data: map[string]any{"ec_index": current.ECIndex, "ec_id": current.ECID},
	})
	if current.ECIndex < current.ECTotal {
		plan, _ := o.Workspace.ReadPlan(feature.ID)
		active := plan.ActiveChunks()
		current.ECIndex++
		if current.ECIndex-1 < len(active) {
			current.ECID = active[current.ECIndex-1].ID
			o.markChunkStatus(feature.ID, current.ECID, "active")
			_ = o.Workspace.AppendEvent(model.Event{
				Type: "ec-start", FeatureID: feature.ID, Timestamp: time.Now().UTC(),
				Data: map[string]any{"ec_index": current.ECIndex, "ec_total": current.ECTotal, "ec_id": current.ECID, "title": active[current.ECIndex-1].Title},
			})
		}
		return o.transition(current, model.PhaseDesign)
	}
	return o.transition(current, model.PhaseAccept)
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
