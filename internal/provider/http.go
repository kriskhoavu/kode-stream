package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type HTTPIntegration struct {
	Provider, BaseURL, Token string
	Client                   *http.Client
}

func (h HTTPIntegration) raw(ctx context.Context, suffix string) ([]byte, error) {
	base, err := url.Parse(strings.TrimRight(h.BaseURL, "/") + "/" + strings.TrimLeft(suffix, "/"))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+h.Token)
	client := h.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("%s provider request failed: %s", h.Provider, res.Status)
	}
	return io.ReadAll(res.Body)
}

func (h HTTPIntegration) Name() string { return h.Provider }
func (h HTTPIntegration) request(ctx context.Context, suffix string, target any) error {
	base, err := url.Parse(strings.TrimRight(h.BaseURL, "/") + "/" + strings.TrimLeft(suffix, "/"))
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+h.Token)
	req.Header.Set("Accept", "application/json")
	client := h.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%s provider request failed: %s", h.Provider, res.Status)
	}
	return json.NewDecoder(res.Body).Decode(target)
}
func (h HTTPIntegration) Repositories(ctx context.Context) ([]Repository, error) {
	return nil, fmt.Errorf("%s repository discovery requires its provider adapter", h.Provider)
}
func (h HTTPIntegration) Refs(ctx context.Context, repository string) ([]Ref, error) {
	return nil, fmt.Errorf("%s ref discovery requires its provider adapter", h.Provider)
}
func (h HTTPIntegration) ResolveRef(ctx context.Context, repository, ref string) (Ref, error) {
	refs, err := h.Refs(ctx, repository)
	if err != nil {
		return Ref{}, err
	}
	return matchRef(refs, ref)
}
func (h HTTPIntegration) ReadFile(ctx context.Context, repository, sha, filePath string) (File, error) {
	return File{}, fmt.Errorf("%s file reads require its provider adapter", h.Provider)
}
func (h HTTPIntegration) Tree(ctx context.Context, repository, sha, directory string) ([]TreeEntry, error) {
	return nil, fmt.Errorf("%s tree reads require its provider adapter", h.Provider)
}
func cleanRepository(repository string) string { return strings.Trim(path.Clean("/"+repository), "/") }
