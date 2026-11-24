package models

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

type SourceSurface string

const (
	SourceSurfaceWorkstream SourceSurface = "workstream"
	SourceSurfaceKnowledge  SourceSurface = "knowledge"
)

// A bare string is a path with defaults: plans shows in Workstream, wiki in Knowledge, others in both.
type WorkspaceSource struct {
	Path    string          `json:"path" yaml:"path"`
	ShownIn []SourceSurface `json:"shownIn" yaml:"shownIn"`
}

func DefaultShownIn(sourcePath string) []SourceSurface {
	switch path.Base(strings.TrimSuffix(strings.ReplaceAll(strings.TrimSpace(sourcePath), "\\", "/"), "/")) {
	case "plans":
		return []SourceSurface{SourceSurfaceWorkstream}
	case "wiki":
		return []SourceSurface{SourceSurfaceKnowledge}
	default:
		return []SourceSurface{SourceSurfaceWorkstream, SourceSurfaceKnowledge}
	}
}

func NewWorkspaceSource(sourcePath string) WorkspaceSource {
	return WorkspaceSource{Path: sourcePath, ShownIn: DefaultShownIn(sourcePath)}
}

func SourceList(paths ...string) []WorkspaceSource {
	sources := make([]WorkspaceSource, 0, len(paths))
	for _, sourcePath := range paths {
		sources = append(sources, NewWorkspaceSource(sourcePath))
	}
	return sources
}

func (s WorkspaceSource) ShowsIn(surface SourceSurface) bool {
	shownIn := s.ShownIn
	if len(shownIn) == 0 {
		shownIn = DefaultShownIn(s.Path)
	}
	for _, candidate := range shownIn {
		if candidate == surface {
			return true
		}
	}
	return false
}

func (s WorkspaceSource) Normalized() (WorkspaceSource, error) {
	if len(s.ShownIn) == 0 {
		return WorkspaceSource{Path: s.Path, ShownIn: DefaultShownIn(s.Path)}, nil
	}
	seen := map[SourceSurface]bool{}
	for _, surface := range s.ShownIn {
		switch surface {
		case SourceSurfaceWorkstream, SourceSurfaceKnowledge:
			seen[surface] = true
		default:
			return WorkspaceSource{}, fmt.Errorf("source %q: unknown surface %q", s.Path, string(surface))
		}
	}
	shownIn := make([]SourceSurface, 0, 2)
	for _, surface := range []SourceSurface{SourceSurfaceWorkstream, SourceSurfaceKnowledge} {
		if seen[surface] {
			shownIn = append(shownIn, surface)
		}
	}
	return WorkspaceSource{Path: s.Path, ShownIn: shownIn}, nil
}

type workspaceSourceRecord struct {
	Path    string          `json:"path" yaml:"path"`
	ShownIn []SourceSurface `json:"shownIn,omitempty" yaml:"shownIn,omitempty"`
}

func (s WorkspaceSource) MarshalJSON() ([]byte, error) {
	shownIn := s.ShownIn
	if len(shownIn) == 0 {
		shownIn = DefaultShownIn(s.Path)
	}
	return json.Marshal(workspaceSourceRecord{Path: s.Path, ShownIn: shownIn})
}

func (s *WorkspaceSource) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "\"") {
		var sourcePath string
		if err := json.Unmarshal(data, &sourcePath); err != nil {
			return err
		}
		*s = NewWorkspaceSource(sourcePath)
		return nil
	}
	var record workspaceSourceRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return err
	}
	*s = fromRecord(record)
	return nil
}

func (s WorkspaceSource) MarshalYAML() (any, error) {
	shownIn := s.ShownIn
	if len(shownIn) == 0 {
		shownIn = DefaultShownIn(s.Path)
	}
	return workspaceSourceRecord{Path: s.Path, ShownIn: shownIn}, nil
}

func (s *WorkspaceSource) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		var sourcePath string
		if err := node.Decode(&sourcePath); err != nil {
			return err
		}
		*s = NewWorkspaceSource(sourcePath)
		return nil
	}
	var record workspaceSourceRecord
	if err := node.Decode(&record); err != nil {
		return err
	}
	*s = fromRecord(record)
	return nil
}

func fromRecord(record workspaceSourceRecord) WorkspaceSource {
	if len(record.ShownIn) == 0 {
		return NewWorkspaceSource(record.Path)
	}
	return WorkspaceSource{Path: record.Path, ShownIn: record.ShownIn}
}

func (w WorkspaceConfig) SourcePaths() []string {
	return sourcePaths(w.Sources)
}

func (w WorkspaceConfig) SourcesShownIn(surface SourceSurface) []string {
	return sourcesShownIn(w.Sources, surface)
}

func (w WorkspaceInput) SourcePaths() []string {
	return sourcePaths(w.Sources)
}

func sourcePaths(sources []WorkspaceSource) []string {
	paths := make([]string, 0, len(sources))
	for _, source := range sources {
		paths = append(paths, source.Path)
	}
	return paths
}

func sourcesShownIn(sources []WorkspaceSource, surface SourceSurface) []string {
	paths := make([]string, 0, len(sources))
	for _, source := range sources {
		if source.ShowsIn(surface) {
			paths = append(paths, source.Path)
		}
	}
	return paths
}
