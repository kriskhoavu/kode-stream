package workspace

import (
	"testing"

	"kode-stream/internal/common/models"
)

func TestProviderAxesSeparateRuntimeWorkspaceAndDatastore(t *testing.T) {
	local := ProviderAxes(models.RuntimeModeLocal, models.AppStateDatastoreSQLite, models.WorkspaceConfig{})
	if local.Topology != models.DeploymentTopologyLocalApplication || local.ContentProvider != models.WorkspaceContentProviderLocalCheckout || local.ExecutionProvider != models.WorkspaceExecutionProviderLocalProcess || local.Datastore != models.AppStateDatastoreSQLite {
		t.Fatalf("local axes = %#v", local)
	}

	agent := ProviderAxes(models.RuntimeModeCloud, models.AppStateDatastorePostgres, models.WorkspaceConfig{AccessMode: models.WorkspaceAccessModeAgentBacked, Location: models.WorkspaceLocationCloudAgent})
	if agent.Topology != models.DeploymentTopologyCloudControlPlane || agent.ContentProvider != models.WorkspaceContentProviderAgentCheckout || agent.ExecutionProvider != models.WorkspaceExecutionProviderCloudAgent || agent.Datastore != models.AppStateDatastorePostgres {
		t.Fatalf("agent axes = %#v", agent)
	}

	snapshot := ProviderAxes(models.RuntimeModeCloud, models.AppStateDatastorePostgres, models.WorkspaceConfig{AccessMode: models.WorkspaceAccessModeRemoteSnapshot})
	if snapshot.ContentProvider != models.WorkspaceContentProviderProviderSnapshot || snapshot.ExecutionProvider != models.WorkspaceExecutionProviderNone {
		t.Fatalf("snapshot axes = %#v", snapshot)
	}
}

func TestResolveActionCapabilitiesAvailableForWritableLocalCheckout(t *testing.T) {
	capabilities := ResolveActionCapabilities(ActionCapabilityInput{
		Axes:               ProviderAxes(models.RuntimeModeLocal, models.AppStateDatastoreDataDir, models.WorkspaceConfig{}),
		ContentAvailable:   true,
		ExecutionAvailable: true,
		Writable:           true,
		CurrentBranch:      "feature/PM-037",
		TargetBranch:       "feature/PM-037",
	})
	for _, action := range []models.WorkspaceAction{models.WorkspaceActionLayoutMove, models.WorkspaceActionRepositoryRead, models.WorkspaceActionGitStatus, models.WorkspaceActionTerminalLaunch, models.WorkspaceActionVerificationRun} {
		if capabilities[action].State != models.ActionCapabilityAvailable {
			t.Fatalf("%s = %#v", action, capabilities[action])
		}
	}
}

func TestResolveActionCapabilitiesDistinguishesBlockedStates(t *testing.T) {
	tests := []struct {
		name   string
		input  ActionCapabilityInput
		action models.WorkspaceAction
		state  models.ActionCapabilityState
		reason string
	}{
		{
			name: "unsupported snapshot execution",
			input: ActionCapabilityInput{
				Axes:               ProviderAxes(models.RuntimeModeCloud, models.AppStateDatastorePostgres, models.WorkspaceConfig{AccessMode: models.WorkspaceAccessModeRemoteSnapshot}),
				ContentAvailable:   true,
				ExecutionAvailable: false,
				Writable:           false,
			},
			action: models.WorkspaceActionTerminalLaunch,
			state:  models.ActionCapabilityUnsupported,
			reason: "execution_provider_missing",
		},
		{
			name: "unavailable agent",
			input: ActionCapabilityInput{
				Axes:               ProviderAxes(models.RuntimeModeCloud, models.AppStateDatastorePostgres, models.WorkspaceConfig{AccessMode: models.WorkspaceAccessModeAgentBacked, Location: models.WorkspaceLocationCloudAgent}),
				ContentAvailable:   true,
				ExecutionAvailable: false,
				Writable:           true,
			},
			action: models.WorkspaceActionVerificationRun,
			state:  models.ActionCapabilityUnavailable,
			reason: "execution_provider_unavailable",
		},
		{
			name: "forbidden viewer",
			input: ActionCapabilityInput{
				Axes:               ProviderAxes(models.RuntimeModeLocal, models.AppStateDatastoreDataDir, models.WorkspaceConfig{}),
				Authorization:      map[models.Capability]bool{models.CapabilityRead: true},
				ContentAvailable:   true,
				ExecutionAvailable: true,
				Writable:           true,
			},
			action: models.WorkspaceActionTerminalLaunch,
			state:  models.ActionCapabilityForbidden,
			reason: "terminal_launch_forbidden",
		},
		{
			name: "branch conflict",
			input: ActionCapabilityInput{
				Axes:               ProviderAxes(models.RuntimeModeLocal, models.AppStateDatastoreDataDir, models.WorkspaceConfig{}),
				ContentAvailable:   true,
				ExecutionAvailable: true,
				Writable:           true,
				CurrentBranch:      "main",
				TargetBranch:       "feature/PM-037",
			},
			action: models.WorkspaceActionTerminalLaunch,
			state:  models.ActionCapabilityConflicted,
			reason: "branch_mismatch",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			capability := ResolveActionCapabilities(test.input)[test.action]
			if capability.State != test.state || capability.ReasonCode != test.reason {
				t.Fatalf("capability = %#v", capability)
			}
		})
	}
}
