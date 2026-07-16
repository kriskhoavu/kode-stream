package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type GitMetadata interface {
	WorkspaceRoot(string) (string, error)
	CurrentBranch(string) (string, error)
	RemoteURL(string) (string, error)
}

type WorkspaceMetadata struct {
	Name             string   `json:"name"`
	BaselineBranch   string   `json:"baselineBranch"`
	Sources          []string `json:"sources"`
	RemoteURL        string   `json:"remoteUrl,omitempty"`
	LocalRootLabel   string   `json:"localRootLabel,omitempty"`
	PublishedSummary bool     `json:"publishedSummary"`
	ScanStatus       string   `json:"scanStatus,omitempty"`
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
	return WorkspaceMetadata{
		Name:             filepath.Base(root),
		BaselineBranch:   branch,
		Sources:          detectSources(root),
		RemoteURL:        gitRemoteURL(root, git),
		LocalRootLabel:   root,
		PublishedSummary: true,
		ScanStatus:       "published",
	}, nil
}

func PublishWorkspaceMetadata(ctx context.Context, httpClient *http.Client, cloudURL, token string, metadata WorkspaceMetadata) error {
	if strings.TrimSpace(metadata.Name) == "" {
		return nil
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
	candidates := []string{"plans", "docs", "items"}
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
