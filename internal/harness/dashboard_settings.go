package harness

import (
	"context"
	"strings"
	"time"

	"kode-stream/internal/harness/domain"
)

type DashboardSource struct {
	WorkspaceID   string     `json:"workspaceId"`
	Kind          string     `json:"kind"`
	Enabled       bool       `json:"enabled"`
	Query         string     `json:"query"`
	Labels        []string   `json:"labels,omitempty"`
	TokenEnvVar   string     `json:"tokenEnvVar,omitempty"`
	CheckReady    *bool      `json:"checkReady,omitempty"`
	ReadyStatuses []string   `json:"readyStatuses,omitempty"`
	LastCheckedAt *time.Time `json:"lastCheckedAt,omitempty"`
	LastFound     int        `json:"lastFound,omitempty"`
	LastError     string     `json:"lastError,omitempty"`
}

type DashboardWorkspace struct {
	WorkspaceID          string                               `json:"workspaceId"`
	Enabled              *bool                                `json:"enabled,omitempty"`
	AutoStart            *bool                                `json:"autoStart,omitempty"`
	RequireReview        *bool                                `json:"requireReview,omitempty"`
	AskQuestions         *bool                                `json:"askQuestions,omitempty"`
	VerificationCommands []string                             `json:"verificationCommands"`
	Scope                []string                             `json:"scope"`
	Models               map[domain.Role]domain.RoleModel     `json:"models"`
	Procedure            map[domain.Role]domain.ProcedureStep `json:"procedure"`
}

type DashboardRolePolicy struct {
	Engine    string `json:"engine"`
	Model     string `json:"model"`
	HardModel string `json:"hardModel,omitempty"`
	MaxTurns  int    `json:"maxTurns"`
}

type DashboardSettings struct {
	Revision      int64             `json:"revision"`
	Enabled       bool              `json:"enabled"`
	AutoStart     bool              `json:"autoStart"`
	RequireReview bool              `json:"requireReview"`
	AskQuestions  bool              `json:"askQuestions"`
	Slots         map[string]int    `json:"slots"`
	Paused        map[string]bool   `json:"paused"`
	Sources       []DashboardSource `json:"sources"`
	// A save that omits Workspaces keeps the stored ones.
	Workspaces       []DashboardWorkspace                `json:"workspaces"`
	ModelPolicy      map[domain.Role]DashboardRolePolicy `json:"modelPolicy"`
	WorkspaceModels  bool                                `json:"workspaceModels"`
	ExpectedRevision *int64                              `json:"expectedRevision,omitempty"`
}

