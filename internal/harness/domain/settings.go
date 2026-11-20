package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type RolePolicy struct {
	Engine              string   `json:"engine"`
	Model               string   `json:"model"`
	HighDifficultyModel string   `json:"highDifficultyModel,omitempty"`
	MaxTurns            int      `json:"maxTurns"`
	PermissionMode      string   `json:"permissionMode"`
	AllowedTools        []string `json:"allowedTools"`
}

type PolicyMax struct {
	MaxTurns       int     `json:"maxTurns"`
	PerAgentRunUSD float64 `json:"perAgentRunUsd"`
}

// ModelPolicy is a local CLI launch setting, like ai_settings: which CLI and
// model alias each role runs with. It holds no keys and no provider routing.
type ModelPolicy struct {
	Roles           map[Role]RolePolicy `json:"roles"`
	DisallowedTools []string            `json:"disallowedTools"`
	Max             PolicyMax           `json:"max"`
	PerAgentRunUSD  float64             `json:"perAgentRunUsd"`
}

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

var readOnlyTools = []string{"Read", "Grep", "Glob"}

// AlwaysDisallowedTools are refused for every role, whatever the settings say.
var AlwaysDisallowedTools = []string{"Bash(git commit:*)", "Bash(git push:*)", "Bash(git checkout:*)", "Bash(git switch:*)", "Bash(git reset:*)", "Bash(rm -rf:*)"}

func DefaultModelPolicy() ModelPolicy {
	editTools := append(append([]string{}, readOnlyTools...), "Write", "Edit")
	gitRead := []string{"Bash(git diff:*)", "Bash(git log:*)", "Bash(git status:*)"}
	tests := []string{"Bash(go test:*)", "Bash(go vet:*)", "Bash(npm test:*)", "Bash(npm run typecheck:*)", "Bash(make verify:*)"}
	join := func(parts ...[]string) []string {
		var out []string
		for _, part := range parts {
			out = append(out, part...)
		}
		return out
	}
	return ModelPolicy{
		Roles: map[Role]RolePolicy{
			RolePlanner:      {Engine: "claudecode", Model: "opus", MaxTurns: 60, PermissionMode: "acceptEdits", AllowedTools: join(editTools, gitRead[:2])},
			RolePlanReviewer: {Engine: "claudecode", Model: "opus", MaxTurns: 25, PermissionMode: "plan", AllowedTools: join(readOnlyTools, gitRead[:1])},
			RoleCoder:        {Engine: "claudecode", Model: "sonnet", HighDifficultyModel: "opus", MaxTurns: 120, PermissionMode: "acceptEdits", AllowedTools: join(editTools, gitRead, tests)},
			RoleCodeReviewer: {Engine: "claudecode", Model: "opus", MaxTurns: 40, PermissionMode: "plan", AllowedTools: join(readOnlyTools, gitRead[:1], tests)},
			RoleQAFixer:      {Engine: "claudecode", Model: "sonnet", MaxTurns: 60, PermissionMode: "acceptEdits", AllowedTools: join(editTools, gitRead[:1], tests)},
		},
		DisallowedTools: append([]string{}, AlwaysDisallowedTools...),
		Max:             PolicyMax{MaxTurns: DefaultPolicyMaxTurn, PerAgentRunUSD: DefaultPolicyMaxUSD},
		PerAgentRunUSD:  5,
	}
}

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

