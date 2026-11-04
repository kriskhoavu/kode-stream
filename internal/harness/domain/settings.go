package domain

import (
	"fmt"
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

type BitbucketSourceSettings struct {
	Enabled     bool   `json:"enabled"`
	Workspace   string `json:"workspace"`
	Repo        string `json:"repo"`
	TokenEnvVar string `json:"tokenEnvVar"`
	Cloud       bool   `json:"cloud"`
	BaseURL     string `json:"baseUrl,omitempty"`
}

type WorkspaceSettings struct {
	WorkspaceID          string                   `json:"workspaceId"`
	MaxConcurrentRuns    int                      `json:"maxConcurrentRuns"`
	VerificationCommands []string                 `json:"verificationCommands"`
	Jira                 *JiraSourceSettings      `json:"jira,omitempty"`
	GitHub               *GitHubSourceSettings    `json:"github,omitempty"`
	Bitbucket            *BitbucketSourceSettings `json:"bitbucket,omitempty"`
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
	Enabled           bool                `json:"enabled"`
	AutoStart         bool                `json:"autoStart"`
	RequirePRApproval bool                `json:"requirePrApproval"`
	Slots             Slots               `json:"slots"`
	Intervals         Intervals           `json:"intervals"`
	Paused            map[RoleKey]bool    `json:"paused"`
	Workspaces        []WorkspaceSettings `json:"workspaces"`
	ModelPolicy       ModelPolicy         `json:"modelPolicy"`
	Ranker            RankerSettings      `json:"ranker"`
	RetentionDays     int                 `json:"retentionDays"`
	Revision          int64               `json:"revision"`
	UpdatedAt         *time.Time          `json:"updatedAt,omitempty"`
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

// DefaultSettings: disabled, Start is a human approval, one slot per pool.
func DefaultSettings() Settings {
	return Settings{
		Enabled:           false,
		AutoStart:         false,
		RequirePRApproval: false,
		Slots:             Slots{Planner: 1, Reviewer: 1, Coder: 1, QAFixer: 1},
		Intervals:         Intervals{ScoutSeconds: 300, SorterSeconds: 60, WorkerSeconds: 15, LeaseSeconds: 2700},
		Paused:            map[RoleKey]bool{},
		Workspaces:        []WorkspaceSettings{},
		ModelPolicy:       DefaultModelPolicy(),
		Ranker:            RankerSettings{LabelBoosts: map[string]float64{"bug": 5, "urgent": 10}, LLM: LLMRankerSettings{Endpoint: "http://127.0.0.1:8000/v1", Model: "qwen", APIKeyEnvVar: "", TopN: 10}},
		RetentionDays:     3,
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
