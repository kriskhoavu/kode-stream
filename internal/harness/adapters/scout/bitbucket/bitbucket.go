// Package bitbucket lists a Bitbucket Cloud repository's open issues. Bitbucket
// Server and Data Center have no issue tracker, so a workspace there gets no
// candidates and an explanatory error.
package bitbucket

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
	return &Scout{BaseURL: "https://api.bitbucket.org/2.0", HTTPClient: &http.Client{Timeout: 15 * time.Second}, Tokens: credential.Default()}
}

func (s *Scout) Kind() domain.SourceKind { return domain.SourceBitbucket }

func (s *Scout) Enabled(_ models.WorkspaceConfig, settings domain.WorkspaceSettings) bool {
	return settings.Bitbucket != nil && settings.Bitbucket.Enabled
}

type issuesPage struct {
	Values []struct {
		ID       int    `json:"id"`
		Title    string `json:"title"`
		State    string `json:"state"`
		Priority string `json:"priority"`
		Kind     string `json:"kind"`
		Content  struct {
			Raw string `json:"raw"`
		} `json:"content"`
		Assignee *struct {
			DisplayName string `json:"display_name"`
		} `json:"assignee"`
		Milestone *struct {
			Name string `json:"name"`
		} `json:"milestone"`
		UpdatedOn string `json:"updated_on"`
		Links     struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
		} `json:"links"`
	} `json:"values"`
}

func (s *Scout) Poll(ctx context.Context, workspace models.WorkspaceConfig, settings domain.WorkspaceSettings) ([]domain.Candidate, error) {
	if !s.Enabled(workspace, settings) {
		return nil, fmt.Errorf("workspace %s has no Bitbucket repository", workspace.ID)
	}
	config := settings.Bitbucket
	if !config.Cloud {
		return nil, fmt.Errorf("Bitbucket Server has no issue tracker; use Jira for workspace %s", workspace.ID)
	}
	base := strings.TrimRight(s.BaseURL, "/")
	if config.BaseURL != "" {
		base = strings.TrimRight(config.BaseURL, "/")
	}
	query := url.Values{}
	query.Set("q", `state="new" OR state="open"`)
	query.Set("pagelen", "50")
	endpoint := base + "/repositories/" + url.PathEscape(config.Workspace) + "/" + url.PathEscape(config.Repo) + "/issues?" + query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	token, err := s.Tokens.Resolve(config.TokenEnvVar)
	if err != nil {
		return nil, err
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := s.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Bitbucket is unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return nil, fmt.Errorf("Bitbucket returned status %d for %s/%s", response.StatusCode, config.Workspace, config.Repo)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseBytes {
		return nil, fmt.Errorf("Bitbucket response exceeds the size limit")
	}
	var page issuesPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("decode Bitbucket issues: %w", err)
	}
	candidates := make([]domain.Candidate, 0, len(page.Values))
	for _, item := range page.Values {
		if item.State != "new" && item.State != "open" {
			continue
		}
		source := domain.Source{Kind: domain.SourceBitbucket, Key: fmt.Sprintf("%s/%s#%d", config.Workspace, config.Repo, item.ID), URL: item.Links.HTML.Href, Status: item.State, Labels: []string{}, Priority: bitbucketPriority(item.Priority)}
		if item.Kind != "" {
			source.Labels = append(source.Labels, item.Kind)
		}
		if item.Assignee != nil {
			source.Assignee = item.Assignee.DisplayName
		}
		if item.Milestone != nil {
			source.Milestone = item.Milestone.Name
		}
		if updated, err := time.Parse(time.RFC3339Nano, item.UpdatedOn); err == nil {
			updated = updated.UTC()
			source.UpdatedAt = &updated
		}
		candidates = append(candidates, domain.Candidate{WorkspaceID: workspace.ID, Source: source, Title: item.Title, Body: item.Content.Raw})
	}
	return candidates, nil
}

func bitbucketPriority(priority string) string {
	switch priority {
	case "blocker", "critical":
		return "Highest"
	case "major":
		return "High"
	case "minor":
		return "Low"
	case "trivial":
		return "Lowest"
	default:
		return "Medium"
	}
}