// Normalize fills defaults and validates every knob against its cap. It
// returns an InvalidError naming the first value out of range.
func (s Settings) Normalize() (Settings, error) {
	defaults := DefaultSettings()
	if s.Paused == nil {
		s.Paused = map[RoleKey]bool{}
	}
	for key := range s.Paused {
		if !ValidRoleKey(key) {
			return s, Invalid("unknown role %q in paused", key)
		}
	}
	for _, pool := range []struct {
		name  string
		value *int
	}{{"planner", &s.Slots.Planner}, {"reviewer", &s.Slots.Reviewer}, {"coder", &s.Slots.Coder}, {"qa_fixer", &s.Slots.QAFixer}} {
		if *pool.value == 0 {
			*pool.value = 1
		}
		if *pool.value < 0 || *pool.value > maxSlotsPerPool {
			return s, Invalid("slots.%s must be between 1 and %d", pool.name, maxSlotsPerPool)
		}
	}
	for _, interval := range []struct {
		name     string
		value    *int
		fallback int
		min, max int
	}{
		{"scoutSeconds", &s.Intervals.ScoutSeconds, defaults.Intervals.ScoutSeconds, minIntervalSeconds, maxIntervalSeconds},
		{"sorterSeconds", &s.Intervals.SorterSeconds, defaults.Intervals.SorterSeconds, minIntervalSeconds, maxIntervalSeconds},
		{"workerSeconds", &s.Intervals.WorkerSeconds, defaults.Intervals.WorkerSeconds, 1, 3600},
		{"leaseSeconds", &s.Intervals.LeaseSeconds, defaults.Intervals.LeaseSeconds, minLeaseSeconds, maxLeaseSeconds},
	} {
		if *interval.value == 0 {
			*interval.value = interval.fallback
		}
		if *interval.value < interval.min || *interval.value > interval.max {
			return s, Invalid("intervals.%s must be between %d and %d", interval.name, interval.min, interval.max)
		}
	}
	if s.RetentionDays == 0 {
		s.RetentionDays = defaults.RetentionDays
	}
	if s.RetentionDays < 1 || s.RetentionDays > maxRetentionDays {
		return s, Invalid("retentionDays must be between 1 and %d", maxRetentionDays)
	}
	if s.Workspaces == nil {
		s.Workspaces = []WorkspaceSettings{}
	}
	seen := map[string]bool{}
	for i := range s.Workspaces {
		workspace := &s.Workspaces[i]
		workspace.WorkspaceID = strings.TrimSpace(workspace.WorkspaceID)
		if workspace.WorkspaceID == "" {
			return s, Invalid("workspaces[%d].workspaceId is required", i)
		}
		if seen[workspace.WorkspaceID] {
			return s, Invalid("workspace %s is listed twice", workspace.WorkspaceID)
		}
		seen[workspace.WorkspaceID] = true
		if workspace.MaxConcurrentRuns == 0 {
			workspace.MaxConcurrentRuns = 1
		}
		if workspace.MaxConcurrentRuns < 1 || workspace.MaxConcurrentRuns > maxConcurrentRuns {
			return s, Invalid("workspace %s maxConcurrentRuns must be between 1 and %d", workspace.WorkspaceID, maxConcurrentRuns)
		}
		if workspace.VerificationCommands == nil {
			workspace.VerificationCommands = []string{}
		}
		if len(workspace.VerificationCommands) > maxVerificationCmds {
			return s, Invalid("workspace %s lists more than %d verification commands", workspace.WorkspaceID, maxVerificationCmds)
		}
		for _, command := range workspace.VerificationCommands {
			if strings.TrimSpace(command) == "" || strings.ContainsAny(command, "\n\r") {
				return s, Invalid("workspace %s has an empty or multi-line verification command", workspace.WorkspaceID)
			}
		}
		if workspace.Scope == nil {
			workspace.Scope = []string{}
		}
		for _, prefix := range workspace.Scope {
			clean := strings.TrimSpace(prefix)
			if clean == "" || strings.HasPrefix(clean, "/") || strings.Contains(clean, "..") {
				return s, Invalid("workspace %s scope entry %q must be a relative path inside the repository", workspace.WorkspaceID, prefix)
			}
		}
		if workspace.Jira != nil {
			if workspace.Jira.Max == 0 {
				workspace.Jira.Max = 50
			}
			if workspace.Jira.Max < 1 || workspace.Jira.Max > maxJiraResults {
				return s, Invalid("workspace %s jira.max must be between 1 and %d", workspace.WorkspaceID, maxJiraResults)
			}
			if workspace.Jira.Enabled && strings.TrimSpace(workspace.Jira.JQL) == "" {
				return s, Invalid("workspace %s jira.jql is required when Jira is enabled", workspace.WorkspaceID)
			}
		}
		if workspace.GitHub != nil {
			if workspace.GitHub.Labels == nil {
				workspace.GitHub.Labels = []string{}
			}
			if workspace.GitHub.Enabled && strings.Count(strings.TrimSpace(workspace.GitHub.Repo), "/") != 1 {
				return s, Invalid("workspace %s github.repo must be owner/name", workspace.WorkspaceID)
			}
			if workspace.GitHub.TokenEnvVar == "" {
				workspace.GitHub.TokenEnvVar = "GITHUB_TOKEN"
			}
		}
		if workspace.Bitbucket != nil {
			if workspace.Bitbucket.Enabled && (strings.TrimSpace(workspace.Bitbucket.Workspace) == "" || strings.TrimSpace(workspace.Bitbucket.Repo) == "") {
				return s, Invalid("workspace %s bitbucket.workspace and bitbucket.repo are required", workspace.WorkspaceID)
			}
			if workspace.Bitbucket.TokenEnvVar == "" {
				workspace.Bitbucket.TokenEnvVar = "BITBUCKET_TOKEN"
			}
		}
		if workspace.Local != nil && workspace.Local.Enabled && strings.TrimSpace(workspace.Local.Dir) == "" {
			return s, Invalid("workspace %s local.dir is required when the local source is enabled", workspace.WorkspaceID)
		}
		for role, model := range workspace.Models {
			if !validRole(role) {
				return s, Invalid("workspace %s names an unknown role %q in models", workspace.WorkspaceID, role)
			}
			if model.MaxTurns < 0 || model.MaxTurns > DefaultPolicyMaxTurn {
				return s, Invalid("workspace %s models.%s.maxTurns must be between 1 and %d", workspace.WorkspaceID, role, DefaultPolicyMaxTurn)
			}
			if model == (RoleModel{}) {
				delete(workspace.Models, role)
			}
		}
		for role, step := range workspace.Procedure {
			if err := ValidateProcedure(role, step); err != nil {
				return s, Invalid("workspace %s: %v", workspace.WorkspaceID, err)
			}
		}
	}
	policy, err := s.ModelPolicy.Normalize()
	if err != nil {
		return s, err
	}
	s.ModelPolicy = policy
	if s.Ranker.LabelBoosts == nil {
		s.Ranker.LabelBoosts = map[string]float64{}
	}
	for label, boost := range s.Ranker.LabelBoosts {
		if boost < -100 || boost > 100 {
			return s, Invalid("ranker.labelBoosts[%s] must be between -100 and 100", label)
		}
	}
	if s.Ranker.LLM.TopN == 0 {
		s.Ranker.LLM.TopN = 10
	}
	if s.Ranker.LLM.TopN < 2 || s.Ranker.LLM.TopN > 50 {
		return s, Invalid("ranker.llm.topN must be between 2 and 50")
	}
	if s.Ranker.LLM.Enabled && (strings.TrimSpace(s.Ranker.LLM.Endpoint) == "" || strings.TrimSpace(s.Ranker.LLM.Model) == "") {
		return s, Invalid("ranker.llm.endpoint and ranker.llm.model are required when the re-ranker is enabled")
	}
	return s, nil
}

