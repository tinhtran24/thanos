package model

import "time"

type Phase string

const (
	PhaseInit         Phase = "init"
	PhaseDesign       Phase = "designing"
	PhaseDesignReview Phase = "design-reviewing"
	PhaseCode         Phase = "coding"
	PhaseReview       Phase = "reviewing"
	PhaseTest         Phase = "testing"
	PhaseDeepReview   Phase = "deep-reviewing"
	PhaseAmend        Phase = "amending"
	PhaseAccept       Phase = "accepting"
	PhasePending      Phase = "pending-review"
	PhaseDone         Phase = "done"
	PhaseBlocked      Phase = "blocked"
	PhaseAttention    Phase = "needs-attention"
)

type Role string

const (
	RoleDesigner       Role = "designer"
	RoleDesignReviewer Role = "design-reviewer"
	RoleCoder          Role = "coder"
	RoleReviewer       Role = "reviewer"
	RoleTester         Role = "tester"
	RoleDeepReviewer   Role = "deep-reviewer"
	RoleAcceptor       Role = "acceptor"
	RoleMiniCoder      Role = "mini-coder"
	RoleReVerifier     Role = "re-verifier"
	RoleSynthesizer    Role = "synthesizer"
	RoleGate           Role = "gate"
)

type Project struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Language       string   `json:"language"`
	Framework      string   `json:"framework,omitempty"`
	PackageManager string   `json:"package_manager,omitempty"`
	MultiPackage   bool     `json:"multi_package,omitempty"`
	Packages       []string `json:"packages,omitempty"`
	Build          []string `json:"build,omitempty"`
	Test           []string `json:"test,omitempty"`
	Lint           []string `json:"lint,omitempty"`
	Rules          []string `json:"rules,omitempty"`
}

type Runner struct {
	Command   string   `json:"command"`
	Args      []string `json:"args,omitempty"`
	Agent     string   `json:"agent,omitempty"`
	SkillsDir string   `json:"skills_dir,omitempty"`
}

type LSP struct {
	Command  string            `json:"command"`
	Args     []string          `json:"args,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	Disabled bool              `json:"disabled,omitempty"`
}

type MCP struct {
	Type          string            `json:"type"`
	Command       string            `json:"command,omitempty"`
	Args          []string          `json:"args,omitempty"`
	URL           string            `json:"url,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Disabled      bool              `json:"disabled,omitempty"`
	DisabledTools []string          `json:"disabled_tools,omitempty"`
}

type Skill struct {
	Name   string   `json:"name"`
	Path   string   `json:"path"`
	Source string   `json:"source,omitempty"`
	Roles  []string `json:"roles,omitempty"`
}

type PluginMarketplace struct {
	Runner string `json:"runner"`
	Source string `json:"source"`
}

type Plugin struct {
	Runner string `json:"runner"`
	Name   string `json:"name"`
	Scope  string `json:"scope,omitempty"`
}

type Config struct {
	Project            Project             `json:"project"`
	DefaultRunner      string              `json:"default_runner"`
	MaxRounds          int                 `json:"max_rounds"`
	Runners            map[string]Runner   `json:"runners"`
	LSP                map[string]LSP      `json:"lsp,omitempty"`
	MCP                map[string]MCP      `json:"mcp,omitempty"`
	Skills             []Skill             `json:"skills,omitempty"`
	PluginMarketplaces []PluginMarketplace `json:"plugin_marketplaces,omitempty"`
	Plugins            []Plugin            `json:"plugins,omitempty"`
	Locale             string              `json:"locale,omitempty"`
}

type Feature struct {
	ID           string   `yaml:"id" json:"id"`
	Title        string   `yaml:"title" json:"title"`
	Type         string   `yaml:"type,omitempty" json:"type,omitempty"`
	Parent       string   `yaml:"parent,omitempty" json:"parent,omitempty"`
	Description  string   `yaml:"description" json:"description"`
	Acceptance   []string `yaml:"acceptance" json:"acceptance"`
	Rules        []string `yaml:"rules,omitempty" json:"rules,omitempty"`
	Decisions    []string `yaml:"decisions,omitempty" json:"decisions,omitempty"`
	Scope        []string `yaml:"scope,omitempty" json:"scope,omitempty"`
	Related      []string `yaml:"related,omitempty" json:"related,omitempty"`
	Dependencies []string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Priority     string   `yaml:"priority" json:"priority"`
	Status       string   `yaml:"status" json:"status"`
	Runner       string   `yaml:"runner,omitempty" json:"runner,omitempty"`
	MaxRounds    int      `yaml:"max_rounds,omitempty" json:"max_rounds,omitempty"`
}

type State struct {
	FeatureID string    `json:"feature_id"`
	Phase     Phase     `json:"phase"`
	Role      Role      `json:"role,omitempty"`
	Round     int       `json:"round"`
	MaxRounds int       `json:"max_rounds"`
	Active    bool      `json:"active"`
	Runner    string    `json:"runner,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Reason    string    `json:"reason,omitempty"`
}

type Event struct {
	Type      string         `json:"type"`
	FeatureID string         `json:"feature_id"`
	Timestamp time.Time      `json:"timestamp"`
	Phase     Phase          `json:"phase,omitempty"`
	Role      Role           `json:"role,omitempty"`
	Round     int            `json:"round,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}
