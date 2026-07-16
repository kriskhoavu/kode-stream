package agent

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ParseConnectToken accepts either a raw connect token or a kodestream://connect deep link.
func ParseConnectToken(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("connect token is required")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" {
		return value, nil
	}
	if parsed.Scheme != "kodestream" || parsed.Host != "connect" {
		return "", fmt.Errorf("unsupported connect link %q", parsed.Scheme+"://"+parsed.Host)
	}
	token := strings.TrimSpace(parsed.Query().Get("token"))
	if token == "" {
		return "", errors.New("connect link is missing token")
	}
	return token, nil
}

func ChannelURL(cloudURL, token string) (string, error) {
	cloudURL = strings.TrimSpace(cloudURL)
	if cloudURL == "" {
		return "", errors.New("cloud URL is required")
	}
	parsed, err := url.Parse(cloudURL)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("cloud URL is invalid: %q", cloudURL)
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("cloud URL scheme %q is unsupported", parsed.Scheme)
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	parsed.Path = basePath + "/api/agents/channel"
	query := parsed.Query()
	query.Set("token", strings.TrimSpace(token))
	parsed.RawQuery = query.Encode()
	parsed.Fragment = ""
	return parsed.String(), nil
}
