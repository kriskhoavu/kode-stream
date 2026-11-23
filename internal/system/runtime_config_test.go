package system

import (
	"testing"

	"kode-stream/internal/common/models"
)

func TestResolveRuntimeConfigDefaultsToLocalLoopback(t *testing.T) {
	config, err := ResolveRuntimeConfigFromEnv(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if config.Mode != models.RuntimeModeLocal || config.BindAddress != "127.0.0.1" {
		t.Fatalf("config = %#v", config)
	}
	if !config.Capabilities[models.CapabilityTerminal] || !config.Agent.Available || config.Agent.Status != "local" {
		t.Fatalf("local capabilities/agent = %#v %#v", config.Capabilities, config.Agent)
	}
}

func TestResolveRuntimeConfigCloudDefaultsToPublicBindAndAgentOffline(t *testing.T) {
	config, err := ResolveRuntimeConfigFromEnv(func(key string) string {
		switch key {
		case EnvRuntimeMode:
			return "cloud"
		case EnvCookieSecret:
			return "secret"
		case EnvPublicURL:
			return "https://cloud.example.com/"
		case EnvAdminUsers:
			return "admin@example.com, subject-1"
		case EnvTrustedProxyCIDRs:
			return "10.0.0.0/8"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.Mode != models.RuntimeModeCloud || config.BindAddress != "0.0.0.0" {
		t.Fatalf("config = %#v", config)
	}
	if config.Capabilities[models.CapabilityTerminal] || config.Capabilities[models.CapabilityGit] || config.Agent.Available || config.Agent.Status != "offline" {
		t.Fatalf("cloud capabilities/agent = %#v %#v", config.Capabilities, config.Agent)
	}
	if config.AuthMode != "oauth2_proxy" || config.CookieSecret != "secret" || config.PublicURL != "https://cloud.example.com" || len(config.AdminUsers) != 2 {
		t.Fatalf("cloud auth config = %#v", config)
	}
	if err := ValidateCloudRuntimeConfig(config); err != nil {
		t.Fatal(err)
	}
}

func TestResolveRuntimeConfigCloudRejectsUnimplementedAppOIDC(t *testing.T) {
	config, err := ResolveRuntimeConfigFromEnv(func(key string) string {
		switch key {
		case EnvRuntimeMode:
			return "cloud"
		case EnvAuthMode:
			return "app_oidc"
		case EnvCookieSecret:
			return "secret"
		case EnvOIDCIssuer:
			return "https://issuer.example.com"
		case EnvOIDCClientID:
			return "client-id"
		case EnvOIDCClientSecret:
			return "client-secret"
		case EnvPublicURL:
			return "https://cloud.example.com"
		case EnvAdminUsers:
			return "admin@example.com"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.AuthMode != "app_oidc" || config.OIDCIssuer != "https://issuer.example.com" {
		t.Fatalf("app oidc config = %#v", config)
	}
	if err := ValidateCloudRuntimeConfig(config); err == nil {
		t.Fatal("expected app oidc to be rejected until verified callback support exists")
	}
}

func TestValidateCloudRuntimeConfigRejectsIncompleteOrUnsafeSettings(t *testing.T) {
	valid := RuntimeConfig{Mode: models.RuntimeModeCloud, AuthMode: "oauth2_proxy", PublicURL: "https://cloud.example.com", CookieSecret: "secret", AdminUsers: []string{"admin@example.com"}}
	withAuth := func(mode string) RuntimeConfig { config := valid; config.AuthMode = mode; return config }
	withProxies := func(cidr string) RuntimeConfig {
		config := valid
		config.TrustedProxyCIDRs = []string{cidr}
		return config
	}
	for name, config := range map[string]RuntimeConfig{
		"missing deployment settings": {Mode: models.RuntimeModeCloud},
		"unknown auth mode":           withAuth("basic"),
		"catch-all IPv4 proxy":        withProxies("0.0.0.0/0"),
		"catch-all IPv6 proxy":        withProxies("::/0"),
	} {
		if err := ValidateCloudRuntimeConfig(config); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
}

func TestResolveRuntimeConfigRejectsInvalidMode(t *testing.T) {
	_, err := ResolveRuntimeConfigFromEnv(func(key string) string {
		if key == EnvRuntimeMode {
			return "agent"
		}
		return ""
	})
	if err == nil {
		t.Fatal("expected invalid mode rejection")
	}
}

func TestResolveRuntimeConfigUsesExplicitBindAddress(t *testing.T) {
	config, err := ResolveRuntimeConfigFromEnv(func(key string) string {
		if key == EnvBindAddress {
			return "127.0.0.2"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.BindAddress != "127.0.0.2" {
		t.Fatalf("bind address = %q", config.BindAddress)
	}
}
