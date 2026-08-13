package verification

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"kode-stream/internal/common/models"
)

const maxFingerprintFileBytes int64 = 10 * 1024 * 1024
const maxFingerprintUntrackedBytes int64 = 50 * 1024 * 1024
const maxFingerprintGitOutputBytes int64 = 8 * 1024 * 1024

type Fingerprinter interface {
	Fingerprint(models.WorkspaceConfig, Job) (RepositoryFingerprint, error)
}

type ContextFingerprinter interface {
	FingerprintContext(context.Context, models.WorkspaceConfig, Job) (RepositoryFingerprint, error)
}

type GitFingerprinter struct{}

func (GitFingerprinter) Fingerprint(workspace models.WorkspaceConfig, job Job) (RepositoryFingerprint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return GitFingerprinter{}.FingerprintContext(ctx, workspace, job)
}

func (GitFingerprinter) FingerprintContext(ctx context.Context, workspace models.WorkspaceConfig, job Job) (RepositoryFingerprint, error) {
	root := strings.TrimSpace(workspace.Path)
	if root == "" {
		return RepositoryFingerprint{}, errors.New("workspace path is unavailable")
	}
	branch, err := gitText(ctx, root, "branch", "--show-current")
	if err != nil {
		return RepositoryFingerprint{}, err
	}
	if branch == "" {
		branch = "HEAD"
	}
	commit, err := gitText(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return RepositoryFingerprint{}, err
	}
	hasher := sha256.New()
	writeFingerprintPart(hasher, "branch", []byte(branch))
	writeFingerprintPart(hasher, "commit", []byte(commit))
	pathspec := []string{"--", ".", ":(exclude,glob).artifacts/verification/**"}
	if err := hashGitOutput(ctx, hasher, root, "staged", append([]string{"diff", "--cached", "--binary", "--no-ext-diff"}, pathspec...)...); err != nil {
		return RepositoryFingerprint{}, err
	}
	if err := hashGitOutput(ctx, hasher, root, "working", append([]string{"diff", "--binary", "--no-ext-diff"}, pathspec...)...); err != nil {
		return RepositoryFingerprint{}, err
	}
	untracked, err := gitBytes(ctx, root, append([]string{"ls-files", "--others", "--exclude-standard", "-z"}, pathspec...)...)
	if err != nil {
		return RepositoryFingerprint{}, err
	}
	var total int64
	for _, raw := range bytes.Split(untracked, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		rel := string(raw)
		full, err := safeFingerprintPath(root, rel)
		if err != nil {
			return RepositoryFingerprint{}, err
		}
		info, err := os.Lstat(full)
		if err != nil {
			return RepositoryFingerprint{}, err
		}
		if !info.Mode().IsRegular() || info.Size() > maxFingerprintFileBytes || total+info.Size() > maxFingerprintUntrackedBytes {
			return RepositoryFingerprint{}, fmt.Errorf("untracked content is outside fingerprint limits")
		}
		total += info.Size()
		writeFingerprintPart(hasher, "untracked-path", []byte(filepath.ToSlash(rel)))
		file, err := os.Open(full)
		if err != nil {
			return RepositoryFingerprint{}, err
		}
		_, copyErr := io.Copy(hasher, file)
		closeErr := file.Close()
		if copyErr != nil {
			return RepositoryFingerprint{}, copyErr
		}
		if closeErr != nil {
			return RepositoryFingerprint{}, closeErr
		}
	}
	config, err := json.Marshal(struct {
		Runtime       *models.WorkspaceRuntimeConfig `json:"runtime"`
		Mode          JobMode                        `json:"mode"`
		Profile       string                         `json:"profile"`
		Environment   string                         `json:"environment"`
		DisplayMode   models.AutomationDisplayMode   `json:"displayMode"`
		SelectedSpecs []string                       `json:"selectedSpecs"`
	}{workspace.Runtime, job.Mode, string(job.Profile), job.Environment, job.DisplayMode, job.SelectedSpecs})
	if err != nil {
		return RepositoryFingerprint{}, err
	}
	writeFingerprintPart(hasher, "verification-config", config)
	return RepositoryFingerprint{Value: hex.EncodeToString(hasher.Sum(nil)), Branch: branch, Commit: commit}, nil
}

func gitText(ctx context.Context, root string, args ...string) (string, error) {
	data, err := gitBytes(ctx, root, args...)
	return strings.TrimSpace(string(data)), err
}

func gitBytes(ctx context.Context, root string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	output := &limitedOutput{limit: maxFingerprintGitOutputBytes}
	command.Stdout = output
	command.Stderr = output
	err := command.Run()
	data := output.Bytes()
	if output.exceeded {
		return nil, errors.New("git fingerprint output exceeds limit")
	}
	if err != nil {
		return nil, fmt.Errorf("git fingerprint command failed: %w: %s", err, strings.TrimSpace(string(data)))
	}
	return data, nil
}

type limitedOutput struct {
	bytes.Buffer
	limit    int64
	exceeded bool
}

func (w *limitedOutput) Write(data []byte) (int, error) {
	if int64(w.Len()+len(data)) > w.limit {
		remaining := int(w.limit - int64(w.Len()))
		if remaining > 0 {
			_, _ = w.Buffer.Write(data[:remaining])
		}
		w.exceeded = true
		return len(data), nil
	}
	return w.Buffer.Write(data)
}

func hashGitOutput(ctx context.Context, hasher hash.Hash, root, label string, args ...string) error {
	writeFingerprintPart(hasher, label, nil)
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	var stderr bytes.Buffer
	command.Stdout = hasher
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("git fingerprint command failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func writeFingerprintPart(writer io.Writer, label string, value []byte) {
	_, _ = io.WriteString(writer, label)
	_, _ = writer.Write([]byte{0})
	_, _ = writer.Write(value)
	_, _ = writer.Write([]byte{0})
}

func safeFingerprintPath(root, rel string) (string, error) {
	full := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
	relative, err := filepath.Rel(root, full)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("untracked path escapes workspace")
	}
	return full, nil
}
