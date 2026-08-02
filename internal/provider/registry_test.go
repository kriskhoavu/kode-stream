package provider

import "testing"

func TestRegistryResolvesConfiguredProviderInstance(t *testing.T) {
	registry := NewRegistry(GitHubFactory{}, BitbucketServerFactory{})
	if err := registry.Upsert(Instance{ID: "corp-bitbucket", Name: "Company Bitbucket", Kind: "bitbucket_server", BaseURL: "https://bitbucket.example.com"}); err != nil {
		t.Fatal(err)
	}
	integration, err := registry.Integration("corp-bitbucket", "token")
	if err != nil {
		t.Fatal(err)
	}
	if integration.Name() != "bitbucket_server" {
		t.Fatalf("provider = %q", integration.Name())
	}
}

func TestRegistryRejectsUnknownProviderKind(t *testing.T) {
	registry := NewRegistry(GitHubFactory{})
	if err := registry.Upsert(Instance{ID: "unknown", Name: "Unknown", Kind: "unknown", BaseURL: "https://example.com"}); err == nil {
		t.Fatal("expected error")
	}
}
