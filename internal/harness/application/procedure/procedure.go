// Package procedure reads a repository's own procedure for the harness's
// roles: `.claude/harness.yaml` maps a role to the skill, and optionally the
// track of that skill, which the role follows in that repository. A role the
// file does not map keeps the harness's built-in instructions.
package procedure

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"kode-stream/internal/harness/domain"
)

// Path is where a repository keeps the file, relative to its root.
const Path = ".claude/harness.yaml"

const maxFileBytes = 16 << 10

// Step is the skill, and the track of it, one role follows.
type Step struct {
	Skill string `yaml:"skill"`
	Track string `yaml:"track"`
}

// Map is the procedure of one repository, by role.
type Map map[domain.Role]Step

type file struct {
	Version int             `yaml:"version"`
	Roles   map[string]Step `yaml:"roles"`
}

var (
	skillName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	trackName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _-]{0,39}$`)
	roles     = map[string]domain.Role{
		string(domain.RolePlanner):      domain.RolePlanner,
		string(domain.RolePlanReviewer): domain.RolePlanReviewer,
		string(domain.RoleCoder):        domain.RoleCoder,
		string(domain.RoleCodeReviewer): domain.RoleCodeReviewer,
		string(domain.RoleQAFixer):      domain.RoleQAFixer,
	}
)

// Load reads the procedure of the repository checked out at dir. A missing
// file is no procedure and no error. A file that does not parse, names an
// unknown version or role, or names a skill or track outside the allowed
// characters is an error, and the caller keeps the built-in instructions.
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
		role, known := roles[key]
		if !known {
			return nil, fmt.Errorf("%s: role %q is not a harness role", Path, key)
		}
		step.Skill, step.Track = strings.TrimSpace(step.Skill), strings.TrimSpace(step.Track)
		if !skillName.MatchString(step.Skill) {
			return nil, fmt.Errorf("%s: role %s names the skill %q", Path, key, step.Skill)
		}
		if step.Track != "" && !trackName.MatchString(step.Track) {
			return nil, fmt.Errorf("%s: role %s names the track %q", Path, key, step.Track)
		}
		result[role] = step
	}
	return result, nil
}
