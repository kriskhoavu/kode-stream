// Package bitbucket opens a draft pull request. On Server the title starts with [DRAFT], since older versions have no draft flag.
package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/adapters/credential"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

type Adapter struct {
	CloudURL   string
	HTTPClient *http.Client
	Tokens     credential.Resolver
	RemoteURL  func(path string) (string, error)
}

const defaultTokenEnvVar = "BITBUCKET_TOKEN"

func (a *Adapter) Target(workspace models.WorkspaceConfig, settings domain.WorkspaceSettings) (domain.BitbucketSourceSettings, bool) {
	if config := settings.Bitbucket; config != nil && config.Workspace != "" && config.Repo != "" && (config.Cloud || config.BaseURL != "") {
		out := *config
		if out.TokenEnvVar == "" {
			out.TokenEnvVar = defaultTokenEnvVar
		}
		return out, true
	}
	remote := strings.TrimSpace(workspace.RemoteURL)
	if remote == "" && a.RemoteURL != nil && workspace.Path != "" {
		if origin, err := a.RemoteURL(workspace.Path); err == nil {
			remote = strings.TrimSpace(origin)
		}
	}
	target, ok := ParseRemote(remote)
	if !ok {
		return domain.BitbucketSourceSettings{}, false
	}
	if config := settings.Bitbucket; config != nil && config.TokenEnvVar != "" {
		target.TokenEnvVar = config.TokenEnvVar
	}
	return target, true
}

var (
	sshRemote   = regexp.MustCompile(`^ssh://(?:[^@/]+@)?([^/:]+)(?::\d+)?/(.+)$`)
	scpRemote   = regexp.MustCompile(`^(?:[^@/]+@)?([^/:]+):(.+)$`)
	httpsRemote = regexp.MustCompile(`^https?://(?:[^@/]+@)?([^/:]+)(?::\d+)?/(.+)$`)
)

func ParseRemote(remote string) (domain.BitbucketSourceSettings, bool) {
	var host, path string
	for _, pattern := range []*regexp.Regexp{sshRemote, httpsRemote, scpRemote} {
		if match := pattern.FindStringSubmatch(remote); match != nil {
			host, path = strings.ToLower(match[1]), match[2]
			break
		}
	}
	if host == "" || !strings.Contains(host, "bitbucket") {
		return domain.BitbucketSourceSettings{}, false
	}
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	path = strings.TrimPrefix(path, "scm/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return domain.BitbucketSourceSettings{}, false
	}
	target := domain.BitbucketSourceSettings{Workspace: parts[0], Repo: parts[1], TokenEnvVar: defaultTokenEnvVar}
	if host == "bitbucket.org" {
		target.Cloud = true
	} else {
		target.BaseURL = "https://" + host
	}
	return target, true
}

func New() *Adapter {
	return &Adapter{CloudURL: "https://api.bitbucket.org/2.0", HTTPClient: &http.Client{Timeout: 30 * time.Second}, Tokens: credential.Default()}
}

func (a *Adapter) ID() string { return "bitbucket" }

func (a *Adapter) Supports(workspace models.WorkspaceConfig, settings domain.WorkspaceSettings) bool {
	_, ok := a.Target(workspace, settings)
	return ok
}

func (a *Adapter) CreateDraft(ctx context.Context, request ports.PullRequest) (string, error) {
	target, ok := a.Target(request.Workspace, request.Settings)
	if !ok {
		return "", fmt.Errorf("workspace %s has no Bitbucket repository: its origin is not a Bitbucket host and Harness settings name none", request.Workspace.ID)
	}
	config := &target
	token, err := a.Tokens.Resolve(config.TokenEnvVar)
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", fmt.Errorf("the Bitbucket token %s is not set", config.TokenEnvVar)
	}
	var endpoint string
	var body map[string]any
	if config.Cloud {
		base := strings.TrimRight(a.CloudURL, "/")
		if config.BaseURL != "" {
			base = strings.TrimRight(config.BaseURL, "/")
		}
		endpoint = base + "/repositories/" + url.PathEscape(config.Workspace) + "/" + url.PathEscape(config.Repo) + "/pullrequests"
		body = map[string]any{"title": request.Title, "description": request.Body, "draft": true, "source": map[string]any{"branch": map[string]string{"name": request.Branch}}, "destination": map[string]any{"branch": map[string]string{"name": request.Base}}}
	} else {
		endpoint = strings.TrimRight(config.BaseURL, "/") + "/rest/api/1.0/projects/" + url.PathEscape(config.Workspace) + "/repos/" + url.PathEscape(config.Repo) + "/pull-requests"
		body = map[string]any{"title": "[DRAFT] " + request.Title, "description": request.Body, "fromRef": map[string]string{"id": "refs/heads/" + request.Branch}, "toRef": map[string]string{"id": "refs/heads/" + request.Base}}
	}
	data, _ := json.Marshal(body)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+token)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	response, err := a.HTTPClient.Do(httpRequest)
	if err != nil {
		return "", fmt.Errorf("Bitbucket is unavailable: %w", err)
	}
	defer response.Body.Close()
	answer, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Bitbucket refused the pull request with status %d", response.StatusCode)
	}
	var created struct {
		Links struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
			Self []struct {
				Href string `json:"href"`
			} `json:"self"`
		} `json:"links"`
	}
	if err := json.Unmarshal(answer, &created); err != nil {
		return "", fmt.Errorf("decode Bitbucket pull request: %w", err)
	}
	if created.Links.HTML.Href != "" {
		return created.Links.HTML.Href, nil
	}
	if len(created.Links.Self) > 0 && created.Links.Self[0].Href != "" {
		return created.Links.Self[0].Href, nil
	}
	return "", fmt.Errorf("Bitbucket answered without a pull request URL")
}
