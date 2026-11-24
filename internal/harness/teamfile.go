package harness

import (
	"context"
	"log"
	"strings"

	"kode-stream/internal/harness/application/procedure"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/workspace/settingsfile"
)

func (s *Service) WorkspaceFile(ctx context.Context, workspaceID string) (settingsfile.Harness, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return settingsfile.Harness{}, err
	}
	workspace, _ := settings.Workspace(workspaceID)
	own := settings.For(workspaceID)
	enabled := workspace.Enabled == nil || *workspace.Enabled
	ask := settings.QuestionGate(domain.Task{WorkspaceID: workspaceID})
	out := settingsfile.Harness{Enabled: &enabled, AutoStart: &own.AutoStart, Review: &own.RequireReview, AskQuestions: &ask,
		Tests: nonNil(workspace.VerificationCommands), Scope: nonNil(workspace.Scope), Models: map[domain.Role]settingsfile.Model{}, Procedure: orEmpty(workspace.Procedure), Sources: []settingsfile.Source{}}
	for _, role := range domain.Roles {
		policy := own.ModelPolicy.Roles[role]
		out.Models[role] = settingsfile.Model{Engine: policy.Engine, Model: policy.Model, HardModel: policy.HighDifficultyModel, MaxTurns: policy.MaxTurns}
	}
	if jira := workspace.Jira; jira != nil {
		out.Sources = append(out.Sources, settingsfile.Source{Kind: string(domain.SourceJira), Enabled: jira.Enabled, Query: jira.JQL})
	}
	if github := workspace.GitHub; github != nil {
		out.Sources = append(out.Sources, settingsfile.Source{Kind: string(domain.SourceGitHub), Enabled: github.Enabled, Query: github.Repo, Labels: github.Labels, TokenEnvVar: github.TokenEnvVar})
	}
	if bitbucket := workspace.Bitbucket; bitbucket != nil {
		out.Sources = append(out.Sources, settingsfile.Source{Kind: string(domain.SourceBitbucket), Enabled: bitbucket.Enabled, Query: bitbucket.Workspace + "/" + bitbucket.Repo, TokenEnvVar: bitbucket.TokenEnvVar})
	}
	if local := workspace.Local; local != nil {
		out.Sources = append(out.Sources, settingsfile.Source{Kind: string(domain.SourceLocal), Enabled: local.Enabled, Query: local.Dir})
	}
	return out, nil
}

func (s *Service) ImportWorkspaceFile(ctx context.Context, workspaceID string, file settingsfile.Harness, parts []string, actor domain.Actor) error {
	if err := s.mutable(); err != nil {
		return err
	}
	settings := s.settingsNow(ctx)
	index := -1
	for i := range settings.Workspaces {
		if settings.Workspaces[i].WorkspaceID == workspaceID {
			index = i
		}
	}
	if index < 0 {
		settings.Workspaces = append(settings.Workspaces, domain.WorkspaceSettings{WorkspaceID: workspaceID})
		index = len(settings.Workspaces) - 1
	}
	workspace := &settings.Workspaces[index]
	for _, part := range parts {
		switch part {
		case settingsfile.PartPipeline:
			workspace.Enabled, workspace.AutoStart, workspace.RequireReview, workspace.AskQuestions = file.Enabled, file.AutoStart, file.Review, file.AskQuestions
		case settingsfile.PartTests:
			workspace.VerificationCommands = cleanCommands(file.Tests)
		case settingsfile.PartScope:
			workspace.Scope = cleanCommands(file.Scope)
		case settingsfile.PartModels:
			workspace.Models = map[domain.Role]domain.RoleModel{}
			for role, model := range file.Models {
				workspace.Models[role] = domain.RoleModel{Engine: model.Engine, Model: model.Model, HighDifficultyModel: model.HardModel, MaxTurns: model.MaxTurns}
			}
		case settingsfile.PartProcedure:
			workspace.Procedure = file.Procedure
		case settingsfile.PartSources:
			previous := workspace.Bitbucket
			workspace.Jira, workspace.GitHub, workspace.Bitbucket, workspace.Local = nil, nil, nil, nil
			for _, source := range file.Sources {
				query := strings.TrimSpace(source.Query)
				switch domain.SourceKind(source.Kind) {
				case domain.SourceJira:
					workspace.Jira = &domain.JiraSourceSettings{Enabled: source.Enabled, JQL: query}
				case domain.SourceGitHub:
					workspace.GitHub = &domain.GitHubSourceSettings{Enabled: source.Enabled, Repo: query, Labels: nonNil(source.Labels), TokenEnvVar: source.TokenEnvVar}
				case domain.SourceBitbucket:
					owner, repo, _ := strings.Cut(query, "/")
					bitbucket := &domain.BitbucketSourceSettings{Enabled: source.Enabled, Workspace: owner, Repo: repo, TokenEnvVar: source.TokenEnvVar, Cloud: true}
					if previous != nil {
						bitbucket.Cloud, bitbucket.BaseURL = previous.Cloud, previous.BaseURL
					}
					workspace.Bitbucket = bitbucket
				case domain.SourceLocal:
					workspace.Local = &domain.LocalSourceSettings{Enabled: source.Enabled, Dir: query}
				}
			}
		}
	}
	_, err := s.SaveSettings(ctx, settings, settings.Revision, actor)
	return err
}

func (s *Service) MoveProcedureFiles(ctx context.Context) {
	if s.workspaces == nil || s.mutable() != nil {
		return
	}
	workspaces, err := s.workspaces.List()
	if err != nil {
		return
	}
	settings := s.settingsNow(ctx)
	moved := false
	for _, config := range workspaces {
		current, _ := settings.Workspace(config.ID)
		if current.ProcedureMoved || len(current.Procedure) > 0 {
			continue
		}
		steps, err := procedure.Load(config.Path)
		if err != nil {
			log.Printf("harness: workspace %s keeps no role skills: %v", config.ID, err)
			continue
		}
		if len(steps) == 0 {
			continue
		}
		index := -1
		for i := range settings.Workspaces {
			if settings.Workspaces[i].WorkspaceID == config.ID {
				index = i
			}
		}
		if index < 0 {
			settings.Workspaces = append(settings.Workspaces, domain.WorkspaceSettings{WorkspaceID: config.ID})
			index = len(settings.Workspaces) - 1
		}
		settings.Workspaces[index].Procedure = steps
		settings.Workspaces[index].ProcedureMoved = true
		moved = true
	}
	if !moved {
		return
	}
	if _, err := s.SaveSettings(ctx, settings, settings.Revision, domain.Actor{ActorID: "harness", DisplayName: "Kode Stream"}); err != nil {
		log.Printf("harness: moving role skills into the settings: %v", err)
	}
}

// Runs on start, after MoveProcedureFiles.
func (s *Service) OwnWorkspaceModels(ctx context.Context) {
	if s.workspaces == nil || s.mutable() != nil {
		return
	}
	workspaces, err := s.workspaces.List()
	if err != nil {
		return
	}
	ids := make([]string, 0, len(workspaces))
	for _, config := range workspaces {
		ids = append(ids, config.ID)
	}
	settings := s.settingsNow(ctx)
	if !settings.OwnModels(ids) {
		return
	}
	if _, err := s.SaveSettings(ctx, settings, settings.Revision, domain.Actor{ActorID: "harness", DisplayName: "Kode Stream"}); err != nil {
		log.Printf("harness: giving each workspace its own models: %v", err)
	}
}
