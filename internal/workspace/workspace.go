package workspace

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tinhtran/thanos/internal/model"
	"gopkg.in/yaml.v3"
)

const DirName = ".thanos"

type Workspace struct {
	Root string
}

func Open(root string) *Workspace {
	return &Workspace{Root: root}
}

func (w *Workspace) DotDir() string { return filepath.Join(w.Root, DirName) }

func (w *Workspace) Init(config model.Config) error {
	if _, err := os.Stat(w.ConfigPath()); err == nil {
		return fmt.Errorf("%s already exists", w.ConfigPath())
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, dir := range []string{w.DotDir(), filepath.Join(w.DotDir(), "features"), filepath.Join(w.DotDir(), "prompts")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return writeJSON(w.ConfigPath(), config)
}

func (w *Workspace) ConfigPath() string { return filepath.Join(w.DotDir(), "settings.json") }

func (w *Workspace) ReadConfig() (model.Config, error) {
	var config model.Config
	if err := readJSON(w.ConfigPath(), &config); err != nil {
		return config, err
	}
	return config, nil
}

func (w *Workspace) WriteConfig(config model.Config) error {
	return writeJSON(w.ConfigPath(), config)
}

func (w *Workspace) FeaturePath(id string) string {
	return filepath.Join(w.DotDir(), "features", id+".yaml")
}

func (w *Workspace) SaveFeature(feature model.Feature) error {
	data, err := yaml.Marshal(feature)
	if err != nil {
		return err
	}
	return os.WriteFile(w.FeaturePath(feature.ID), data, 0o644)
}

func (w *Workspace) LoadFeature(id string) (model.Feature, error) {
	var feature model.Feature
	path, err := w.resolveFeaturePath(id)
	if err != nil {
		return feature, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return feature, err
	}
	if err := yaml.Unmarshal(data, &feature); err != nil {
		return feature, err
	}
	return feature, nil
}

func (w *Workspace) ListFeatures() ([]model.Feature, error) {
	paths, err := filepath.Glob(filepath.Join(w.DotDir(), "features", "*.yaml"))
	if err != nil {
		return nil, err
	}
	features := make([]model.Feature, 0, len(paths))
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}
		var feature model.Feature
		if err := yaml.Unmarshal(data, &feature); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		features = append(features, feature)
	}
	sort.Slice(features, func(i, j int) bool { return features[i].ID < features[j].ID })
	return features, nil
}

func (w *Workspace) NextFeatureID(title string) (string, error) {
	features, err := w.ListFeatures()
	if err != nil {
		return "", err
	}
	slug := slugify(title)
	return fmt.Sprintf("F%03d-%s", len(features)+1, slug), nil
}

func (w *Workspace) RuntimeDir(id string) string { return filepath.Join(w.DotDir(), id) }
func (w *Workspace) StatePath(id string) string  { return filepath.Join(w.RuntimeDir(id), "state.json") }

func (w *Workspace) EnsureRuntime(feature model.Feature, config model.Config) (model.State, error) {
	var current model.State
	if err := readJSON(w.StatePath(feature.ID), &current); err == nil {
		return current, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return current, err
	}
	if err := os.MkdirAll(filepath.Join(w.RuntimeDir(feature.ID), "rounds"), 0o755); err != nil {
		return current, err
	}
	maxRounds := feature.MaxRounds
	if maxRounds == 0 {
		maxRounds = config.MaxRounds
	}
	now := time.Now().UTC()
	current = model.State{
		FeatureID: feature.ID,
		Phase:     model.PhaseInit,
		MaxRounds: maxRounds,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return current, w.WriteState(current)
}

func (w *Workspace) ReadState(id string) (model.State, error) {
	var current model.State
	err := readJSON(w.StatePath(id), &current)
	return current, err
}

func (w *Workspace) WriteState(current model.State) error {
	return writeJSON(w.StatePath(current.FeatureID), current)
}

func (w *Workspace) AppendEvent(event model.Event) error {
	path := filepath.Join(w.RuntimeDir(event.FeatureID), "events.jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(event)
}

func (w *Workspace) WriteArtifact(id, name, content string) error {
	path := filepath.Join(w.RuntimeDir(id), name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// EnsureArtifactDir creates the directory that would hold the named (possibly
// nested, e.g. "ec-1/rounds/round-2") artifact, so an agent subprocess can write
// into it.
func (w *Workspace) EnsureArtifactDir(id, name string) error {
	return os.MkdirAll(filepath.Join(w.RuntimeDir(id), name), 0o755)
}

// PlanPath returns the path to a feature's execution plan.
func (w *Workspace) PlanPath(id string) string {
	return filepath.Join(w.RuntimeDir(id), "execution-plan.yaml")
}

// ReadPlan loads the execution plan; a missing file yields an empty plan.
func (w *Workspace) ReadPlan(id string) (model.ExecutionPlan, error) {
	var plan model.ExecutionPlan
	data, err := os.ReadFile(w.PlanPath(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return plan, nil
		}
		return plan, err
	}
	if err := yaml.Unmarshal(data, &plan); err != nil {
		return plan, fmt.Errorf("%s: %w", w.PlanPath(id), err)
	}
	return plan, nil
}

// WritePlan persists the execution plan.
func (w *Workspace) WritePlan(id string, plan model.ExecutionPlan) error {
	data, err := yaml.Marshal(plan)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(w.RuntimeDir(id), 0o755); err != nil {
		return err
	}
	return os.WriteFile(w.PlanPath(id), data, 0o644)
}

func (w *Workspace) ReadArtifact(id, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(w.RuntimeDir(id), name))
	return string(data), err
}

func (w *Workspace) ArtifactExists(id, name string) bool {
	_, err := os.Stat(filepath.Join(w.RuntimeDir(id), name))
	return err == nil
}

func (w *Workspace) resolveFeaturePath(id string) (string, error) {
	exact := w.FeaturePath(id)
	if _, err := os.Stat(exact); err == nil {
		return exact, nil
	}
	matches, err := filepath.Glob(filepath.Join(w.DotDir(), "features", id+"*.yaml"))
	if err != nil {
		return "", err
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("feature %q not found or ambiguous", id)
	}
	return matches[0], nil
}

func readJSON(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(bufio.NewReader(file)).Decode(target)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func slugify(value string) string {
	value = strings.ToLower(value)
	var out strings.Builder
	dash := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			out.WriteRune(r)
			dash = false
		} else if !dash && out.Len() > 0 {
			out.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(out.String(), "-")
}
