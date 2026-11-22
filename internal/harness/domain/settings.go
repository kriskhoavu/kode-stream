package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Slots struct {
	Planner  int `json:"planner"`
	Reviewer int `json:"reviewer"`
	Coder    int `json:"coder"`
	QAFixer  int `json:"qa_fixer"`
}

func (s Slots) For(pool RoleKey) int {
	switch pool {
	case RoleKeyPlanner:
		return s.Planner
	case RoleKeyReviewer:
		return s.Reviewer
	case RoleKeyCoder:
		return s.Coder
	case RoleKeyQAFixer:
		return s.QAFixer
	}
	return 1
}

type Intervals struct {
	ScoutSeconds  int `json:"scoutSeconds"`
	SorterSeconds int `json:"sorterSeconds"`
	WorkerSeconds int `json:"workerSeconds"`
	LeaseSeconds  int `json:"leaseSeconds"`
}

type JiraSourceSettings struct {
	Enabled bool   `json:"enabled"`
	JQL     string `json:"jql"`
	Max     int    `json:"max"`
	// CheckReady turns the readiness check on; nil means off. With it on, a ticket in one of ReadyStatuses starts on its own, and its planner
	// first judges whether the ticket says enough to plan from.
	CheckReady *bool `json:"checkReady,omitempty"`
	// ReadyStatuses are the Jira statuses that mean a ticket is ready to
	// work on; empty means DefaultReadyStatuses.
	ReadyStatuses []string `json:"readyStatuses,omitempty"`
}

// DefaultReadyStatuses is the Jira status a ticket is ready in, unless a
// workspace names its own.
var DefaultReadyStatuses = []string{"READY"}

// MaxReadyStatuses bounds how many ready statuses a workspace names.
const MaxReadyStatuses = 10

// Ready reports whether the readiness check applies to a ticket in status:
// Jira is on, the check is on, and status is one of the ready ones, in any
// case.
func (j *JiraSourceSettings) Ready(status string) bool {
	if j == nil || !j.Enabled || j.CheckReady == nil || !*j.CheckReady {
		return false
	}
	status = strings.TrimSpace(status)
	if status == "" {
		return false
	}
	statuses := j.ReadyStatuses
	if len(statuses) == 0 {
		statuses = DefaultReadyStatuses
	}
	for _, ready := range statuses {
		if strings.EqualFold(strings.TrimSpace(ready), status) {
			return true
		}
	}
	return false
}

type GitHubSourceSettings struct {
	Enabled     bool     `json:"enabled"`
	Repo        string   `json:"repo"`
	Labels      []string `json:"labels"`
	TokenEnvVar string   `json:"tokenEnvVar"`
}

// LocalSourceSettings names a folder of task files, one Markdown file per
// task. A relative Dir is read from the workspace's checkout.
type LocalSourceSettings struct {
	Enabled bool   `json:"enabled"`
	Dir     string `json:"dir"`
}

type BitbucketSourceSettings struct {
	Enabled     bool   `json:"enabled"`
	Workspace   string `json:"workspace"`
	Repo        string `json:"repo"`
	TokenEnvVar string `json:"tokenEnvVar"`
	Cloud       bool   `json:"cloud"`
	BaseURL     string `json:"baseUrl,omitempty"`
}

type WorkspaceSettings struct {
	WorkspaceID       string `json:"workspaceId"`
	MaxConcurrentRuns int    `json:"maxConcurrentRuns"`
	// AskQuestions overrides, for this workspace, whether an agent may stop
	// and ask a person; nil follows the settings.
	AskQuestions         *bool    `json:"askQuestions,omitempty"`
	VerificationCommands []string `json:"verificationCommands"`
	// Scope lists the path prefixes a builder may change; empty allows any
	// path that is not protected.
	Scope     []string                 `json:"scope"`
	Jira      *JiraSourceSettings      `json:"jira,omitempty"`
	GitHub    *GitHubSourceSettings    `json:"github,omitempty"`
	Bitbucket *BitbucketSourceSettings `json:"bitbucket,omitempty"`
	Local     *LocalSourceSettings     `json:"local,omitempty"`
	// The overrides below lay this workspace's own choices over the
	// settings'; nil or empty follows the settings.
	Enabled       *bool                  `json:"enabled,omitempty"`
	AutoStart     *bool                  `json:"autoStart,omitempty"`
	RequireReview *bool                  `json:"requireReview,omitempty"`
	Models        map[Role]RoleModel     `json:"models,omitempty"`
	Procedure     map[Role]ProcedureStep `json:"procedure,omitempty"`
	// ProcedureMoved records that the repository's harness.yaml, if it had
	// one, was moved into Procedure, so it is never read again.
	ProcedureMoved bool `json:"procedureMoved,omitempty"`
}

