package provider

import (
	"fmt"
	"sync"
)

// Instance identifies one administrator-configured provider endpoint.
type Instance struct {
	ID      string
	Name    string
	Kind    string
	BaseURL string
}

// Factory constructs an integration for one configured endpoint and user connection.
type Factory interface {
	Kind() string
	New(Instance, string) GitProviderIntegration
}

type Registry struct {
	mu        sync.RWMutex
	instances map[string]Instance
	factories map[string]Factory
}

func NewRegistry(factories ...Factory) *Registry {
	r := &Registry{instances: map[string]Instance{}, factories: map[string]Factory{}}
	for _, factory := range factories {
		r.factories[factory.Kind()] = factory
	}
	return r
}

func (r *Registry) Upsert(instance Instance) error {
	if err := ValidateInstance(instance); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.factories[instance.Kind]; !ok {
		return fmt.Errorf("unsupported provider kind %q", instance.Kind)
	}
	r.instances[instance.ID] = instance
	return nil
}

func ValidateInstance(instance Instance) error {
	if instance.ID == "" || instance.Name == "" || instance.Kind == "" || instance.BaseURL == "" {
		return fmt.Errorf("provider instance id, name, kind, and base URL are required")
	}
	return nil
}

func (r *Registry) Integration(instanceID, token string) (GitProviderIntegration, error) {
	r.mu.RLock()
	instance, ok := r.instances[instanceID]
	factory := r.factories[instance.Kind]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("provider instance %q was not found", instanceID)
	}
	if token == "" {
		return nil, fmt.Errorf("provider connection is required")
	}
	return factory.New(instance, token), nil
}

type GitHubFactory struct{}

func (GitHubFactory) Kind() string { return "github" }
func (GitHubFactory) New(instance Instance, token string) GitProviderIntegration {
	return NewGitHub(instance.BaseURL, token)
}

type BitbucketServerFactory struct{}

func (BitbucketServerFactory) Kind() string { return "bitbucket_server" }
func (BitbucketServerFactory) New(instance Instance, token string) GitProviderIntegration {
	return NewBitbucketServer(instance.BaseURL, token)
}
