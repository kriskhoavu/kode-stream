package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

type GitHub struct{ HTTPIntegration }

func NewGitHub(baseURL, token string) *GitHub {
	return &GitHub{HTTPIntegration: HTTPIntegration{Provider: "github", BaseURL: baseURL, Token: token}}
}
func (g *GitHub) Repositories(ctx context.Context) ([]Repository, error) {
	var rows []struct {
		ID       interface{} `json:"id"`
		Name     string      `json:"name"`
		FullName string      `json:"full_name"`
		HTMLURL  string      `json:"html_url"`
	}
	if err := g.request(ctx, "user/repos?per_page=100", &rows); err != nil {
		return nil, err
	}
	if len(rows) > MaxListEntries {
		return nil, &LimitError{Resource: "repository list"}
	}
	out := make([]Repository, 0, len(rows))
	for _, r := range rows {
		out = append(out, Repository{ID: fmt.Sprint(r.ID), Name: r.Name, FullName: r.FullName, WebURL: r.HTMLURL})
	}
	return out, nil
}
func (g *GitHub) Refs(ctx context.Context, repository string) ([]Ref, error) {
	var branches []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := g.request(ctx, "repos/"+cleanRepository(repository)+"/branches?per_page=100", &branches); err != nil {
		return nil, err
	}
	if len(branches) > MaxListEntries {
		return nil, &LimitError{Resource: "branch list"}
	}
	out := make([]Ref, 0, len(branches))
	for _, b := range branches {
		out = append(out, Ref{Name: b.Name, CommitSHA: b.Commit.SHA})
	}
	var tags []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := g.request(ctx, "repos/"+cleanRepository(repository)+"/tags?per_page=100", &tags); err != nil {
		return nil, err
	}
	if len(branches)+len(tags) > MaxListEntries {
		return nil, &LimitError{Resource: "ref list"}
	}
	for _, tag := range tags {
		out = append(out, Ref{Name: tag.Name, CommitSHA: tag.Commit.SHA})
	}
	return out, nil
}

func (g *GitHub) ResolveRef(ctx context.Context, repository, ref string) (Ref, error) {
	refs, err := g.Refs(ctx, repository)
	if err != nil {
		return Ref{}, err
	}
	return matchRef(refs, ref)
}

func (g *GitHub) Tree(ctx context.Context, repository, sha, directory string) ([]TreeEntry, error) {
	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
			Size int64  `json:"size"`
		} `json:"tree"`
	}
	if err := g.request(ctx, "repos/"+cleanRepository(repository)+"/git/trees/"+url.PathEscape(sha)+"?recursive=1", &tree); err != nil {
		return nil, err
	}
	if len(tree.Tree) > MaxTreeEntries {
		return nil, &LimitError{Resource: "tree"}
	}
	prefix := strings.Trim(strings.TrimSpace(directory), "/")
	entries := make([]TreeEntry, 0, len(tree.Tree))
	for _, entry := range tree.Tree {
		if !ValidPath(entry.Path) {
			return nil, fmt.Errorf("provider returned an invalid tree path")
		}
		if prefix != "" && entry.Path != prefix && !strings.HasPrefix(entry.Path, prefix+"/") {
			continue
		}
		entries = append(entries, TreeEntry{Path: entry.Path, Type: entry.Type, SHA: entry.SHA, Size: entry.Size})
	}
	return entries, nil
}
func (g *GitHub) ReadFile(ctx context.Context, repository, sha, filePath string) (File, error) {
	if !ValidPath(filePath) {
		return File{}, fmt.Errorf("file path is invalid")
	}
	var row struct {
		Path     string `json:"path"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	suffix := "repos/" + cleanRepository(repository) + "/contents/" + url.PathEscape(filePath) + "?ref=" + url.QueryEscape(sha)
	if err := g.request(ctx, suffix, &row); err != nil {
		return File{}, err
	}
	data, err := base64.StdEncoding.DecodeString(row.Content)
	if err != nil {
		return File{}, err
	}
	if len(data) > MaxFileBytes {
		return File{}, &LimitError{Resource: "file"}
	}
	if !ValidPath(row.Path) {
		return File{}, fmt.Errorf("provider returned an invalid file path")
	}
	return File{Path: row.Path, Content: string(data)}, nil
}
