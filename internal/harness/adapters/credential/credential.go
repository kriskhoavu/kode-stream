// Package credential reads a token from the environment, then from export lines in ~/.creds.zsh and ~/.creds.sh.
package credential

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Resolver struct {
	Getenv   func(string) string
	ReadFile func(string) ([]byte, error)
	HomeDir  func() (string, error)
	Files    []string
}

func Default() Resolver {
	return Resolver{Getenv: os.Getenv, ReadFile: os.ReadFile, HomeDir: os.UserHomeDir, Files: []string{".creds.zsh", ".creds.sh"}}
}

func (r Resolver) Resolve(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	if r.Getenv != nil {
		if value := strings.TrimSpace(r.Getenv(name)); value != "" {
			return value, nil
		}
	}
	if r.HomeDir == nil || r.ReadFile == nil {
		return "", nil
	}
	home, err := r.HomeDir()
	if err != nil {
		return "", err
	}
	for _, file := range r.Files {
		data, err := r.ReadFile(filepath.Join(home, file))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(string(data), "\n") {
			if value, ok := exportedValue(line, name); ok {
				return value, nil
			}
		}
	}
	return "", nil
}

func exportedValue(line, name string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "export ") {
		return "", false
	}
	key, raw, ok := strings.Cut(strings.TrimSpace(strings.TrimPrefix(trimmed, "export ")), "=")
	if !ok || strings.TrimSpace(key) != name {
		return "", false
	}
	value := strings.TrimSpace(raw)
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		return value[1 : len(value)-1], true
	}
	if index := strings.Index(value, " #"); index >= 0 {
		value = strings.TrimSpace(value[:index])
	}
	return value, true
}
