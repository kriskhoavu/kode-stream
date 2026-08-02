package verification

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"kode-stream/internal/common/models"
	appruntime "kode-stream/internal/runtime"
)

func TestGitFingerprintChangesForRepositoryAndVerificationConfiguration(t *testing.T) {
	root := verificationGitWorkspace(t)
	workspace := models.WorkspaceConfig{ID: "workspace-1", Path: root, Runtime: verificationRuntime("true", nil)}
	job := Job{Mode: JobModeRuntime, Profile: appruntime.VerifyProfileSmoke}
	fingerprinter := GitFingerprinter{}
	first, err := fingerprinter.Fingerprint(workspace, job)
	if err != nil {
		t.Fatal(err)
	}
	second, err := fingerprinter.Fingerprint(workspace, job)
	if err != nil || second.Value != first.Value || first.Branch != "main" || first.Commit == "" {
		t.Fatalf("first=%#v second=%#v err=%v", first, second, err)
	}
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	untracked, _ := fingerprinter.Fingerprint(workspace, job)
	if untracked.Value == first.Value {
		t.Fatal("untracked content did not change fingerprint")
	}
	verificationGit(t, root, "add", "untracked.txt")
	staged, _ := fingerprinter.Fingerprint(workspace, job)
	if staged.Value == untracked.Value || staged.Value == first.Value {
		t.Fatal("staged state did not change fingerprint")
	}
	workspace.Runtime.Commands.Verify.Smoke = "printf changed"
	configured, _ := fingerprinter.Fingerprint(workspace, job)
	if configured.Value == staged.Value {
		t.Fatal("verification configuration did not change fingerprint")
	}
}

func TestVerificationFreshnessBecomesStaleAndDetectsDuringRunChanges(t *testing.T) {
	root := verificationGitWorkspace(t)
	runtimeConfig := verificationRuntime("true", nil)
	runtimeConfig.Commands.Down = "true"
	workspace := models.WorkspaceConfig{ID: "workspace-1", Name: "Workspace", Path: root, BaselineBranch: "main", Sources: []string{"plans"}, Runtime: runtimeConfig}
	service := NewService(verificationRegistry(t, workspace), appruntime.NewService())
	t.Cleanup(service.Close)
	job, err := service.Start(workspace.ID, CreateInput{})
	if err != nil {
		t.Fatal(err)
	}
	job = waitVerificationJob(t, service, workspace.ID, job.ID)
	if job.Freshness != FreshnessFresh || job.StartFingerprint == nil || job.FinishFingerprint == nil {
		t.Fatalf("job=%#v", job)
	}
	if err := os.WriteFile(filepath.Join(root, "after.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	job, _ = service.Get(workspace.ID, job.ID)
	if job.Freshness != FreshnessStale {
		t.Fatalf("freshness=%s", job.Freshness)
	}

	workspace.Runtime = verificationRuntime("sleep 0.2", nil)
	workspace.Runtime.Commands.Down = "true"
	if _, err := service.registry.SetRuntime(workspace.ID, workspace.Runtime); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "after.txt")); err != nil {
		t.Fatal(err)
	}
	running, err := service.Start(workspace.ID, CreateInput{})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(root, "during.txt"), []byte("changed during verification"), 0o644); err != nil {
		t.Fatal(err)
	}
	running = waitVerificationJob(t, service, workspace.ID, running.ID)
	if running.Freshness != FreshnessInconclusive || running.StartFingerprint == nil || running.FinishFingerprint == nil || running.StartFingerprint.Value == running.FinishFingerprint.Value {
		t.Fatalf("job=%#v", running)
	}
}

func verificationGitWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	verificationGit(t, root, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("seed"), 0o644); err != nil {
		t.Fatal(err)
	}
	verificationGit(t, root, "add", "tracked.txt")
	command := exec.Command("git", "-C", root, "commit", "-m", "seed")
	command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	return root
}

func verificationGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}
