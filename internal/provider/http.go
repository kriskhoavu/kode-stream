package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// These limits apply before provider responses are decoded or exposed through
// the Cloud snapshot API.  They deliberately bound a single request rather
// than relying on a provider's pagination defaults.
const (
	MaxResponseBytes = 2 << 20
	MaxFileBytes     = 1 << 20
	MaxListEntries   = 500
	MaxTreeEntries   = 2_000
	MaxPathBytes     = 1_024
	MaxPathDepth     = 32
)

// LimitError is deliberately typed so transports can distinguish an upstream
// capacity boundary from an unavailable provider.
type LimitError struct{ Resource string }

func (e *LimitError) Error() string { return "provider " + e.Resource + " exceeds the supported limit" }

func IsLimitError(err error) bool {
	_, ok := err.(*LimitError)
	return ok
}

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
	return readBounded(res.Body, MaxFileBytes, "file")
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
	data, err := readBounded(res.Body, MaxResponseBytes, "response")
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("provider response contains multiple JSON values")
		}
		return err
	}
	return nil
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

func readBounded(reader io.Reader, limit int64, resource string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, &LimitError{Resource: resource}
	}
	return data, nil
}

func ValidPath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > MaxPathBytes || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return false
	}
	parts := strings.Split(value, "/")
	if len(parts) > MaxPathDepth {
		return false
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
