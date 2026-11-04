// Package github opens a draft pull request on GitHub: through the REST API
// when a token is set, otherwise through `gh pr create --draft`.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/adapters/credential"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

type Adapter struct {
	BaseURL    string
	HTTPClient *http.Client
	Tokens     credential.Resolver
	// GH runs the gh CLI in dir; nil means exec.
	GH func(ctx context.Context, dir string, args ...string) (string, error)
}

func New() *Adapter {
	return &Adapter{BaseURL: "https://api.github.com", HTTPClient: &http.Client{Timeout: 30 * time.Second}, Tokens: credential.Default()}
}

func (a *Adapter) ID() string { return "github" }

var githubRemote = regexp.MustCompile(`github\.com[:/]([^/]+/[^/]+?)(\.git)?/?$`)

// Repo names the repository: the settings' repo, else the workspace remote.
func Repo(workspace models.WorkspaceConfig, settings domain.WorkspaceSettings) string {
	if settings.GitHub != nil && strings.Count(settings.GitHub.Repo, "/") == 1 {
		return settings.GitHub.Repo
	}
	if match := githubRemote.FindStringSubmatch(strings.TrimSpace(workspace.RemoteURL)); match != nil {
		return match[1]
	}
	return ""
}

func (a *Adapter) Supports(workspace models.WorkspaceConfig, settings domain.WorkspaceSettings) bool {
	return Repo(workspace, settings) != ""
}

func (a *Adapter) CreateDraft(ctx context.Context, request ports.PullRequest) (string, error) {
	repo := Repo(request.Workspace, request.Settings)
	if repo == "" {
		return "", fmt.Errorf("workspace %s has no GitHub repository", request.Workspace.ID)
	}
	tokenName := "GITHUB_TOKEN"
	if request.Settings.GitHub != nil && request.Settings.GitHub.TokenEnvVar != "" {
		tokenName = request.Settings.GitHub.TokenEnvVar
	}
	token, err := a.Tokens.Resolve(tokenName)
	if err != nil {
		return "", err
	}
	if token == "" {
		return a.viaCLI(ctx, repo, request)
	}
	body, _ := json.Marshal(map[string]any{"title": request.Title, "head": request.Branch, "base": request.Base, "body": request.Body, "draft": true})
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(a.BaseURL, "/")+"/repos/"+repo+"/pulls", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpRequest.Header.Set("Accept", "application/vnd.github+json")
	httpRequest.Header.Set("Authorization", "Bearer "+token)
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := a.HTTPClient.Do(httpRequest)
	if err != nil {
		return "", fmt.Errorf("GitHub is unavailable: %w", err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("GitHub refused the pull request with status %d", response.StatusCode)
	}
	var created struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(data, &created); err != nil || created.HTMLURL == "" {
		return "", fmt.Errorf("GitHub answered without a pull request URL")
	}
	return created.HTMLURL, nil
}

func (a *Adapter) viaCLI(ctx context.Context, repo string, request ports.PullRequest) (string, error) {
	run := a.GH
	if run == nil {
		if _, err := exec.LookPath("gh"); err != nil {
			return "", fmt.Errorf("no GitHub token is set and the gh CLI is not installed")
		}
		run = func(ctx context.Context, dir string, args ...string) (string, error) {
			cmd := exec.CommandContext(ctx, "gh", args...)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				return "", fmt.Errorf("gh pr create: %s", strings.TrimSpace(string(out)))
			}
			return string(out), nil
		}
	}
	out, err := run(ctx, request.WorktreePath, "pr", "create", "--draft", "--repo", repo, "--head", request.Branch, "--base", request.Base, "--title", request.Title, "--body", request.Body)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "https://") {
			return line, nil
		}
	}
	return "", fmt.Errorf("gh did not print a pull request URL")
}
