package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// BitbucketServer implements the Bitbucket Server/Data Center REST API.
type BitbucketServer struct{ HTTPIntegration }

func NewBitbucketServer(baseURL, token string) *BitbucketServer {
	return &BitbucketServer{HTTPIntegration: HTTPIntegration{Provider: "bitbucket_server", BaseURL: baseURL, Token: token}}
}

func (b *BitbucketServer) Repositories(ctx context.Context) ([]Repository, error) {
	var page struct {
		Values []struct {
			ID         int64 `json:"id"`
			Name, Slug string
			Project    struct {
				Key string `json:"key"`
			}
			Links struct {
				Self []struct {
					Href string `json:"href"`
				} `json:"self"`
			} `json:"links"`
		} `json:"values"`
	}
	if err := b.request(ctx, "rest/api/1.0/repos?limit=100", &page); err != nil {
		return nil, err
	}
	if len(page.Values) > MaxListEntries {
		return nil, &LimitError{Resource: "repository list"}
	}
	out := make([]Repository, 0, len(page.Values))
	for _, repo := range page.Values {
		webURL := ""
		if len(repo.Links.Self) > 0 {
			webURL = repo.Links.Self[0].Href
		}
		out = append(out, Repository{ID: fmt.Sprint(repo.ID), Name: repo.Name, FullName: repo.Project.Key + "/" + repo.Slug, WebURL: webURL})
	}
	return out, nil
}

func (b *BitbucketServer) Refs(ctx context.Context, repository string) ([]Ref, error) {
	var page struct {
		Values []struct {
			DisplayID, LatestCommit string `json:"displayId"`
		} `json:"values"`
	}
	if err := b.request(ctx, "rest/api/1.0/projects/"+project(repository)+"/repos/"+slug(repository)+"/branches?limit=100", &page); err != nil {
		return nil, err
	}
	if len(page.Values) > MaxListEntries {
		return nil, &LimitError{Resource: "branch list"}
	}
	out := make([]Ref, 0, len(page.Values))
	for _, branch := range page.Values {
		out = append(out, Ref{Name: branch.DisplayID, CommitSHA: branch.LatestCommit})
	}
	var tags struct {
		Values []struct {
			DisplayID, LatestCommit string `json:"displayId"`
		} `json:"values"`
	}
	if err := b.request(ctx, "rest/api/1.0/projects/"+project(repository)+"/repos/"+slug(repository)+"/tags?limit=100", &tags); err != nil {
		return nil, err
	}
	if len(page.Values)+len(tags.Values) > MaxListEntries {
		return nil, &LimitError{Resource: "ref list"}
	}
	for _, tag := range tags.Values {
		out = append(out, Ref{Name: tag.DisplayID, CommitSHA: tag.LatestCommit})
	}
	return out, nil
}

func (b *BitbucketServer) ResolveRef(ctx context.Context, repository, ref string) (Ref, error) {
	refs, err := b.Refs(ctx, repository)
	if err != nil {
		return Ref{}, err
	}
	return matchRef(refs, ref)
}

func (b *BitbucketServer) Tree(ctx context.Context, repository, sha, directory string) ([]TreeEntry, error) {
	directory = strings.Trim(strings.TrimSpace(directory), "/")
	suffix := "rest/api/1.0/projects/" + project(repository) + "/repos/" + slug(repository) + "/browse"
	if directory != "" {
		suffix += "/" + url.PathEscape(directory)
	}
	suffix += "?at=" + url.QueryEscape(sha) + "&limit=1000"
	var page struct {
		Children struct {
			Values []struct {
				Path struct {
					Components []string `json:"components"`
				} `json:"path"`
				Type string `json:"type"`
				Size int64  `json:"size"`
			} `json:"values"`
		} `json:"children"`
	}
	if err := b.request(ctx, suffix, &page); err != nil {
		return nil, err
	}
	if len(page.Children.Values) > MaxTreeEntries {
		return nil, &LimitError{Resource: "tree"}
	}
	entries := make([]TreeEntry, 0, len(page.Children.Values))
	for _, entry := range page.Children.Values {
		entryPath := strings.Join(entry.Path.Components, "/")
		if !ValidPath(entryPath) {
			return nil, fmt.Errorf("provider returned an invalid tree path")
		}
		entries = append(entries, TreeEntry{Path: entryPath, Type: entry.Type, Size: entry.Size})
	}
	return entries, nil
}

func (b *BitbucketServer) ReadFile(ctx context.Context, repository, sha, filePath string) (File, error) {
	if !ValidPath(filePath) {
		return File{}, fmt.Errorf("file path is invalid")
	}
	data, err := b.raw(ctx, "rest/api/1.0/projects/"+project(repository)+"/repos/"+slug(repository)+"/raw/"+url.PathEscape(filePath)+"?at="+url.QueryEscape(sha))
	if err != nil {
		return File{}, err
	}
	return File{Path: filePath, Content: string(data)}, nil
}

func project(repository string) string {
	parts := strings.Split(cleanRepository(repository), "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
func slug(repository string) string {
	parts := strings.Split(cleanRepository(repository), "/")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}
