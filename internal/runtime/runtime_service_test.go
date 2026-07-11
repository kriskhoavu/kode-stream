package runtime

import (
	"strings"
	"testing"

	"kode-stream/internal/common/models"
)

func TestNormalizeRuntimeConfigDefaultsAutomation(t *testing.T) {
	repo := t.TempDir()
	config, err := NormalizeRuntimeConfig(&models.WorkspaceRuntimeConfig{
		Type: models.RuntimeTypeCustom,
		Commands: models.RuntimeCommandSet{
			Up:   "npm run dev",
			Down: "true",
			Verify: models.RuntimeVerifyCommands{
				Smoke: "npm test",
			},
		},
		Automation: &models.RuntimeAutomationConfig{Enabled: true, RepositoryPath: repo},
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.Automation == nil || config.Automation.Runner != models.AutomationRunnerCypress {
		t.Fatalf("automation runner = %#v", config.Automation)
	}
	if config.Automation.DefaultEnvironment != "local" {
		t.Fatalf("environment = %q", config.Automation.DefaultEnvironment)
	}
	if !strings.Contains(config.Automation.CommandTemplate, "cypress run") {
		t.Fatalf("command template = %q", config.Automation.CommandTemplate)
	}
	if len(config.Automation.ArtifactPaths) == 0 {
		t.Fatalf("artifact paths should default")
	}
}

func TestNormalizeRuntimeConfigValidatesEnabledAutomation(t *testing.T) {
	_, err := NormalizeRuntimeConfig(&models.WorkspaceRuntimeConfig{
		Type: models.RuntimeTypeCustom,
		Commands: models.RuntimeCommandSet{
			Up:   "npm run dev",
			Down: "true",
			Verify: models.RuntimeVerifyCommands{
				Smoke: "npm test",
			},
		},
		Automation: &models.RuntimeAutomationConfig{Enabled: true, Runner: "other"},
	})
	if err == nil || !strings.Contains(err.Error(), "runner") {
		t.Fatalf("err = %v", err)
	}
}
