package jira

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func (c *Client) resolveToken(name string) (string, error) {
	token := strings.TrimSpace(c.getenv(name))
	if token != "" {
		return token, nil
	}
	for _, candidate := range c.credentialFiles {
		path, err := c.expandHome(candidate)
		if err != nil {
			return "", err
		}
		value, ok, err := c.loadTokenFromFile(path, name)
		if err != nil {
			return "", err
		}
		if ok {
			return value, nil
		}
	}
	return "", nil
}

func (c *Client) expandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := c.homeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func (c *Client) loadTokenFromFile(path, name string) (string, bool, error) {
	data, err := c.readFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		value, ok := parseEnvAssignment(line, name)
		if ok {
			return value, true, nil
		}
	}
	return "", false, nil
}

func parseEnvAssignment(line, name string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	if !strings.HasPrefix(trimmed, "export ") {
		return "", false
	}
	trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "export "))
	key, raw, ok := strings.Cut(trimmed, "=")
	if !ok || strings.TrimSpace(key) != name {
		return "", false
	}
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", true
	}
	if len(value) >= 2 {
		switch value[0] {
		case '"':
			if value[len(value)-1] == '"' {
				return value[1 : len(value)-1], true
			}
		case '\'':
			if value[len(value)-1] == '\'' {
				return value[1 : len(value)-1], true
			}
		}
	}
	if idx := strings.Index(value, " #"); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}
	return value, true
}
