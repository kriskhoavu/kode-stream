package provider

import (
	"context"
	"fmt"
	"strings"
)

type Repository struct{ ID, Name, FullName, WebURL string }
type Ref struct{ Name, CommitSHA string }
type File struct{ Path, Content string }
type TreeEntry struct {
	Path string
	Type string
	SHA  string
	Size int64
}

type GitProviderIntegration interface {
	Name() string
	Repositories(context.Context) ([]Repository, error)
	Refs(context.Context, string) ([]Ref, error)
	ResolveRef(context.Context, string, string) (Ref, error)
	Tree(context.Context, string, string, string) ([]TreeEntry, error)
	ReadFile(context.Context, string, string, string) (File, error)
}

func matchRef(refs []Ref, ref string) (Ref, error) {
	for _, candidate := range refs {
		if candidate.Name == ref || candidate.CommitSHA == ref {
			return candidate, nil
		}
	}
	return Ref{}, fmt.Errorf("ref %q was not found", ref)
}

func Require(integration GitProviderIntegration, name string) (GitProviderIntegration, error) {
	if integration == nil || !strings.EqualFold(integration.Name(), name) {
		return nil, fmt.Errorf("%s provider is not configured", name)
	}
	return integration, nil
}
