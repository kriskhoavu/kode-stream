package registry

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/sourceguard"
	appruntime "kode-stream/internal/runtime"
)

func (r *Registry) validate(input models.WorkspaceInput) (models.WorkspaceConfig, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.WorkspaceConfig{}, errors.New("workspace name is required")
	}
	mode := normalizeRegistrationMode(input.RegistrationMode)
	branch := strings.TrimSpace(input.BaselineBranch)
	if branch == "" {
		branch = "main"
	}
	pathValue := strings.TrimSpace(input.Path)
	if mode != models.WorkspaceRegistrationModeRemoteClone {
		if pathValue == "" {
			return models.WorkspaceConfig{}, errors.New("workspace path is required")
		}
	} else {
		if pathValue == "" {
			return models.WorkspaceConfig{}, errors.New("cloned workspace path is required")
		}
		if strings.TrimSpace(input.RemoteURL) == "" {
			return models.WorkspaceConfig{}, errors.New("remote URL is required")
		}
	}
	path, err := filepath.Abs(expandHome(strings.TrimSpace(input.Path)))
	if err != nil || path == "" {
		return models.WorkspaceConfig{}, errors.New("workspace path is invalid")
	}
	root, err := r.git.WorkspaceRoot(path)
	if err != nil {
		return models.WorkspaceConfig{}, fmt.Errorf("not a Git workspace: %w", err)
	}
	if err := r.git.ValidateBranch(root, branch); err != nil {
		return models.WorkspaceConfig{}, fmt.Errorf("baseline branch is invalid: %w", err)
	}
	if len(input.Sources) == 0 {
		return models.WorkspaceConfig{}, errors.New("at least one workspace source is required")
	}
	validatedSources, err := sourceguard.ResolveAll(root, input.SourcePaths())
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	cleanDirs := make([]models.WorkspaceSource, 0, len(validatedSources))
	for index, source := range validatedSources {
		normalized, err := models.WorkspaceSource{Path: source.Relative, ShownIn: input.Sources[index].ShownIn}.Normalized()
		if err != nil {
			return models.WorkspaceConfig{}, err
		}
		if len(normalized.ShownIn) == 0 {
			return models.WorkspaceConfig{}, fmt.Errorf("source %q must show in Workstream or Knowledge", source.Relative)
		}
		cleanDirs = append(cleanDirs, normalized)
	}
	jira, err := ValidateJiraConnection(input.Jira)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}
	runtimeConfig, err := appruntime.NormalizeRuntimeConfig(input.Runtime)
	if err != nil {
		return models.WorkspaceConfig{}, err
	}

	return models.WorkspaceConfig{
		ID:                   slug(name) + "-" + shortHash(root),
		Name:                 name,
		Path:                 root,
		Location:             models.WorkspaceLocationLocalPath,
		BaselineBranch:       branch,
		RegistrationMode:     mode,
		RemoteURL:            strings.TrimSpace(input.RemoteURL),
		ClonePathManaged:     mode == models.WorkspaceRegistrationModeRemoteClone && input.ManagedCloneRoot != "" && input.ManagedCloneID != "",
		ManagedCloneRoot:     input.ManagedCloneRoot,
		ManagedCloneID:       input.ManagedCloneID,
		ManagedCloneVerified: mode == models.WorkspaceRegistrationModeRemoteClone && input.ManagedCloneRoot != "" && input.ManagedCloneID != "",
		Sources:              cleanDirs,
		CreatedAt:            time.Now().UTC(),
		Jira:                 jira,
		Knowledge:            normalizeKnowledgeSettings(input.Knowledge),
		Runtime:              runtimeConfig,
	}, nil
}

var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var projectKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func ValidateJiraConnection(input *models.JiraConnection) (*models.JiraConnection, error) {
	if input == nil {
		return nil, nil
	}
	connection := *input
	connection.DeploymentType = strings.ToLower(strings.TrimSpace(connection.DeploymentType))
	connection.BaseURL = strings.TrimRight(strings.TrimSpace(connection.BaseURL), "/")
	connection.ProjectKey = strings.ToUpper(strings.TrimSpace(connection.ProjectKey))
	connection.AccountEmail = strings.TrimSpace(connection.AccountEmail)
	connection.TokenEnvVar = strings.TrimSpace(connection.TokenEnvVar)
	if connection.DeploymentType != "cloud" && connection.DeploymentType != "server" {
		return nil, errors.New("Jira deployment type must be cloud or server")
	}
	parsed, err := url.Parse(connection.BaseURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("Jira base URL is invalid")
	}
	loopback := parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "::1"
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && loopback) {
		return nil, errors.New("Jira base URL must use HTTPS")
	}
	if !projectKeyPattern.MatchString(connection.ProjectKey) {
		return nil, errors.New("Jira project key is invalid")
	}
	if !envNamePattern.MatchString(connection.TokenEnvVar) {
		return nil, errors.New("Jira token environment variable is invalid")
	}
	if connection.DeploymentType == "cloud" && connection.AccountEmail == "" {
		return nil, errors.New("Jira Cloud account email is required")
	}
	if connection.DeploymentType == "server" {
		connection.AccountEmail = ""
	}
	return &connection, nil
}

func normalizeWorkspace(workspace models.WorkspaceConfig) models.WorkspaceConfig {
	if workspace.Sources == nil {
		workspace.Sources = []models.WorkspaceSource{}
	}
	if workspace.Location != models.WorkspaceLocationCloudAgent {
		workspace.Location = models.WorkspaceLocationLocalPath
	}
	workspace.RegistrationMode = normalizeRegistrationMode(workspace.RegistrationMode)
	if workspace.RegistrationMode != models.WorkspaceRegistrationModeRemoteClone {
		workspace.RemoteURL = ""
		workspace.ClonePathManaged = false
		workspace.ManagedCloneRoot = ""
		workspace.ManagedCloneID = ""
		workspace.ManagedCloneVerified = false
	} else if workspace.ManagedCloneRoot == "" || workspace.ManagedCloneID == "" {
		// Legacy managed flags are not proof of ownership.
		workspace.ManagedCloneVerified = false
	}
	workspace.Knowledge = normalizeKnowledgeSettings(workspace.Knowledge)
	if normalized, err := appruntime.NormalizeRuntimeConfig(workspace.Runtime); err == nil {
		workspace.Runtime = normalized
	} else {
		workspace.Runtime = nil
	}
	return workspace
}

func normalizeKnowledgeSettings(settings *models.KnowledgeSettings) *models.KnowledgeSettings {
	if settings == nil {
		return nil
	}
	normalized := *settings
	normalized.EnrichExecutable = strings.TrimSpace(normalized.EnrichExecutable)
	if normalized.EnrichArgs == nil {
		normalized.EnrichArgs = []string{}
	} else {
		normalized.EnrichArgs = append([]string(nil), normalized.EnrichArgs...)
	}
	return &normalized
}

func normalizeRegistrationMode(mode models.WorkspaceRegistrationMode) models.WorkspaceRegistrationMode {
	switch strings.TrimSpace(string(mode)) {
	case string(models.WorkspaceRegistrationModeRemoteClone):
		return models.WorkspaceRegistrationModeRemoteClone
	case string(models.WorkspaceRegistrationModeExisting):
		return models.WorkspaceRegistrationModeExisting
	default:
		return models.WorkspaceRegistrationModeLocalPath
	}
}