// Normalize fills missing roles from the defaults and caps every knob by max.
// The always-disallowed tools are added back if a person removed them.
func (p ModelPolicy) Normalize() (ModelPolicy, error) {
	defaults := DefaultModelPolicy()
	if p.Max.MaxTurns == 0 {
		p.Max.MaxTurns = defaults.Max.MaxTurns
	}
	if p.Max.PerAgentRunUSD == 0 {
		p.Max.PerAgentRunUSD = defaults.Max.PerAgentRunUSD
	}
	if p.Max.MaxTurns < 1 || p.Max.MaxTurns > DefaultPolicyMaxTurn {
		return p, Invalid("modelPolicy.max.maxTurns must be between 1 and %d", DefaultPolicyMaxTurn)
	}
	if p.Max.PerAgentRunUSD <= 0 || p.Max.PerAgentRunUSD > 1000 {
		return p, Invalid("modelPolicy.max.perAgentRunUsd must be above 0 and at most 1000")
	}
	if p.PerAgentRunUSD == 0 {
		p.PerAgentRunUSD = defaults.PerAgentRunUSD
	}
	if p.PerAgentRunUSD < 0 || p.PerAgentRunUSD > p.Max.PerAgentRunUSD {
		return p, Invalid("modelPolicy.perAgentRunUsd must be above 0 and at most %.2f", p.Max.PerAgentRunUSD)
	}
	if p.Roles == nil {
		p.Roles = map[Role]RolePolicy{}
	}
	for role := range p.Roles {
		if !validRole(role) {
			return p, Invalid("unknown role %q in modelPolicy.roles", role)
		}
	}
	for _, role := range Roles {
		policy, ok := p.Roles[role]
		if !ok {
			p.Roles[role] = defaults.Roles[role]
			continue
		}
		fallback := defaults.Roles[role]
		if strings.TrimSpace(policy.Engine) == "" {
			policy.Engine = fallback.Engine
		}
		if strings.TrimSpace(policy.Model) == "" {
			policy.Model = fallback.Model
		}
		if policy.MaxTurns == 0 {
			policy.MaxTurns = fallback.MaxTurns
		}
		if policy.MaxTurns < 1 || policy.MaxTurns > p.Max.MaxTurns {
			return p, Invalid("modelPolicy.roles.%s.maxTurns must be between 1 and %d", role, p.Max.MaxTurns)
		}
		switch policy.PermissionMode {
		case "":
			policy.PermissionMode = fallback.PermissionMode
		case "plan", "acceptEdits", "default":
		default:
			return p, Invalid("modelPolicy.roles.%s.permissionMode must be plan, acceptEdits or default", role)
		}
		if (role == RolePlanReviewer || role == RoleCodeReviewer) && policy.PermissionMode != "plan" {
			return p, Invalid("modelPolicy.roles.%s must stay read-only (permissionMode plan)", role)
		}
		if policy.AllowedTools == nil {
			policy.AllowedTools = fallback.AllowedTools
		}
		for _, tool := range policy.AllowedTools {
			if strings.TrimSpace(tool) == "" || strings.ContainsAny(tool, "\n\r") {
				return p, Invalid("modelPolicy.roles.%s.allowedTools has an empty or multi-line entry", role)
			}
			if isAlwaysDisallowed(tool) {
				return p, Invalid("modelPolicy.roles.%s may not allow %s", role, tool)
			}
		}
		p.Roles[role] = policy
	}
	p.DisallowedTools = mergeTools(p.DisallowedTools, AlwaysDisallowedTools)
	return p, nil
}

