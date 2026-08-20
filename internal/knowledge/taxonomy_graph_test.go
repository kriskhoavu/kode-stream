package knowledge

import "testing"

// The graph payload must carry the taxonomy, otherwise the views cannot group
// by it and fall back to the raw domain path.
func TestBuildGraphCarriesTaxonomyOntoNodes(t *testing.T) {
	taxonomy := DetectTaxonomy(referenceWikiPaths())
	pages := []KnowledgePage{
		{Slug: "approval", Title: "Approval", Path: "domains/offer/concepts/approval.md"},
		{Slug: "spec", Title: "Spec", Path: "domains/master-data/article/reference/spec.md"},
		{Slug: "rollback", Title: "Rollback", Path: "platform/concepts/rollback.md"},
		{Slug: "wiki-index", Title: "Index", Path: "index.md"},
	}
	for index := range pages {
		pages[index].Domain = domainForPath(pages[index].Path)
		pages[index].Bucket, pages[index].Area, pages[index].Tier = taxonomy.Classify(pages[index].Path)
	}

	graph := buildGraph(pages, nil)

	bySlug := make(map[string]KnowledgeGraphNode, len(graph.Nodes))
	for _, node := range graph.Nodes {
		bySlug[node.ID] = node
	}
	tests := []struct {
		slug   string
		bucket string
		area   string
		tier   string
		domain string
	}{
		{"approval", "domains", "offer", "concepts", "domains/offer/concepts"},
		{"spec", "domains", "master-data/article", "reference", "domains/master-data/article/reference"},
		{"rollback", "platform", "", "concepts", "platform/concepts"},
		{"wiki-index", "", "", "", "root"},
	}
	for _, test := range tests {
		node := bySlug[test.slug]
		if node.Bucket != test.bucket || node.Area != test.area || node.Tier != test.tier {
			t.Fatalf("%s = bucket %q, area %q, tier %q; want %q, %q, %q",
				test.slug, node.Bucket, node.Area, node.Tier, test.bucket, test.area, test.tier)
		}
		if node.Domain != test.domain {
			t.Fatalf("%s domain = %q, want %q; domain must survive for breadcrumbs", test.slug, node.Domain, test.domain)
		}
	}
}
