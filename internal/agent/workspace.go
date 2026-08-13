package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	MaxWorkspaceNameBytes     = 160
	MaxBranchBytes            = 256
	MaxSourceCount            = 32
	MaxSourceBytes            = 256
	MaxRemoteURLBytes         = 2 << 10
	MaxWorkspaceMetadataBytes = 16 << 10
)

type GitMetadata interface {
	WorkspaceRoot(string) (string, error)
	CurrentBranch(string) (string, error)
	RemoteURL(string) (string, error)
}

type WorkspaceMetadata struct {
	Revision         int64    `json:"revision"`
	Name             string   `json:"name"`
	BaselineBranch   string   `json:"baselineBranch"`
	Sources          []string `json:"sources"`
	RemoteURL        string   `json:"remoteUrl,omitempty"`
	LocalRootLabel   string   `json:"localRootLabel,omitempty"`
	PublishedSummary bool     `json:"publishedSummary"`
	ScanStatus       string   `json:"scanStatus,omitempty"`
}

// ValidateWorkspaceMetadata is the shared Agent wire-contract validator. It
// intentionally rejects ambiguous input rather than rewriting it at either end
// of the protocol.
func ValidateWorkspaceMetadata(metadata WorkspaceMetadata) (WorkspaceMetadata, error) {
	metadata.Name = strings.TrimSpace(metadata.Name)
	metadata.BaselineBranch = strings.TrimSpace(metadata.BaselineBranch)
	metadata.RemoteURL = strings.TrimSpace(metadata.RemoteURL)
	metadata.LocalRootLabel = strings.TrimSpace(metadata.LocalRootLabel)
	metadata.ScanStatus = strings.TrimSpace(metadata.ScanStatus)
	if metadata.Revision < 0 {
		return WorkspaceMetadata{}, errors.New("workspace revision is invalid")
	}
	if metadata.Name == "" || len(metadata.Name) > MaxWorkspaceNameBytes {
		return WorkspaceMetadata{}, errors.New("workspace name is invalid")
	}
	if metadata.BaselineBranch == "" {
		metadata.BaselineBranch = "main"
	}
	if len(metadata.BaselineBranch) > MaxBranchBytes || strings.ContainsAny(metadata.BaselineBranch, "\x00\r\n") {
		return WorkspaceMetadata{}, errors.New("workspace branch is invalid")
	}
	if metadata.ScanStatus == "" {
		metadata.ScanStatus = "published"
	}
	if metadata.ScanStatus != "published" && metadata.ScanStatus != "unavailable" {
		return WorkspaceMetadata{}, errors.New("workspace scan status is invalid")
	}
	if len(metadata.RemoteURL) > MaxRemoteURLBytes || hasCredentialURL(metadata.RemoteURL) {
		return WorkspaceMetadata{}, errors.New("workspace remote URL is invalid")
	}
	if len(metadata.LocalRootLabel) > MaxRemoteURLBytes {
		return WorkspaceMetadata{}, errors.New("workspace label is invalid")
	}
	if len(metadata.Sources) > MaxSourceCount {
		return WorkspaceMetadata{}, errors.New("too many workspace sources")
	}
	seen := make(map[string]struct{}, len(metadata.Sources))
	for index, source := range metadata.Sources {
		if !validSource(source) {
			return WorkspaceMetadata{}, errors.New("workspace source is invalid")
		}
		if _, exists := seen[source]; exists {
			return WorkspaceMetadata{}, errors.New("workspace sources must be unique")
		}
		seen[source] = struct{}{}
		metadata.Sources[index] = source
	}
	return metadata, nil
}

func validSource(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || len(value) > MaxSourceBytes || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return false
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func hasCredentialURL(value string) bool {
	if value == "" {
		return false
	}
	// SCP-style Git remotes (git@host:org/repo.git) contain an SSH account, not
	// a reusable provider credential, and remain an approved redacted label.
	if !strings.Contains(value, "://") {
		return strings.Contains(value, "\x00")
	}
	parsed, err := url.Parse(value)
	return err != nil || parsed.User != nil || strings.Contains(value, "\x00")
}

func BuildWorkspaceMetadata(repo string, git GitMetadata) (WorkspaceMetadata, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return WorkspaceMetadata{}, nil
	}
	if git == nil {
		return WorkspaceMetadata{}, fmt.Errorf("git metadata adapter is required")
	}
	root, err := git.WorkspaceRoot(repo)
	if err != nil {
		return WorkspaceMetadata{}, fmt.Errorf("repo is not a Git workspace: %w", err)
	}
	branch, err := git.CurrentBranch(root)
	if err != nil || strings.TrimSpace(branch) == "" {
		branch = "main"
	}
	metadata := WorkspaceMetadata{
		Name:             filepath.Base(root),
		BaselineBranch:   branch,
		Sources:          detectSources(root),
		RemoteURL:        gitRemoteURL(root, git),
		LocalRootLabel:   root,
		PublishedSummary: true,
		ScanStatus:       "published",
	}
	return ValidateWorkspaceMetadata(metadata)
}

func PublishWorkspaceMetadata(ctx context.Context, httpClient *http.Client, cloudURL, token string, metadata WorkspaceMetadata) error {
	metadata, err := ValidateWorkspaceMetadata(metadata)
	if err != nil {
		return err
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	endpoint, err := apiURL(cloudURL, "/api/workspaces/from-agent")
	if err != nil {
		return err
	}
	body, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("workspace metadata publish failed: %s", response.Status)
	}
	return nil
}

func detectSources(root string) []string {
	candidates := []string{"plans", "wiki", "docs", "items"}
	var sources []string
	for _, candidate := range candidates {
		info, err := os.Stat(filepath.Join(root, candidate))
		if err == nil && info.IsDir() {
			sources = append(sources, candidate)
		}
	}
	return sources
}

func gitRemoteURL(root string, git GitMetadata) string {
	remote, err := git.RemoteURL(root)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(remote)
}