func validRole(role Role) bool {
	for _, candidate := range Roles {
		if candidate == role {
			return true
		}
	}
	return false
}

func isAlwaysDisallowed(tool string) bool {
	normalized := strings.ReplaceAll(strings.TrimSpace(tool), " ", "")
	for _, refused := range AlwaysDisallowedTools {
		if strings.ReplaceAll(refused, " ", "") == normalized {
			return true
		}
	}
	return strings.HasPrefix(normalized, "Bash(gitcommit") || strings.HasPrefix(normalized, "Bash(gitpush") || normalized == "Bash" || normalized == "Bash(*)"
}

func mergeTools(base, required []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, tool := range append(append([]string{}, base...), required...) {
		tool = strings.TrimSpace(tool)
		if tool == "" || seen[tool] {
			continue
		}
		seen[tool] = true
		out = append(out, tool)
	}
	return out
}

// ModelFor picks the model a role runs with, honouring the coder's
// difficulty override.
func (p ModelPolicy) ModelFor(role Role, difficulty string) string {
	policy := p.Roles[role]
	if difficulty == "high" && policy.HighDifficultyModel != "" {
		return policy.HighDifficultyModel
	}
	return policy.Model
}

// BudgetFor is the CLI's own --max-budget-usd for one agent run.
func (p ModelPolicy) BudgetFor() float64 {
	if p.PerAgentRunUSD > 0 && p.PerAgentRunUSD <= p.Max.PerAgentRunUSD {
		return p.PerAgentRunUSD
	}
	return p.Max.PerAgentRunUSD
}

func (s Settings) String() string {
	return fmt.Sprintf("harness settings revision %d", s.Revision)
}
