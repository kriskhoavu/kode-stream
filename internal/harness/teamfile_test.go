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
	// Parts the import did not name stay as they were.
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
	// Cleared by a person, the role skills stay cleared.
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