// RoleModel is a workspace's choice of CLI and model for one role. An empty
// field follows the settings. It holds no tool permissions: those stay the
// settings' alone.
type RoleModel struct {
	Engine              string `json:"engine,omitempty"`
	Model               string `json:"model,omitempty"`
	HighDifficultyModel string `json:"highDifficultyModel,omitempty"`
	MaxTurns            int    `json:"maxTurns,omitempty"`
}

// ProcedureStep is the repository's own skill, and the track of it, that a
// role follows in a workspace.
type ProcedureStep struct {
	Skill string `json:"skill" yaml:"skill"`
	Track string `json:"track,omitempty" yaml:"track,omitempty"`
}

var (
	skillName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	trackName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _-]{0,39}$`)
)

// ValidateProcedure checks a role's skill and track.
func ValidateProcedure(role Role, step ProcedureStep) error {
	if !validRole(role) {
		return Invalid("unknown role %q in procedure", role)
	}
	if !skillName.MatchString(step.Skill) {
		return Invalid("procedure.%s.skill %q must be lower-case letters, digits, - or _", role, step.Skill)
	}
	if step.Track != "" && !trackName.MatchString(step.Track) {
		return Invalid("procedure.%s.track %q must be letters, digits, spaces, - or _", role, step.Track)
	}
	return nil
}

// For is the settings one workspace runs with: the settings, with that
// workspace's own choices laid over them.
func (s Settings) For(workspaceID string) Settings {
	workspace, ok := s.Workspace(workspaceID)
	if !ok {
		return s
	}
	out := s
	if workspace.Enabled != nil {
		out.Enabled = s.Enabled && *workspace.Enabled
	}
	if workspace.AutoStart != nil {
		out.AutoStart = *workspace.AutoStart
	}
	if workspace.RequireReview != nil {
		out.RequireReview = *workspace.RequireReview
	}
	if len(workspace.Models) > 0 {
		roles := make(map[Role]RolePolicy, len(s.ModelPolicy.Roles))
		for role, policy := range s.ModelPolicy.Roles {
			if model, ok := workspace.Models[role]; ok {
				if model.Engine != "" {
					policy.Engine = model.Engine
				}
				if model.Model != "" {
					policy.Model = model.Model
				}
				// Once models are the workspace's own, an empty hard model
				// means it has none, not that it follows anything.
				if s.WorkspaceModels || model.HighDifficultyModel != "" {
					policy.HighDifficultyModel = model.HighDifficultyModel
				}
				if model.MaxTurns > 0 {
					policy.MaxTurns = min(model.MaxTurns, max(s.ModelPolicy.Max.MaxTurns, 1))
				}
			}
			roles[role] = policy
		}
		out.ModelPolicy.Roles = roles
	}
	return out
}

// OwnModels gives each listed workspace a model for every role, so the
// workspace holds its own and nothing global decides them. The first time,
// a workspace's choices are completed from the settings' models, which are
// then set back to Kode Stream's defaults; after that a workspace that lacks
// a role starts from those defaults. It reports whether anything changed.
func (s *Settings) OwnModels(workspaceIDs []string) bool {
	first := !s.WorkspaceModels
	source := DefaultModelPolicy().Roles
	if first {
		source = s.ModelPolicy.Roles
	}
	changed := first
	for _, id := range workspaceIDs {
		index := -1
		for i := range s.Workspaces {
			if s.Workspaces[i].WorkspaceID == id {
				index = i
			}
		}
		if index < 0 {
			s.Workspaces = append(s.Workspaces, WorkspaceSettings{WorkspaceID: id})
			index = len(s.Workspaces) - 1
		}
		workspace := &s.Workspaces[index]
		if workspace.Models == nil {
			workspace.Models = map[Role]RoleModel{}
		}
		for _, role := range Roles {
			from := source[role]
			model, had := workspace.Models[role]
			before := model
			if model.Engine == "" {
				model.Engine = from.Engine
			}
			if model.Model == "" {
				model.Model = from.Model
			}
			// Before the move an empty hard model followed the settings';
			// after it, a role the workspace already holds keeps its choice.
			if model.HighDifficultyModel == "" && (first || !had) {
				model.HighDifficultyModel = from.HighDifficultyModel
			}
			if model.MaxTurns == 0 {
				model.MaxTurns = from.MaxTurns
			}
			if !had || model != before {
				workspace.Models[role] = model
				changed = true
			}
		}
	}
	if first {
		defaults := DefaultModelPolicy().Roles
		for role, policy := range s.ModelPolicy.Roles {
			fallback := defaults[role]
			policy.Engine, policy.Model, policy.HighDifficultyModel, policy.MaxTurns = fallback.Engine, fallback.Model, fallback.HighDifficultyModel, fallback.MaxTurns
			s.ModelPolicy.Roles[role] = policy
		}
		s.WorkspaceModels = true
	}
	return changed
}

type LLMRankerSettings struct {
	Enabled      bool   `json:"enabled"`
	Endpoint     string `json:"endpoint"`
	Model        string `json:"model"`
	APIKeyEnvVar string `json:"apiKeyEnvVar"`
	TopN         int    `json:"topN"`
}

type RankerSettings struct {
	LabelBoosts map[string]float64 `json:"labelBoosts"`
	LLM         LLMRankerSettings  `json:"llm"`
}

type Settings struct {
	Enabled   bool `json:"enabled"`
	AutoStart bool `json:"autoStart"`
	// RequireReview stops a run whose tests passed at Review, where a person
	// tries the change before it is handed off. On unless turned off.
	RequireReview bool `json:"requireReview"`
	// AskQuestions lets an agent stop its round and ask a person when its
	// report is blocked or lists open questions; nil means yes. A workspace
	// or a task may override it.
	AskQuestions  *bool               `json:"askQuestions,omitempty"`
	Slots         Slots               `json:"slots"`
	Intervals     Intervals           `json:"intervals"`
	Paused        map[RoleKey]bool    `json:"paused"`
	Workspaces    []WorkspaceSettings `json:"workspaces"`
	ModelPolicy   ModelPolicy         `json:"modelPolicy"`
	Ranker        RankerSettings      `json:"ranker"`
	RetentionDays int                 `json:"retentionDays"`
	// WorkspaceModels records that each workspace holds its own models,
	// copied from the defaults once; ModelPolicy's engines and models are
	// then only what a new workspace starts with.
	WorkspaceModels bool       `json:"workspaceModels,omitempty"`
	Revision        int64      `json:"revision"`
	UpdatedAt       *time.Time `json:"updatedAt,omitempty"`
}

// QuestionGate reports whether an agent working on task may stop and ask a
// person: the task's own choice first, then its workspace's, then the
// settings', and yes when none says.
func (s Settings) QuestionGate(task Task) bool {
	if task.AskQuestions != nil {
		return *task.AskQuestions
	}
	if workspace, ok := s.Workspace(task.WorkspaceID); ok && workspace.AskQuestions != nil {
		return *workspace.AskQuestions
	}
	if s.AskQuestions != nil {
		return *s.AskQuestions
	}
	return true
}

// ReadyCheck reports whether task is a Jira ticket in one of its workspace's
// ready statuses with the readiness check on: such a task starts on its own,
// and its planner first judges whether the ticket says enough to plan from.
func (s Settings) ReadyCheck(task Task) bool {
	if task.Source.Kind != SourceJira {
		return false
	}
	workspace, ok := s.Workspace(task.WorkspaceID)
	return ok && workspace.Jira.Ready(task.Source.Status)
}

func (s Settings) Workspace(id string) (WorkspaceSettings, bool) {
	for _, workspace := range s.Workspaces {
		if workspace.WorkspaceID == id {
			return workspace, true
		}
	}
	return WorkspaceSettings{}, false
}

// Limits on settings a person can change.
const (
	maxSlotsPerPool      = 8
	maxConcurrentRuns    = 8
	minIntervalSeconds   = 5
	maxIntervalSeconds   = 86400
	minLeaseSeconds      = 60
	maxLeaseSeconds      = 4 * 3600
	maxRetentionDays     = 90
	maxVerificationCmds  = 10
	maxJiraResults       = 200
	DefaultPolicyMaxTurn = 200
	DefaultPolicyMaxUSD  = 25.0
)

// DefaultSettings: disabled, Start is a human approval, Review is on, one
// slot per pool.
func DefaultSettings() Settings {
	return Settings{
		Enabled:       false,
		AutoStart:     false,
		RequireReview: true,
		Slots:         Slots{Planner: 1, Reviewer: 1, Coder: 1, QAFixer: 1},
		Intervals:     Intervals{ScoutSeconds: 300, SorterSeconds: 60, WorkerSeconds: 15, LeaseSeconds: 2700},
		Paused:        map[RoleKey]bool{},
		Workspaces:    []WorkspaceSettings{},
		ModelPolicy:   DefaultModelPolicy(),
		Ranker:        RankerSettings{LabelBoosts: map[string]float64{"bug": 5, "urgent": 10}, LLM: LLMRankerSettings{Endpoint: "http://127.0.0.1:8000/v1", Model: "qwen", APIKeyEnvVar: "", TopN: 10}},
		RetentionDays: 3,
	}
}

func (s Settings) String() string {
	return fmt.Sprintf("harness settings revision %d", s.Revision)
}
