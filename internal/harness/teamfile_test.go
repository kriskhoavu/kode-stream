package harness

import (
	"context"
	"testing"

	"kode-stream/internal/harness/domain"
	"kode-stream/internal/workspace/settingsfile"
)

func TestAWorkspaceExportsAndImportsChosenParts(t *testing.T) {
	ctx := context.Background()
	fixture := newPipeline(t, happyScript, func(settings *domain.Settings) {
		settings.Workspaces[0].Jira = &domain.JiraSourceSettings{Enabled: true, JQL: "project = PM"}
	})
	exported, err := fixture.service.WorkspaceFile(ctx, "ws")
	if err != nil {
		t.Fatal(err)
	}
	if len(exported.Tests) != 1 || exported.Models[domain.RoleCoder].Model != "sonnet" || len(exported.Sources) != 1 || *exported.Review {
		t.Fatalf("exported = %#v", exported)
	}
	review := true
	file := settingsfile.Harness{Review: &review, Tests: []string{"make verify"},
		Models:  map[domain.Role]settingsfile.Model{domain.RoleCoder: {Model: "haiku"}},
		Sources: []settingsfile.Source{{Kind: "local", Enabled: true, Query: "tasks"}}}
	if err := fixture.service.ImportWorkspaceFile(ctx, "ws", file, []string{settingsfile.PartPipeline, settingsfile.PartModels}, domain.Actor{}); err != nil {
		t.Fatal(err)
	}
	settings, _ := fixture.service.Settings(ctx)
	workspace, _ := settings.Workspace("ws")
	if !settings.For("ws").RequireReview || settings.For("ws").ModelPolicy.Roles[domain.RoleCoder].Model != "haiku" {
		t.Fatalf("imported parts were not applied: %#v", workspace)
	}
	if workspace.VerificationCommands[0] != "true" || workspace.Jira == nil || workspace.Local != nil {
		t.Fatalf("parts not named changed: %#v", workspace)
	}
}

func TestHarnessYamlMovesIntoTheSettingsOnce(t *testing.T) {
	ctx := context.Background()
	fixture := newPipeline(t, happyScript)
	writeIn(t, fixture.workspace.Path, ".kode-stream/harness.yaml", "version: 1\nroles:\n  planner: { skill: feature-planning, track: Plan }\n")
	fixture.service.MoveProcedureFiles(ctx)
	settings, _ := fixture.service.Settings(ctx)
	workspace, _ := settings.Workspace("ws")
	if workspace.Procedure[domain.RolePlanner].Skill != "feature-planning" || !workspace.ProcedureMoved {
		t.Fatalf("workspace = %#v", workspace)
	}
	workspace.Procedure = nil
	settings.Workspaces[0] = workspace
	if _, err := fixture.service.SaveSettings(ctx, settings, settings.Revision, domain.Actor{}); err != nil {
		t.Fatal(err)
	}
	fixture.service.MoveProcedureFiles(ctx)
	settings, _ = fixture.service.Settings(ctx)
	if again, _ := settings.Workspace("ws"); len(again.Procedure) != 0 {
		t.Fatalf("harness.yaml was read again: %#v", again.Procedure)
	}
}

func TestEachWorkspaceGetsItsOwnModelsOnce(t *testing.T) {
	ctx := context.Background()
	fixture := newPipeline(t, happyScript, func(settings *domain.Settings) {
		coder := settings.ModelPolicy.Roles[domain.RoleCoder]
		coder.Model = "haiku"
		settings.ModelPolicy.Roles[domain.RoleCoder] = coder
		settings.Workspaces[0].Models = map[domain.Role]domain.RoleModel{domain.RolePlanner: {Model: "sonnet"}}
	})
	fixture.service.OwnWorkspaceModels(ctx)
	settings, _ := fixture.service.Settings(ctx)
	workspace, _ := settings.Workspace("ws")
	if !settings.WorkspaceModels || len(workspace.Models) != len(domain.Roles) {
		t.Fatalf("models were not moved: %#v", workspace.Models)
	}
	if workspace.Models[domain.RolePlanner].Model != "sonnet" || workspace.Models[domain.RoleCoder].Model != "haiku" || workspace.Models[domain.RoleCoder].HighDifficultyModel != "opus" {
		t.Fatalf("workspace models = %#v", workspace.Models)
	}
	if settings.ModelPolicy.Roles[domain.RoleCoder].Model != "sonnet" {
		t.Fatalf("defaults = %#v", settings.ModelPolicy.Roles[domain.RoleCoder])
	}
	coder := workspace.Models[domain.RoleCoder]
	coder.HighDifficultyModel = ""
	workspace.Models[domain.RoleCoder] = coder
	settings.Workspaces[0] = workspace
	if _, err := fixture.service.SaveSettings(ctx, settings, settings.Revision, domain.Actor{}); err != nil {
		t.Fatal(err)
	}
	fixture.service.OwnWorkspaceModels(ctx)
	settings, _ = fixture.service.Settings(ctx)
	if got := settings.For("ws").ModelPolicy.ModelFor(domain.RoleCoder, "high"); got != "haiku" {
		t.Fatalf("hard work runs with %q, want the workspace's own haiku", got)
	}
}
