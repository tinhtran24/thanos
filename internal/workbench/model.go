package workbench

import "time"

type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	RootPath  string    `json:"root_path"`
	Repos     []Repo    `json:"repos,omitempty"`
	Settings  Settings  `json:"settings,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Repo struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Remote  string `json:"remote,omitempty"`
	Branch  string `json:"branch,omitempty"`
	Primary bool   `json:"primary,omitempty"`
}

type Settings struct {
	DefaultPlanner  string `json:"default_planner,omitempty"`
	DefaultCoder    string `json:"default_coder,omitempty"`
	DefaultReviewer string `json:"default_reviewer,omitempty"`
	DefaultTester   string `json:"default_tester,omitempty"`
}

type FeatureStatus string

const (
	FeatureBacklog FeatureStatus = "backlog"
	FeatureActive  FeatureStatus = "active"
	FeatureDone    FeatureStatus = "done"
)

type Feature struct {
	ID          string        `json:"id"`
	ProjectID   string        `json:"project_id"`
	Title       string        `json:"title"`
	Description string        `json:"description,omitempty"`
	Status      FeatureStatus `json:"status"`
	PlanGraphID string        `json:"plan_graph_id,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

type TaskStatus string

const (
	TaskBacklog         TaskStatus = "backlog"
	TaskPlanning        TaskStatus = "planning"
	TaskWaitingApproval TaskStatus = "waiting_approval"
	TaskReady           TaskStatus = "ready"
	TaskRunning         TaskStatus = "running"
	TaskInReview        TaskStatus = "in_review"
	TaskBlocked         TaskStatus = "blocked"
	TaskDone            TaskStatus = "done"
	TaskFailed          TaskStatus = "failed"
)

type Priority string

const (
	PriorityP0 Priority = "P0"
	PriorityP1 Priority = "P1"
	PriorityP2 Priority = "P2"
	PriorityP3 Priority = "P3"
)

type Task struct {
	ID              string     `json:"id"`
	FeatureID       string     `json:"feature_id,omitempty"`
	ParentTaskID    string     `json:"parent_task_id,omitempty"`
	Title           string     `json:"title"`
	Description     string     `json:"description,omitempty"`
	Status          TaskStatus `json:"status"`
	Priority        Priority   `json:"priority"`
	AssignedAgent   string     `json:"assigned_agent,omitempty"`
	ExecutorProfile string     `json:"executor_profile,omitempty"`
	WorktreePath    string     `json:"worktree_path,omitempty"`
	BranchName      string     `json:"branch_name,omitempty"`
	ReviewApproved  bool       `json:"review_approved"`
	TestsPassed     bool       `json:"tests_passed"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type ExecutionPlan struct {
	ID             string     `json:"id"`
	TaskID         string     `json:"task_id"`
	Summary        string     `json:"summary"`
	Steps          []PlanStep `json:"steps,omitempty"`
	Risks          []string   `json:"risks,omitempty"`
	FilesToTouch   []string   `json:"files_to_touch,omitempty"`
	TestStrategy   []string   `json:"test_strategy,omitempty"`
	ApprovalStatus string     `json:"approval_status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type PlanStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
}

type AgentProfile struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Provider     string            `json:"provider"`
	LaunchMode   string            `json:"launch_mode"`
	Command      string            `json:"command"`
	Env          map[string]string `json:"env,omitempty"`
	WorkingDir   string            `json:"working_dir,omitempty"`
	Permissions  []string          `json:"permissions,omitempty"`
	MaxRuntime   string            `json:"max_runtime,omitempty"`
	AutoContinue bool              `json:"auto_continue,omitempty"`
}

type AgentSession struct {
	ID                  string    `json:"id"`
	TaskID              string    `json:"task_id"`
	AgentType           string    `json:"agent_type"`
	Provider            string    `json:"provider"`
	Command             string    `json:"command"`
	Status              string    `json:"status"`
	PTYSessionID        string    `json:"pty_session_id,omitempty"`
	ConversationLogPath string    `json:"conversation_log_path,omitempty"`
	StartedAt           time.Time `json:"started_at"`
	EndedAt             time.Time `json:"ended_at,omitempty"`
}

type ReviewStatus string

const (
	ReviewPending          ReviewStatus = "pending"
	ReviewApproved         ReviewStatus = "approved"
	ReviewRejected         ReviewStatus = "rejected"
	ReviewChangesRequested ReviewStatus = "changes_requested"
)

type Review struct {
	ID            string       `json:"id"`
	TaskID        string       `json:"task_id"`
	DiffSummary   string       `json:"diff_summary,omitempty"`
	ChangedFiles  []string     `json:"changed_files,omitempty"`
	TestResults   []TestResult `json:"test_results,omitempty"`
	ReviewerNotes string       `json:"reviewer_notes,omitempty"`
	Status        ReviewStatus `json:"status"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

type TestResult struct {
	Command  string `json:"command"`
	Status   string `json:"status"`
	Output   string `json:"output,omitempty"`
	Duration string `json:"duration,omitempty"`
}

type MemoryNode struct {
	ID                string    `json:"id"`
	ProjectID         string    `json:"project_id"`
	Type              string    `json:"type"`
	Title             string    `json:"title"`
	Content           string    `json:"content"`
	Links             []string  `json:"links,omitempty"`
	EmbeddingOptional []float64 `json:"embedding_optional,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}
