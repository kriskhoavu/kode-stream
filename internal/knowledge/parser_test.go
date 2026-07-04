package knowledge

import (
	"os"
	"reflect"
	"testing"
)

func TestParsePageNormalizesMetadataAndIgnoresCodeLinks(t *testing.T) {
	data, err := os.ReadFile("testdata/offer-overview.md")
	if err != nil {
		t.Fatal(err)
	}

	page, warnings, err := ParsePage("offer/overview.md", data)
	if err != nil {
		t.Fatalf("ParsePage() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if page.Slug != "offer-overview" || page.Title != "Offer Overview" || page.Domain != "offer" {
		t.Fatalf("page identity = %#v", page)
	}
	if !reflect.DeepEqual(page.Roles, []string{"BA", "DEVELOPER"}) {
		t.Fatalf("roles = %#v", page.Roles)
	}
	if !reflect.DeepEqual(page.Topics, []string{"offer", "workflow"}) {
		t.Fatalf("topics = %#v", page.Topics)
	}
	if len(page.SourceRefs) != 2 || page.SourceCount != 2 {
		t.Fatalf("sources = %#v count=%d", page.SourceRefs, page.SourceCount)
	}
	if len(page.Links) != 3 {
		t.Fatalf("links = %#v", page.Links)
	}
	if page.Links[0].RawTarget != "offer-creation" || page.Links[0].Label != "Creation" {
		t.Fatalf("wiki link = %#v", page.Links[0])
	}
	if page.Links[2].RawTarget != "approval.md" {
		t.Fatalf("markdown link = %#v", page.Links[2])
	}
}

func TestParsePageRejectsMissingIdentity(t *testing.T) {
	_, warnings, err := ParsePage("bad.md", []byte("---\ntitle: Missing slug\n---\nbody"))
	if err == nil {
		t.Fatal("expected missing identity error")
	}
	if len(warnings) != 1 || warnings[0].Code != WarningMissingIdentity {
		t.Fatalf("warnings = %#v", warnings)
	}
}

func TestResolveRelationshipsBuildsDeterministicBacklinksAndGraph(t *testing.T) {
	pages := []KnowledgePage{
		{Slug: "target", Title: "Target", Path: "guide/target.md", Domain: "guide", Roles: []string{}, Topics: []string{}, Links: []KnowledgeLink{}},
		{Slug: "source", Title: "Source", Path: "guide/source.md", Domain: "guide", Roles: []string{}, Topics: []string{}, Links: []KnowledgeLink{
			{SourceSlug: "source", RawTarget: "target", Resolution: LinkUnresolved},
			{SourceSlug: "source", RawTarget: "target.md", Resolution: LinkUnresolved},
			{SourceSlug: "source", RawTarget: "missing", Resolution: LinkUnresolved},
		}},
		{Slug: "target", Title: "Duplicate", Path: "z/duplicate.md", Domain: "z", Roles: []string{}, Topics: []string{}, Links: []KnowledgeLink{}},
	}

	resolved, warnings, graph := ResolveRelationships(pages)
	if resolved[0].Slug != "source" {
		t.Fatalf("pages are not path sorted: %#v", resolved)
	}
	if !reflect.DeepEqual(resolved[1].Backlinks, []string{"source"}) {
		t.Fatalf("backlinks = %#v", resolved[1].Backlinks)
	}
	if len(warnings) != 2 || warnings[0].Code != WarningDuplicateSlug || warnings[1].Code != WarningUnresolvedLink {
		t.Fatalf("warnings = %#v", warnings)
	}
	if len(graph.Nodes) != 2 || len(graph.Edges) != 1 {
		t.Fatalf("graph = %#v", graph)
	}
	if graph.Edges[0] != (KnowledgeGraphEdge{Source: "source", Target: "target"}) {
		t.Fatalf("edge = %#v", graph.Edges[0])
	}
}
