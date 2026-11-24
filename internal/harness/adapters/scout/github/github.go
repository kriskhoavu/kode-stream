package github

import (
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
)

const maxResponseBytes = 4 << 20

type Scout struct {
	BaseURL    string
	HTTPClient *http.Client
	Tokens     credential.Resolver
}

func New() *Scout {
	return &Scout{BaseURL: "https://api.github.com", HTTPClient: &http.Client{Timeout: 15 * time.Second}, Tokens: credential.Default()}
}

func (s *Scout) Kind() domain.SourceKind { return domain.SourceGitHub }

func (s *Scout) Enabled(_ models.WorkspaceConfig, settings domain.WorkspaceSettings) bool {
	return settings.GitHub != nil && settings.GitHub.Enabled && strings.Count(settings.GitHub.Repo, "/") == 1
}

type issue struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	State       string `json:"state"`
	UpdatedAt   string `json:"updated_at"`
	PullRequest *struct {
		URL string `json:"url"`
	} `json:"pull_request"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Assignee *struct {
		Login string `json:"login"`
	} `json:"assignee"`
	Milestone *struct {
		Title string `json:"title"`
	} `json:"milestone"`
}

func (s *Scout) Poll(ctx context.Context, workspace models.WorkspaceConfig, settings domain.WorkspaceSettings) ([]domain.Candidate, error) {
	if !s.Enabled(workspace, settings) {
		return nil, fmt.Errorf("workspace %s has no GitHub repository", workspace.ID)
	}
	config := settings.GitHub
	query := url.Values{}
	query.Set("state", "open")
	query.Set("per_page", "100")
	if len(config.Labels) > 0 {
		query.Set("labels", strings.Join(config.Labels, ","))
	}
	endpoint := strings.TrimRight(s.BaseURL, "/") + "/repos/" + config.Repo + "/issues?" + query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	token, err := s.Tokens.Resolve(config.TokenEnvVar)
	if err != nil {
		return nil, err
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := s.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GitHub is unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return nil, fmt.Errorf("GitHub returned status %d for %s", response.StatusCode, config.Repo)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseBytes {
		return nil, fmt.Errorf("GitHub response exceeds the size limit")
	}
	var issues []issue
	if err := json.Unmarshal(data, &issues); err != nil {
		return nil, fmt.Errorf("decode GitHub issues: %w", err)
	}
	candidates := make([]domain.Candidate, 0, len(issues))
	for _, item := range issues {
		if item.PullRequest != nil || item.State != "open" {
			continue
		}
		source := domain.Source{Kind: domain.SourceGitHub, Key: fmt.Sprintf("%s#%d", config.Repo, item.Number), URL: item.HTMLURL, Status: item.State, Labels: []string{}}
		for _, label := range item.Labels {
			source.Labels = append(source.Labels, label.Name)
			if priority := priorityLabel(label.Name); priority != "" && source.Priority == "" {
				source.Priority = priority
			}
		}
		if item.Assignee != nil {
			source.Assignee = item.Assignee.Login
		}
		if item.Milestone != nil {
			source.Milestone = item.Milestone.Title
		}
		if updated, err := time.Parse(time.RFC3339, item.UpdatedAt); err == nil {
			source.UpdatedAt = &updated
		}
		candidates = append(candidates, domain.Candidate{WorkspaceID: workspace.ID, Source: source, Title: item.Title, Body: item.Body})
	}
	return candidates, nil
}

func priorityLabel(label string) string {
	lower := strings.ToLower(strings.TrimSpace(label))
	lower = strings.TrimPrefix(lower, "priority:")
	lower = strings.TrimPrefix(lower, "priority/")
	switch strings.TrimSpace(lower) {
	case "p0", "critical", "blocker", "highest":
		return "Highest"
	case "p1", "high":
		return "High"
	case "p2", "medium":
		return "Medium"
	case "p3", "low":
		return "Low"
	}
	return ""
}
