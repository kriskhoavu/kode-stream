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
	settings.DefaultProvider = "codex"
	if got := settings.ProviderFor("other"); got != "claude" {
		t.Fatalf("a workspace without its own provider starts with Claude, got %q", got)
	}
	if got := settings.ProviderFor(""); got != "codex" {
		t.Fatalf("a session outside a workspace starts with the machine's default, got %q", got)
	}
	claude := settings.Providers["claude"]
	claude.Enabled = false
	settings.Providers["claude"] = claude
	if got := settings.ProviderFor("other"); got != "codex" {
		t.Fatalf("with Claude turned off, the machine's default, got %q", got)
	}
	if _, kept := settings.WorkspaceProviders["blank"]; kept {
		t.Fatal("a blank provider follows the default")
	}
	claude.Enabled = true
	settings.Providers["claude"] = claude
	if err := Validate(settings); err != nil {
		t.Fatal(err)
	}
	settings.WorkspaceProviders["sandbox"] = "unknown"
	if err := Validate(settings); err == nil {
		t.Fatal("an unknown provider must be refused")
	}
}
