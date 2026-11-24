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
	Enabled    bool   `json:"enabled"`
	JQL        string `json:"jql"`
	Max        int    `json:"max"`
	CheckReady *bool  `json:"checkReady,omitempty"`
	// Empty ReadyStatuses means DefaultReadyStatuses.
	ReadyStatuses []string `json:"readyStatuses,omitempty"`
}

var DefaultReadyStatuses = []string{"READY"}

const MaxReadyStatuses = 10

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
	WorkspaceID          string   `json:"workspaceId"`
	MaxConcurrentRuns    int      `json:"maxConcurrentRuns"`
	AskQuestions         *bool    `json:"askQuestions,omitempty"`
	VerificationCommands []string `json:"verificationCommands"`
	// Empty Scope allows any path that is not protected.
	Scope          []string                 `json:"scope"`
	Jira           *JiraSourceSettings      `json:"jira,omitempty"`
	GitHub         *GitHubSourceSettings    `json:"github,omitempty"`
	Bitbucket      *BitbucketSourceSettings `json:"bitbucket,omitempty"`
	Local          *LocalSourceSettings     `json:"local,omitempty"`
	Enabled        *bool                    `json:"enabled,omitempty"`
	AutoStart      *bool                    `json:"autoStart,omitempty"`
	RequireReview  *bool                    `json:"requireReview,omitempty"`
	Models         map[Role]RoleModel       `json:"models,omitempty"`
	Procedure      map[Role]ProcedureStep   `json:"procedure,omitempty"`
	ProcedureMoved bool                     `json:"procedureMoved,omitempty"`
}

type RoleModel struct {
	Engine              string `json:"engine,omitempty"`
	Model               string `json:"model,omitempty"`
	HighDifficultyModel string `json:"highDifficultyModel,omitempty"`
	MaxTurns            int    `json:"maxTurns,omitempty"`
}

type ProcedureStep struct {
	Skill string `json:"skill" yaml:"skill"`
	Track string `json:"track,omitempty" yaml:"track,omitempty"`
}

var (
	skillName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	trackName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _-]{0,39}$`)
)

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
				// Once models are the workspace's own, an empty hard model means it has none.
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
	Enabled       bool `json:"enabled"`
	AutoStart     bool `json:"autoStart"`
	RequireReview bool `json:"requireReview"`
	// Nil AskQuestions means yes. A workspace or a task may override it.
	AskQuestions    *bool               `json:"askQuestions,omitempty"`
	Slots           Slots               `json:"slots"`
	Intervals       Intervals           `json:"intervals"`
	Paused          map[RoleKey]bool    `json:"paused"`
	Workspaces      []WorkspaceSettings `json:"workspaces"`
	ModelPolicy     ModelPolicy         `json:"modelPolicy"`
	Ranker          RankerSettings      `json:"ranker"`
	RetentionDays   int                 `json:"retentionDays"`
	WorkspaceModels bool                `json:"workspaceModels,omitempty"`
	Revision        int64               `json:"revision"`
	UpdatedAt       *time.Time          `json:"updatedAt,omitempty"`
}

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
