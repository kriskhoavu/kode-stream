package settingsfile

import (
	"strings"
	"testing"

	"kode-stream/internal/harness/domain"
)

func TestParseRefusesWhatAFileMayNotHold(t *testing.T) {
	for name, text := range map[string]string{
		"a tool permission": "version: 1\nharness:\n  models:\n    coder: { model: sonnet, allowedTools: [Bash] }\n",
		"an unknown key":    "version: 1\nharness:\n  permissionMode: default\n",
		"a token value":     "version: 1\nharness:\n  sources:\n    - { kind: github, enabled: true, query: a/b, tokenEnvVar: ghp_secret123 }\n",
		"an unknown engine": "version: 1\nharness:\n  models:\n    coder: { engine: shell }\n",
		"a scope outside":   "version: 1\nharness:\n  scope: [../etc]\n",
		"another version":   "version: 2\n",
		"a bad skill":       "version: 1\nharness:\n  procedure:\n    planner: { skill: Bad Skill }\n",
		"a source twice":    "version: 1\nharness:\n  sources:\n    - { kind: jira, query: a }\n    - { kind: jira, query: b }\n",
	} {
		if _, err := Parse([]byte(text)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestWriteKeepsThePartsAnExportDoesNotName(t *testing.T) {
	dir := t.TempDir()
	review := true
	current := File{Version: 1, AI: &AI{Provider: "claude"}, Harness: &Harness{Review: &review, Tests: []string{"go test ./..."},
		Models:  map[domain.Role]Model{domain.RoleCoder: {Engine: "claudecode", Model: "sonnet", MaxTurns: 120}},
		Sources: []Source{{Kind: "jira", Enabled: true, Query: "project = X"}}}}
	if err := Write(dir, Take(File{}, current, []string{PartTests, PartModels})); err != nil {
		t.Fatal(err)
	}
	file, found, err := Load(dir)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if got := strings.Join(file.Has(), ","); got != "tests,models" {
		t.Fatalf("has = %s", got)
	}
	if err := Write(dir, Take(file, current, []string{PartSources})); err != nil {
		t.Fatal(err)
	}
	file, _, _ = Load(dir)
	if got := strings.Join(file.Has(), ","); got != "tests,models,sources" || file.Harness.Sources[0].Query != "project = X" {
		t.Fatalf("has = %s, file = %#v", got, file.Harness)
	}
	if err := ValidParts([]string{"secrets"}); err == nil {
		t.Fatal("an unknown part must be refused")
	}
}
