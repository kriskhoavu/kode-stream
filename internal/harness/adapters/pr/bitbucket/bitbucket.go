// Package bitbucket opens a pull request on Bitbucket: a draft on Bitbucket
// Cloud, and on Bitbucket Server one whose title starts with [DRAFT], since
// older Server versions have no draft flag.
package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
}

func New() *Adapter {
	return &Adapter{CloudURL: "https://api.bitbucket.org/2.0", HTTPClient: &http.Client{Timeout: 30 * time.Second}, Tokens: credential.Default()}
}

func (a *Adapter) ID() string { return "bitbucket" }

func (a *Adapter) Supports(_ models.WorkspaceConfig, settings domain.WorkspaceSettings) bool {
	return settings.Bitbucket != nil && settings.Bitbucket.Workspace != "" && settings.Bitbucket.Repo != "" && (settings.Bitbucket.Cloud || settings.Bitbucket.BaseURL != "")
}

func (a *Adapter) CreateDraft(ctx context.Context, request ports.PullRequest) (string, error) {
	config := request.Settings.Bitbucket
	if config == nil {
		return "", fmt.Errorf("workspace %s has no Bitbucket repository", request.Workspace.ID)
	}
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
