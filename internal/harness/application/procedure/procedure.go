// Package procedure reads `.kode-stream/harness.yaml` only to move its role skills into the workspace's settings.
package procedure

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"kode-stream/internal/harness/domain"
)

const Path = ".kode-stream/harness.yaml"

const maxFileBytes = 16 << 10

type Map map[domain.Role]domain.ProcedureStep

type file struct {
	Version int                             `yaml:"version"`
	Roles   map[string]domain.ProcedureStep `yaml:"roles"`
}

func Load(dir string) (Map, error) {
	if dir == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(filepath.Join(dir, Path))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) > maxFileBytes {
		return nil, fmt.Errorf("%s is larger than %d bytes", Path, maxFileBytes)
	}
	var parsed file
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("%s: %w", Path, err)
	}
	if parsed.Version != 1 {
		return nil, fmt.Errorf("%s: version %d is not 1", Path, parsed.Version)
	}
	result := Map{}
	for key, step := range parsed.Roles {
		step.Skill, step.Track = strings.TrimSpace(step.Skill), strings.TrimSpace(step.Track)
		if err := domain.ValidateProcedure(domain.Role(key), step); err != nil {
			return nil, fmt.Errorf("%s: %v", Path, err)
		}
		result[domain.Role(key)] = step
	}
	return result, nil
}