func (s *Service) DashboardSettings(ctx context.Context) (DashboardSettings, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return DashboardSettings{}, err
	}
	view := DashboardSettings{Revision: settings.Revision, WorkspaceModels: settings.WorkspaceModels, Enabled: settings.Enabled, AutoStart: settings.AutoStart, RequireReview: settings.RequireReview, AskQuestions: settings.QuestionGate(domain.Task{}),
		Slots:  map[string]int{"scout": 1, "sorter": 1, "planner": settings.Slots.Planner, "reviewer": settings.Slots.Reviewer, "coder": settings.Slots.Coder, "qa": settings.Slots.QAFixer},
		Paused: map[string]bool{}, Sources: []DashboardSource{}, Workspaces: []DashboardWorkspace{}, ModelPolicy: map[domain.Role]DashboardRolePolicy{}}
	for key, paused := range settings.Paused {
		if paused {
			view.Paused[DashboardRoleOf(key)] = true
		}
	}
	health := map[string]SourceHealth{}
	for _, source := range s.sourceHealth() {
		health[source.WorkspaceID+"\x00"+source.Kind] = source
	}
	add := func(source DashboardSource) {
		if known, ok := health[source.WorkspaceID+"\x00"+source.Kind]; ok {
			source.LastCheckedAt, source.LastFound, source.LastError = known.LastPolledAt, known.Added, known.LastError
		}
		view.Sources = append(view.Sources, source)
	}
	for _, workspace := range settings.Workspaces {
		if jira := workspace.Jira; jira != nil {
			add(DashboardSource{WorkspaceID: workspace.WorkspaceID, Kind: string(domain.SourceJira), Enabled: jira.Enabled, Query: jira.JQL, CheckReady: jira.CheckReady, ReadyStatuses: jira.ReadyStatuses})
		}
		if github := workspace.GitHub; github != nil {
			add(DashboardSource{WorkspaceID: workspace.WorkspaceID, Kind: string(domain.SourceGitHub), Enabled: github.Enabled, Query: github.Repo, Labels: github.Labels, TokenEnvVar: github.TokenEnvVar})
		}
		if bitbucket := workspace.Bitbucket; bitbucket != nil {
			add(DashboardSource{WorkspaceID: workspace.WorkspaceID, Kind: string(domain.SourceBitbucket), Enabled: bitbucket.Enabled, Query: bitbucket.Workspace + "/" + bitbucket.Repo, TokenEnvVar: bitbucket.TokenEnvVar})
		}
		if local := workspace.Local; local != nil {
			add(DashboardSource{WorkspaceID: workspace.WorkspaceID, Kind: string(domain.SourceLocal), Enabled: local.Enabled, Query: local.Dir})
		}
	}
	for _, workspace := range settings.Workspaces {
		view.Workspaces = append(view.Workspaces, DashboardWorkspace{WorkspaceID: workspace.WorkspaceID, Enabled: workspace.Enabled, AutoStart: workspace.AutoStart, RequireReview: workspace.RequireReview, AskQuestions: workspace.AskQuestions,
			VerificationCommands: nonNil(workspace.VerificationCommands), Scope: nonNil(workspace.Scope), Models: orEmpty(workspace.Models), Procedure: orEmpty(workspace.Procedure)})
	}
	for _, role := range domain.Roles {
		policy := settings.ModelPolicy.Roles[role]
		view.ModelPolicy[role] = DashboardRolePolicy{Engine: policy.Engine, Model: policy.Model, HardModel: policy.HighDifficultyModel, MaxTurns: policy.MaxTurns}
	}
	return view, nil
}

// Settings the dashboard does not show keep their stored values.
func (s *Service) SaveDashboardSettings(ctx context.Context, view DashboardSettings, actor domain.Actor) (DashboardSettings, error) {
	if err := s.mutable(); err != nil {
		return DashboardSettings{}, err
	}
	settings := s.settingsNow(ctx)
	expected := view.Revision
	if view.ExpectedRevision != nil {
		expected = *view.ExpectedRevision
	}
	settings.Enabled, settings.AutoStart, settings.RequireReview = view.Enabled, view.AutoStart, view.RequireReview
	ask := view.AskQuestions
	settings.AskQuestions = &ask
	slot := func(key string, current int) int {
		if value, ok := view.Slots[key]; ok && value > 0 {
			return value
		}
		return current
	}
	settings.Slots = domain.Slots{Planner: slot("planner", settings.Slots.Planner), Reviewer: slot("reviewer", settings.Slots.Reviewer), Coder: slot("coder", settings.Slots.Coder), QAFixer: slot("qa", settings.Slots.QAFixer)}
	if view.Paused != nil {
		settings.Paused = map[domain.RoleKey]bool{}
		for role, paused := range view.Paused {
			key := RoleKeyFromDashboard(role)
			if !domain.ValidRoleKey(key) {
				return DashboardSettings{}, domain.Invalid("unknown role %q in paused", role)
			}
			if paused {
				settings.Paused[key] = true
			}
		}
	}
	if view.Sources != nil {
		if err := applySources(&settings, view.Sources); err != nil {
			return DashboardSettings{}, err
		}
	}
	if view.Workspaces != nil {
		applyWorkspaces(&settings, view.Workspaces)
	}
	if !settings.WorkspaceModels {
		for role, policy := range view.ModelPolicy {
			current, ok := settings.ModelPolicy.Roles[role]
			if !ok {
				return DashboardSettings{}, domain.Invalid("unknown role %q in modelPolicy", role)
			}
			current.Engine, current.Model, current.MaxTurns = policy.Engine, policy.Model, policy.MaxTurns
			if role == domain.RoleCoder {
				current.HighDifficultyModel = policy.HardModel
			}
			settings.ModelPolicy.Roles[role] = current
		}
	}
	if _, err := s.SaveSettings(ctx, settings, expected, actor); err != nil {
		return DashboardSettings{}, err
	}
	return s.DashboardSettings(ctx)
}

