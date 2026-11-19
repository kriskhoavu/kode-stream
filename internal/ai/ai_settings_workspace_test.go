package ai

import "testing"

func TestAWorkspaceStartsWithItsOwnProvider(t *testing.T) {
	settings := mergeDefaults(Settings{DefaultProvider: "claude", WorkspaceProviders: map[string]string{"sandbox": "codex", "blank": " "}}, "darwin")
	if got := settings.ProviderFor("sandbox"); got != "codex" {
		t.Fatalf("sandbox provider = %q", got)
	}
	if got := settings.ProviderFor("other"); got != "claude" {
		t.Fatalf("other provider = %q", got)
	}
	if _, kept := settings.WorkspaceProviders["blank"]; kept {
		t.Fatal("a blank provider follows the default")
	}
	if err := Validate(settings); err != nil {
		t.Fatal(err)
	}
	settings.WorkspaceProviders["sandbox"] = "unknown"
	if err := Validate(settings); err == nil {
		t.Fatal("an unknown provider must be refused")
	}
}
