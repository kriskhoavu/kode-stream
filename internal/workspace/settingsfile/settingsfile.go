// Package settingsfile reads and writes .kode-stream/settings.yaml. It holds only settings a commit may safely set.
package settingsfile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"kode-stream/internal/harness/domain"
)

const Path = ".kode-stream/settings.yaml"

const maxFileBytes = 64 << 10

const (
	PartProvider  = "provider"
	PartPipeline  = "pipeline"
	PartTests     = "tests"
	PartScope     = "scope"
	PartModels    = "models"
	PartProcedure = "procedure"
	PartSources   = "sources"
)

var Parts = []string{PartProvider, PartPipeline, PartTests, PartScope, PartModels, PartProcedure, PartSources}

var Engines = []string{"claudecode", "codex", "opencode"}

type File struct {
	Version int      `yaml:"version" json:"version"`
	AI      *AI      `yaml:"ai,omitempty" json:"ai,omitempty"`
	Harness *Harness `yaml:"harness,omitempty" json:"harness,omitempty"`
}

type AI struct {
	Provider string `yaml:"provider" json:"provider"`
}

type Harness struct {
	Enabled      *bool                                `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	AutoStart    *bool                                `yaml:"autoStart,omitempty" json:"autoStart,omitempty"`
	Review       *bool                                `yaml:"review,omitempty" json:"review,omitempty"`
	AskQuestions *bool                                `yaml:"askQuestions,omitempty" json:"askQuestions,omitempty"`
	Tests        []string                             `yaml:"tests,omitempty" json:"tests,omitempty"`
	Scope        []string                             `yaml:"scope,omitempty" json:"scope,omitempty"`
	Models       map[domain.Role]Model                `yaml:"models,omitempty" json:"models,omitempty"`
	Procedure    map[domain.Role]domain.ProcedureStep `yaml:"procedure,omitempty" json:"procedure,omitempty"`
	Sources      []Source                             `yaml:"sources,omitempty" json:"sources,omitempty"`
}

type Model struct {
	Engine    string `yaml:"engine,omitempty" json:"engine,omitempty"`
	Model     string `yaml:"model,omitempty" json:"model,omitempty"`
	HardModel string `yaml:"hardModel,omitempty" json:"hardModel,omitempty"`
	MaxTurns  int    `yaml:"maxTurns,omitempty" json:"maxTurns,omitempty"`
}

// TokenEnvVar names an environment setting, never a token.
type Source struct {
	Kind        string   `yaml:"kind" json:"kind"`
	Enabled     bool     `yaml:"enabled" json:"enabled"`
	Query       string   `yaml:"query" json:"query"`
	Labels      []string `yaml:"labels,omitempty" json:"labels,omitempty"`
	TokenEnvVar string   `yaml:"tokenEnvVar,omitempty" json:"tokenEnvVar,omitempty"`
}

var (
	providerName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	modelName    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/\[\]-]{0,127}$`)
	envName      = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,127}$`)
	sourceKinds  = []string{string(domain.SourceJira), string(domain.SourceGitHub), string(domain.SourceBitbucket), string(domain.SourceLocal)}
)

func Load(dir string) (File, bool, error) {
	path := filepath.Join(dir, Path)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return File{}, false, nil
	}
	if err != nil {
		return File{}, true, err
	}
	if !info.Mode().IsRegular() {
		return File{}, true, fmt.Errorf("%s is not a regular file", Path)
	}
	handle, err := os.Open(path)
	if err != nil {
		return File{}, true, err
	}
	defer handle.Close()
	data, err := io.ReadAll(io.LimitReader(handle, maxFileBytes+1))
	if err != nil {
		return File{}, true, err
	}
	if len(data) > maxFileBytes {
		return File{}, true, fmt.Errorf("%s is larger than %d bytes", Path, maxFileBytes)
	}
	file, err := Parse(data)
	return file, true, err
}

// An unknown key is refused, so a setting the file may not hold cannot slip in.
func Parse(data []byte) (File, error) {
	var file File
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil && !errors.Is(err, io.EOF) {
		return File{}, fmt.Errorf("%s does not parse: %w", Path, err)
	}
	return file, Validate(file)
}

func Validate(file File) error {
	if file.Version != 1 {
		return fmt.Errorf("%s: version must be 1", Path)
	}
	if file.AI != nil && !providerName.MatchString(file.AI.Provider) {
		return fmt.Errorf("%s: ai.provider %q is not a provider name", Path, file.AI.Provider)
	}
	harness := file.Harness
	if harness == nil {
		return nil
	}
	for _, command := range harness.Tests {
		if strings.TrimSpace(command) == "" || strings.ContainsAny(command, "\n\r") || len(command) > 1024 {
			return fmt.Errorf("%s: each of harness.tests must be one line", Path)
		}
	}
	for _, prefix := range harness.Scope {
		clean := strings.TrimSpace(prefix)
		if clean == "" || strings.HasPrefix(clean, "/") || strings.Contains(clean, "..") {
			return fmt.Errorf("%s: harness.scope entry %q must be a relative path inside the repository", Path, prefix)
		}
	}
	for role, model := range harness.Models {
		if !slices.Contains(domain.Roles, role) {
			return fmt.Errorf("%s: harness.models names an unknown role %q", Path, role)
		}
		if model.Engine != "" && !slices.Contains(Engines, model.Engine) {
			return fmt.Errorf("%s: harness.models.%s.engine must be one of %s", Path, role, strings.Join(Engines, ", "))
		}
		for _, name := range []string{model.Model, model.HardModel} {
			if name != "" && !modelName.MatchString(name) {
				return fmt.Errorf("%s: harness.models.%s names a model %q outside the allowed characters", Path, role, name)
			}
		}
		if model.MaxTurns < 0 || model.MaxTurns > domain.DefaultPolicyMaxTurn {
			return fmt.Errorf("%s: harness.models.%s.maxTurns must be between 1 and %d", Path, role, domain.DefaultPolicyMaxTurn)
		}
	}
	for role, step := range harness.Procedure {
		if err := domain.ValidateProcedure(role, step); err != nil {
			return fmt.Errorf("%s: %v", Path, err)
		}
	}
	kinds := map[string]bool{}
	for _, source := range harness.Sources {
		if !slices.Contains(sourceKinds, source.Kind) {
			return fmt.Errorf("%s: harness.sources kind %q must be one of %s", Path, source.Kind, strings.Join(sourceKinds, ", "))
		}
		if kinds[source.Kind] {
			return fmt.Errorf("%s: harness.sources lists %s twice", Path, source.Kind)
		}
		kinds[source.Kind] = true
		if strings.ContainsAny(source.Query, "\n\r") || len(source.Query) > 4096 {
			return fmt.Errorf("%s: harness.sources %s query must be one line", Path, source.Kind)
		}
		if source.TokenEnvVar != "" && !envName.MatchString(source.TokenEnvVar) {
			return fmt.Errorf("%s: harness.sources %s tokenEnvVar must name an environment setting, not hold a token", Path, source.Kind)
		}
	}
	return nil
}

func (f File) Has() []string {
	out := []string{}
	h := f.Harness
	for _, part := range Parts {
		present := false
		switch part {
		case PartProvider:
			present = f.AI != nil
		case PartPipeline:
			present = h != nil && (h.Enabled != nil || h.AutoStart != nil || h.Review != nil || h.AskQuestions != nil)
		case PartTests:
			present = h != nil && h.Tests != nil
		case PartScope:
			present = h != nil && h.Scope != nil
		case PartModels:
			present = h != nil && h.Models != nil
		case PartProcedure:
			present = h != nil && h.Procedure != nil
		case PartSources:
			present = h != nil && h.Sources != nil
		}
		if present {
			out = append(out, part)
		}
	}
	return out
}

func Take(into, from File, parts []string) File {
	into.Version = 1
	if into.Harness == nil {
		into.Harness = &Harness{}
	} else {
		copied := *into.Harness
		into.Harness = &copied
	}
	h := from.Harness
	if h == nil {
		h = &Harness{}
	}
	for _, part := range parts {
		switch part {
		case PartProvider:
			into.AI = from.AI
		case PartPipeline:
			into.Harness.Enabled, into.Harness.AutoStart, into.Harness.Review, into.Harness.AskQuestions = h.Enabled, h.AutoStart, h.Review, h.AskQuestions
		case PartTests:
			into.Harness.Tests = h.Tests
		case PartScope:
			into.Harness.Scope = h.Scope
		case PartModels:
			into.Harness.Models = h.Models
		case PartProcedure:
			into.Harness.Procedure = h.Procedure
		case PartSources:
			into.Harness.Sources = h.Sources
		}
	}
	if into.Harness.empty() {
		into.Harness = nil
	}
	return into
}

func (h *Harness) empty() bool {
	return h.Enabled == nil && h.AutoStart == nil && h.Review == nil && h.AskQuestions == nil && h.Tests == nil && h.Scope == nil && h.Models == nil && h.Procedure == nil && h.Sources == nil
}

func ValidParts(parts []string) error {
	if len(parts) == 0 {
		return errors.New("name at least one part")
	}
	for _, part := range parts {
		if !slices.Contains(Parts, part) {
			return fmt.Errorf("unknown part %q: the parts are %s", part, strings.Join(Parts, ", "))
		}
	}
	return nil
}

const header = "# A workspace's settings, shared through the repository. Kode Stream exports\n# chosen parts into this file and imports chosen parts from it; it never\n# reads it on its own. See Settings in Kode Stream.\n"

func Write(dir string, file File) error {
	if err := Validate(file); err != nil {
		return err
	}
	data, err := yaml.Marshal(file)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, Path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", Path)
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, append([]byte(header), data...), 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
