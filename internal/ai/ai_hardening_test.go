package ai

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCapabilityDiscoverySkipsSymlinkEscapesAndHonorsCancellation(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, ".skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "safe.md"), []byte("# Safe"), 0o644); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(external, []byte("# Private capability"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "escape.md")); err != nil {
		t.Fatal(err)
	}
	skills, _ := discoverProviderCapabilitiesContext(context.Background(), "unknown", workspace)
	if len(skills) != 1 || skills[0].SourcePath != ".skills/safe.md" {
		t.Fatalf("skills=%#v", skills)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	skills, agents := discoverProviderCapabilitiesContext(ctx, "unknown", workspace)
	if len(skills) != 0 || len(agents) != 0 {
		t.Fatalf("cancelled discovery returned skills=%#v agents=%#v", skills, agents)
	}
}

func TestDurablePublicationRestoresPriorAndNonexistentGenerationsOnDirectoryFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.yaml")
	if err := os.WriteFile(path, []byte("prior"), 0o640); err != nil {
		t.Fatal(err)
	}
	fail := durableFileOps{syncDirectory: func(string) error { return errors.New("directory sync failed") }, remove: os.Remove}
	if err := publishDurableFileWith(path, []byte("next"), fail); err == nil {
		t.Fatal("expected durable publication failure")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "prior" {
		t.Fatalf("data=%q err=%v", data, err)
	}
	fresh := filepath.Join(dir, "new.yaml")
	if err := publishDurableFileWith(fresh, []byte("next"), fail); err == nil {
		t.Fatal("expected first publication failure")
	}
	if _, err := os.Stat(fresh); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed first publication left a file: %v", err)
	}
}

func TestCapabilityDiscoveryRejectsSymlinkedRootOutsideWorkspaceAnchor(t *testing.T) {
	workspace := t.TempDir()
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "private.md"), []byte("# Private"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(workspace, ".skills")); err != nil {
		t.Fatal(err)
	}
	skills, agents := discoverProviderCapabilitiesContext(context.Background(), "unknown", workspace)
	if len(skills) != 0 || len(agents) != 0 {
		t.Fatalf("escaped root was discovered: skills=%#v agents=%#v", skills, agents)
	}
}

func TestCapabilityDiscoveryBoundsFanoutAndOversizedMetadataDeterministically(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, ".skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for index := range maxCapabilityEntries + 20 {
		name := fmt.Sprintf("skill-%03d.md", index)
		if err := os.WriteFile(filepath.Join(root, name), []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "huge.md"), make([]byte, maxCapabilityFileBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	skills, _ := discoverProviderCapabilitiesContext(context.Background(), "unknown", workspace)
	if len(skills) != maxCapabilityResults {
		t.Fatalf("skills=%d, want %d", len(skills), maxCapabilityResults)
	}
	for index := 1; index < len(skills); index++ {
		if capabilitySortKey(skills[index-1]) > capabilitySortKey(skills[index]) {
			t.Fatalf("capabilities are not stable: %#v", skills)
		}
	}
}

func TestAIYAMLPersistencePreservesModeAndRejectsOversizedGeneration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	if err := os.WriteFile(path, []byte("providers: {}\nterminals: {}\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	repository := NewSettingsRepository(path)
	if _, err := repository.Save(Settings{Providers: map[string]LaunchTemplate{}, Terminals: map[string]LaunchTemplate{}}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode=%#o", info.Mode().Perm())
	}
	if err := os.WriteFile(path, make([]byte, maxAIYAMLBytes+1), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Load(); err == nil {
		t.Fatal("oversized YAML was accepted")
	}
}

func TestAIYAMLPersistenceRejectsTrailingDocuments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	if err := os.WriteFile(path, []byte("providers: {}\nterminals: {}\n---\nproviders: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSettingsRepository(path).Load(); err == nil {
		t.Fatal("trailing settings document was accepted")
	}
	recordPath := filepath.Join(t.TempDir(), "records.yaml")
	if err := os.WriteFile(recordPath, []byte("[]\n---\n[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileSessionRecordRepository(recordPath).Snapshot(); err == nil {
		t.Fatal("trailing record document was accepted")
	}
}

func TestSettingsRejectTemplateMapFanout(t *testing.T) {
	providers := make(map[string]LaunchTemplate, maxTemplatesPerKind+1)
	for index := range maxTemplatesPerKind + 1 {
		providers[fmt.Sprintf("provider-%d", index)] = LaunchTemplate{Enabled: true, Executable: "tool"}
	}
	if err := Validate(Settings{Providers: providers, Terminals: map[string]LaunchTemplate{}}); err == nil {
		t.Fatal("template fanout was accepted")
	}
}

func TestSettingsAndSessionRecordsKeepPriorGenerationForEveryPublicationPhaseFailure(t *testing.T) {
	for _, phase := range []string{"write", "sync", "close", "rename", "directory"} {
		t.Run(phase, func(t *testing.T) {
			settingsPath := filepath.Join(t.TempDir(), "settings.yaml")
			settings := NewSettingsRepository(settingsPath)
			baseline := Settings{Providers: map[string]LaunchTemplate{"one": {Enabled: true, Executable: "one"}}, Terminals: map[string]LaunchTemplate{}}
			if _, err := settings.Save(baseline); err != nil {
				t.Fatal(err)
			}
			settings.publish = func(path string, data []byte) error { return publishDurableFileWith(path, data, failureOps(phase)) }
			if _, err := settings.Save(Settings{Providers: map[string]LaunchTemplate{"two": {Enabled: true, Executable: "two"}}, Terminals: map[string]LaunchTemplate{}}); err == nil {
				t.Fatal("expected settings failure")
			}
			loaded, err := NewSettingsRepository(settingsPath).Load()
			if err != nil || loaded.Providers["one"].Executable != "one" {
				t.Fatalf("settings=%#v err=%v", loaded, err)
			}

			recordPath := filepath.Join(t.TempDir(), "records.yaml")
			records := NewFileSessionRecordRepository(recordPath)
			now := time.Now().UTC()
			first := SessionRecord{ID: "one", WorkspaceID: "workspace", Provider: "codex", Intent: "test", RequestedBranch: "main", State: StateExited, StartedAt: now, LastKnownAt: now}
			if _, err := records.Upsert(first); err != nil {
				t.Fatal(err)
			}
			records.publish = func(path string, data []byte) error { return publishDurableFileWith(path, data, failureOps(phase)) }
			second := first
			second.ID = "two"
			if _, err := records.Upsert(second); err == nil {
				t.Fatal("expected record failure")
			}
			loadedRecords, err := NewFileSessionRecordRepository(recordPath).Snapshot()
			if err != nil || len(loadedRecords) != 1 || loadedRecords[0].ID != "one" {
				t.Fatalf("records=%#v err=%v", loadedRecords, err)
			}
		})
	}
}

func failureOps(phase string) durableFileOps {
	fail := func(name string) error {
		if phase == name {
			return errors.New(name + " failed")
		}
		return nil
	}
	return durableFileOps{before: fail, syncDirectory: func(directory string) error { return fail("directory") }, remove: os.Remove}
}
