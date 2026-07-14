package system

import (
	"fmt"
	"os"
	"strings"

	"kode-stream/internal/common/models"
)

const (
	EnvRuntimeMode      = "KODE_STREAM_MODE"
	EnvBindAddress      = "KODE_STREAM_BIND_ADDR"
	EnvCookieSecret     = "KODE_STREAM_COOKIE_SECRET"
	EnvOIDCIssuer       = "KODE_STREAM_OIDC_ISSUER"
	EnvOIDCClientID     = "KODE_STREAM_OIDC_CLIENT_ID"
	EnvOIDCClientSecret = "KODE_STREAM_OIDC_CLIENT_SECRET"
	EnvPublicURL        = "KODE_STREAM_PUBLIC_URL"
	EnvAdminUsers       = "KODE_STREAM_ADMIN_USERS"
)

type RuntimeConfig struct {
	Mode             models.RuntimeMode         `json:"mode"`
	BindAddress      string                     `json:"bindAddress"`
	CookieSecret     string                     `json:"-"`
	OIDCIssuer       string                     `json:"-"`
	OIDCClientID     string                     `json:"-"`
	OIDCClientSecret string                     `json:"-"`
	PublicURL        string                     `json:"-"`
	AdminUsers       []string                   `json:"-"`
	User             *models.CloudUser          `json:"user,omitempty"`
	Role             models.CloudRole           `json:"role"`
	Capabilities     map[models.Capability]bool `json:"capabilities"`
	Agent            models.AgentConnection     `json:"agent"`
}

func ResolveRuntimeConfig() (RuntimeConfig, error) {
	return ResolveRuntimeConfigFromEnv(os.Getenv)
}

func ResolveRuntimeConfigFromEnv(getenv func(string) string) (RuntimeConfig, error) {
	mode := models.RuntimeMode(strings.ToLower(strings.TrimSpace(getenv(EnvRuntimeMode))))
	if mode == "" {
		mode = models.RuntimeModeLocal
	}
	if mode != models.RuntimeModeLocal && mode != models.RuntimeModeCloud {
		return RuntimeConfig{}, fmt.Errorf("%s must be local or cloud", EnvRuntimeMode)
	}

	bindAddress := strings.TrimSpace(getenv(EnvBindAddress))
	if bindAddress == "" {
		if mode == models.RuntimeModeCloud {
			bindAddress = "0.0.0.0"
		} else {
			bindAddress = "127.0.0.1"
		}
	}

	config := RuntimeConfig{
		Mode:             mode,
		BindAddress:      bindAddress,
		CookieSecret:     strings.TrimSpace(getenv(EnvCookieSecret)),
		OIDCIssuer:       strings.TrimSpace(getenv(EnvOIDCIssuer)),
		OIDCClientID:     strings.TrimSpace(getenv(EnvOIDCClientID)),
		OIDCClientSecret: strings.TrimSpace(getenv(EnvOIDCClientSecret)),
		PublicURL:        strings.TrimRight(strings.TrimSpace(getenv(EnvPublicURL)), "/"),
		AdminUsers:       splitList(getenv(EnvAdminUsers)),
		Role:             models.CloudRoleAdmin,
		Capabilities:     defaultCapabilities(mode),
		Agent:            models.AgentConnection{Available: mode == models.RuntimeModeLocal, Status: "unsupported"},
	}
	if mode == models.RuntimeModeLocal {
		config.Agent.Status = "local"
		return config, nil
	}
	config.Role = ""
	config.Agent = models.AgentConnection{Available: false, Status: "offline"}
	return config, nil
}

func ValidateCloudRuntimeConfig(config RuntimeConfig) error {
	if config.Mode != models.RuntimeModeCloud {
		return nil
	}
	missing := []string{}
	if config.PublicURL == "" {
		missing = append(missing, EnvPublicURL)
	}
	if config.CookieSecret == "" {
		missing = append(missing, EnvCookieSecret)
	}
	if config.OIDCIssuer == "" {
		missing = append(missing, EnvOIDCIssuer)
	}
	if config.OIDCClientID == "" {
		missing = append(missing, EnvOIDCClientID)
	}
	if config.OIDCClientSecret == "" {
		missing = append(missing, EnvOIDCClientSecret)
	}
	if len(config.AdminUsers) == 0 {
		missing = append(missing, EnvAdminUsers)
	}
	if len(missing) > 0 {
		return fmt.Errorf("cloud mode requires %s", strings.Join(missing, ", "))
	}
	return nil
}

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func defaultCapabilities(mode models.RuntimeMode) map[models.Capability]bool {
	capabilities := map[models.Capability]bool{
		models.CapabilityRead:                  true,
		models.CapabilityWrite:                 true,
		models.CapabilityWorkspaceRegistration: true,
		models.CapabilityGit:                   true,
		models.CapabilitySystem:                true,
		models.CapabilityTerminal:              true,
		models.CapabilityAI:                    true,
		models.CapabilityRuntime:               true,
		models.CapabilityVerification:          true,
	}
	if mode == models.RuntimeModeCloud {
		capabilities[models.CapabilitySystem] = false
		capabilities[models.CapabilityTerminal] = false
		capabilities[models.CapabilityAI] = false
		capabilities[models.CapabilityRuntime] = false
		capabilities[models.CapabilityVerification] = false
		capabilities[models.CapabilityGit] = false
		capabilities[models.CapabilityWrite] = false
	}
	return capabilities
}
