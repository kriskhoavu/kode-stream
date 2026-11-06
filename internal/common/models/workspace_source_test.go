package models

import (
	"encoding/json"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWorkspaceSourceBareStringParsesWithDefaults(t *testing.T) {
	var config WorkspaceConfig
	if err := json.Unmarshal([]byte(`{"sources":["plans","wiki","docs",{"path":"notes","shownIn":["knowledge"]}]}`), &config); err != nil {
		t.Fatal(err)
	}
	want := []WorkspaceSource{
		{Path: "plans", ShownIn: []SourceSurface{SourceSurfaceWorkstream}},
		{Path: "wiki", ShownIn: []SourceSurface{SourceSurfaceKnowledge}},
		{Path: "docs", ShownIn: []SourceSurface{SourceSurfaceWorkstream, SourceSurfaceKnowledge}},
		{Path: "notes", ShownIn: []SourceSurface{SourceSurfaceKnowledge}},
	}
	if !reflect.DeepEqual(config.Sources, want) {
		t.Fatalf("sources = %#v", config.Sources)
	}
	if got := config.SourcesShownIn(SourceSurfaceWorkstream); !reflect.DeepEqual(got, []string{"plans", "docs"}) {
		t.Fatalf("workstream sources = %v", got)
	}
	if got := config.SourcesShownIn(SourceSurfaceKnowledge); !reflect.DeepEqual(got, []string{"wiki", "docs", "notes"}) {
		t.Fatalf("knowledge sources = %v", got)
	}
	if got := config.SourcePaths(); !reflect.DeepEqual(got, []string{"plans", "wiki", "docs", "notes"}) {
		t.Fatalf("paths = %v", got)
	}
}

func TestWorkspaceSourceYAMLBareStringParsesWithDefaults(t *testing.T) {
	var config WorkspaceConfig
	if err := yaml.Unmarshal([]byte("sources:\n  - plans\n  - path: wiki\n    shownIn: [workstream]\n"), &config); err != nil {
		t.Fatal(err)
	}
	want := []WorkspaceSource{
		{Path: "plans", ShownIn: []SourceSurface{SourceSurfaceWorkstream}},
		{Path: "wiki", ShownIn: []SourceSurface{SourceSurfaceWorkstream}},
	}
	if !reflect.DeepEqual(config.Sources, want) {
		t.Fatalf("sources = %#v", config.Sources)
	}
	out, err := yaml.Marshal(config.Sources)
	if err != nil {
		t.Fatal(err)
	}
	var again []WorkspaceSource
	if err := yaml.Unmarshal(out, &again); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again, want) {
		t.Fatalf("round trip = %#v", again)
	}
}

func TestWorkspaceSourceMarshalsObjectForm(t *testing.T) {
	out, err := json.Marshal(SourceList("plans"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `[{"path":"plans","shownIn":["workstream"]}]` {
		t.Fatalf("json = %s", out)
	}
}

func TestWorkspaceSourceNormalizedRefusesUnknownSurface(t *testing.T) {
	if _, err := (WorkspaceSource{Path: "docs", ShownIn: []SourceSurface{"board"}}).Normalized(); err == nil {
		t.Fatal("expected an error")
	}
	normalized, err := (WorkspaceSource{Path: "docs", ShownIn: []SourceSurface{SourceSurfaceKnowledge, SourceSurfaceWorkstream, SourceSurfaceKnowledge}}).Normalized()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(normalized.ShownIn, []SourceSurface{SourceSurfaceWorkstream, SourceSurfaceKnowledge}) {
		t.Fatalf("shownIn = %v", normalized.ShownIn)
	}
}

func TestDefaultShownInMatchesTheDirectoryName(t *testing.T) {
	if got := DefaultShownIn("docs/plans"); !reflect.DeepEqual(got, []SourceSurface{SourceSurfaceWorkstream}) {
		t.Fatalf("docs/plans = %v", got)
	}
	if got := DefaultShownIn("wiki/"); !reflect.DeepEqual(got, []SourceSurface{SourceSurfaceKnowledge}) {
		t.Fatalf("wiki/ = %v", got)
	}
}