func applySources(settings *domain.Settings, sources []DashboardSource) error {
	listed := map[string]map[string]DashboardSource{}
	order := []string{}
	for _, source := range sources {
		if strings.TrimSpace(source.WorkspaceID) == "" {
			return domain.Invalid("a source needs its workspaceId")
		}
		if listed[source.WorkspaceID] == nil {
			listed[source.WorkspaceID] = map[string]DashboardSource{}
			order = append(order, source.WorkspaceID)
		}
		listed[source.WorkspaceID][source.Kind] = source
	}
	index := map[string]int{}
	for i, workspace := range settings.Workspaces {
		index[workspace.WorkspaceID] = i
	}
	for _, id := range order {
		if _, ok := index[id]; !ok {
			settings.Workspaces = append(settings.Workspaces, domain.WorkspaceSettings{WorkspaceID: id})
			index[id] = len(settings.Workspaces) - 1
		}
	}
	for i := range settings.Workspaces {
		workspace := &settings.Workspaces[i]
		kinds := listed[workspace.WorkspaceID]
		workspace.Jira, workspace.GitHub, workspace.Local = nil, nil, nil
		previousBitbucket := workspace.Bitbucket
		workspace.Bitbucket = nil
		for kind, source := range kinds {
			query := strings.TrimSpace(source.Query)
			switch domain.SourceKind(kind) {
			case domain.SourceJira:
				workspace.Jira = &domain.JiraSourceSettings{Enabled: source.Enabled, JQL: query, CheckReady: source.CheckReady, ReadyStatuses: source.ReadyStatuses}
			case domain.SourceGitHub:
				workspace.GitHub = &domain.GitHubSourceSettings{Enabled: source.Enabled, Repo: query, Labels: source.Labels, TokenEnvVar: source.TokenEnvVar}
			case domain.SourceBitbucket:
				owner, repo, _ := strings.Cut(query, "/")
				bitbucket := &domain.BitbucketSourceSettings{Enabled: source.Enabled, Workspace: owner, Repo: repo, TokenEnvVar: source.TokenEnvVar, Cloud: true}
				if previousBitbucket != nil {
					bitbucket.Cloud, bitbucket.BaseURL = previousBitbucket.Cloud, previousBitbucket.BaseURL
				}
				workspace.Bitbucket = bitbucket
			case domain.SourceLocal:
				workspace.Local = &domain.LocalSourceSettings{Enabled: source.Enabled, Dir: query}
			default:
				return domain.Invalid("source kind %q must be jira, github, bitbucket or local", kind)
			}
		}
	}
	return nil
}

func applyWorkspaces(settings *domain.Settings, workspaces []DashboardWorkspace) {
	for _, view := range workspaces {
		if strings.TrimSpace(view.WorkspaceID) == "" {
			continue
		}
		index := -1
		for i := range settings.Workspaces {
			if settings.Workspaces[i].WorkspaceID == view.WorkspaceID {
				index = i
			}
		}
		if index < 0 {
			settings.Workspaces = append(settings.Workspaces, domain.WorkspaceSettings{WorkspaceID: view.WorkspaceID})
			index = len(settings.Workspaces) - 1
		}
		workspace := &settings.Workspaces[index]
		workspace.Enabled, workspace.AutoStart, workspace.RequireReview, workspace.AskQuestions = view.Enabled, view.AutoStart, view.RequireReview, view.AskQuestions
		workspace.VerificationCommands = cleanCommands(view.VerificationCommands)
		workspace.Scope = cleanCommands(view.Scope)
		workspace.Models, workspace.Procedure = view.Models, view.Procedure
	}
}

func cleanCommands(commands []string) []string {
	out := []string{}
	for _, command := range commands {
		if command = strings.TrimSpace(command); command != "" {
			out = append(out, command)
		}
	}
	return out
}
