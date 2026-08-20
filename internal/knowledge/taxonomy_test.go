package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"kode-stream/internal/common/models"
)

// referenceWikiPaths mirrors the shapes of the reference Wiki Root: tiers
// recurring inside `domains`, an area nested in an area under `master-data`,
// a bucket whose tiers carry no area, and a bucket with no tier at all.
func referenceWikiPaths() []string {
	return []string{
		"index.md",
		"conventions.md",
		"domains/README.md",
		"domains/offer/README.md",
		"domains/offer/concepts/approval.md",
		"domains/offer/reference/permissions.md",
		"domains/user/concepts/roles.md",
		"domains/user/reference/api.md",
		"domains/pricing/concepts/rules.md",
		"domains/pricing/reference/tables.md",
		"domains/master-data/README.md",
		"domains/master-data/article/concepts/lifecycle.md",
		"domains/master-data/article/reference/spec.md",
		"domains/master-data/customer/concepts/dedup.md",
		"domains/master-data/customer/reference/fields.md",
		"cross-functional/a12/concepts/forms.md",
		"cross-functional/a12/reference/widgets.md",
		"platform/concepts/deployment-rollback.md",
		"platform/reference/database-tuning.md",
		"e2e-testing/base-setup.md",
		"e2e-testing/offer/approval.md",
		"e2e-testing/offer/creation.md",
		"e2e-testing/master-data/rebuild-line-item-index.md",
	}
}

func sortedTiers(taxonomy Taxonomy) []string {
	names := make([]string, 0, len(taxonomy.Tiers))
	for name := range taxonomy.Tiers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func TestDetectTaxonomyFindsRecurringTierNames(t *testing.T) {
	taxonomy := DetectTaxonomy(referenceWikiPaths())

	want := []string{"concepts", "reference"}
	if got := sortedTiers(taxonomy); !reflect.DeepEqual(got, want) {
		t.Fatalf("tiers = %#v, want %#v", got, want)
	}
}

// offer and master-data each appear under two buckets. Counting parents across
// the whole tree would reach the threshold and promote them to tiers; counting
// within a single bucket must not.
func TestDetectTaxonomyIgnoresNamesRecurringOnlyAcrossBuckets(t *testing.T) {
	taxonomy := DetectTaxonomy(referenceWikiPaths())

	for _, name := range []string{"offer", "master-data", "article", "customer", "a12"} {
		if taxonomy.Tiers[name] {
			t.Fatalf("%q classified as a tier; it recurs across buckets, not within one", name)
		}
	}
}

func TestDetectTaxonomyOnEmptyPathSet(t *testing.T) {
	taxonomy := DetectTaxonomy(nil)

	if len(taxonomy.Tiers) != 0 {
		t.Fatalf("tiers = %#v, want empty", taxonomy.Tiers)
	}
	bucket, area, tier := taxonomy.Classify("domains/offer/concepts/approval.md")
	if bucket != "domains" || area != "offer/concepts" || tier != "" {
		t.Fatalf("Classify() = %q, %q, %q; with no detected tier every segment is area", bucket, area, tier)
	}
}

// A wiki early in its life has no name recurring within a bucket, so detection
// declines rather than guessing a tier from a single example.
func TestDetectTaxonomyDeclinesWithoutRepeatedEvidence(t *testing.T) {
	taxonomy := DetectTaxonomy([]string{
		"index.md",
		"domains/offer/README.md",
		"domains/offer/concepts/approval.md",
	})

	if len(taxonomy.Tiers) != 0 {
		t.Fatalf("tiers = %#v, want empty", sortedTiers(taxonomy))
	}
}

func TestClassifyResolvesStructuralShapes(t *testing.T) {
	taxonomy := DetectTaxonomy(referenceWikiPaths())

	tests := []struct {
		name   string
		path   string
		bucket string
		area   string
		tier   string
	}{
		{"root page has no bucket", "index.md", "", "", ""},
		{"bucket landing page", "domains/README.md", "domains", "", ""},
		{"area landing page", "domains/offer/README.md", "domains", "offer", ""},
		{"area then tier", "domains/offer/concepts/approval.md", "domains", "offer", "concepts"},
		{"area nested in area", "domains/master-data/article/reference/spec.md", "domains", "master-data/article", "reference"},
		{"tier with no area", "platform/concepts/deployment-rollback.md", "platform", "", "concepts"},
		{"bucket without tiers", "e2e-testing/offer/approval.md", "e2e-testing", "offer", ""},
		{"page directly in a bucket", "e2e-testing/base-setup.md", "e2e-testing", "", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bucket, area, tier := taxonomy.Classify(test.path)
			if bucket != test.bucket || area != test.area || tier != test.tier {
				t.Fatalf("Classify(%q) = %q, %q, %q; want %q, %q, %q",
					test.path, bucket, area, tier, test.bucket, test.area, test.tier)
			}
		})
	}
}

// `concepts` earns tier status from the evidence inside `domains`. The tier set
// is global once learned, so it is recognised in `platform` too, where it
// appears under a single parent.
func TestClassifyAppliesGloballyLearnedTierInsideAnyBucket(t *testing.T) {
	taxonomy := DetectTaxonomy(referenceWikiPaths())

	bucket, area, tier := taxonomy.Classify("cross-functional/a12/concepts/forms.md")
	if bucket != "cross-functional" || area != "a12" || tier != "concepts" {
		t.Fatalf("Classify() = %q, %q, %q", bucket, area, tier)
	}
}

// A layout deeper than the reference wiki must degrade into a longer area
// rather than silently dropping a path component.
func TestClassifyFoldsSegmentsAfterTierIntoArea(t *testing.T) {
	taxonomy := DetectTaxonomy(referenceWikiPaths())

	bucket, area, tier := taxonomy.Classify("domains/offer/concepts/pricing/margin.md")
	if bucket != "domains" || area != "offer/pricing" || tier != "concepts" {
		t.Fatalf("Classify() = %q, %q, %q; want domains, offer/pricing, concepts", bucket, area, tier)
	}
}

func TestClassifyNormalisesPathSeparatorsAndPrefixes(t *testing.T) {
	taxonomy := DetectTaxonomy(referenceWikiPaths())

	bucket, area, tier := taxonomy.Classify("./domains/offer/concepts/approval.md")
	if bucket != "domains" || area != "offer" || tier != "concepts" {
		t.Fatalf("Classify() = %q, %q, %q", bucket, area, tier)
	}
}

// writeReferenceWiki lays out the reference wiki shapes on disk: tiers
// recurring inside `domains`, an area nested in an area, a bucket whose tier
// carries no area, and a bucket with no tier.
func writeReferenceWiki(t *testing.T, root string) {
	t.Helper()
	for _, page := range referenceWikiPaths() {
		slug := strings.ReplaceAll(strings.TrimSuffix(page, ".md"), "/", "-")
		writePage(t, root, "wiki/"+page, slug, slug, "")
	}
}

func pageBySlug(t *testing.T, wiki KnowledgeWiki, slug string) KnowledgePage {
	t.Helper()
	for _, page := range wiki.Pages {
		if page.Slug == slug {
			return page
		}
	}
	t.Fatalf("page %q not found", slug)
	return KnowledgePage{}
}

func detectReferenceWiki(t *testing.T, root string) KnowledgeWiki {
	t.Helper()
	wikis, err := NewDetector().DetectWorkspace(context.Background(), models.WorkspaceConfig{
		ID: "ws", Path: root, Sources: []string{"wiki"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(wikis) != 1 {
		t.Fatalf("wikis = %#v, want exactly one", wikis)
	}
	return wikis[0]
}

func TestDetectorClassifiesPagesIntoBucketAreaAndTier(t *testing.T) {
	root := t.TempDir()
	writeReferenceWiki(t, root)

	wiki := detectReferenceWiki(t, root)

	tests := []struct {
		slug   string
		bucket string
		area   string
		tier   string
	}{
		{"index", "", "", ""},
		{"domains-README", "domains", "", ""},
		{"domains-offer-concepts-approval", "domains", "offer", "concepts"},
		{"domains-master-data-article-reference-spec", "domains", "master-data/article", "reference"},
		{"platform-concepts-deployment-rollback", "platform", "", "concepts"},
		{"e2e-testing-offer-approval", "e2e-testing", "offer", ""},
		{"e2e-testing-base-setup", "e2e-testing", "", ""},
	}
	for _, test := range tests {
		page := pageBySlug(t, wiki, test.slug)
		if page.Bucket != test.bucket || page.Area != test.area || page.Tier != test.tier {
			t.Fatalf("%s = bucket %q, area %q, tier %q; want %q, %q, %q",
				test.slug, page.Bucket, page.Area, page.Tier, test.bucket, test.area, test.tier)
		}
	}
}

// PM-040 must not disturb the field the browser tree and graph filter read.
func TestDetectorLeavesDomainUnchanged(t *testing.T) {
	root := t.TempDir()
	writeReferenceWiki(t, root)

	wiki := detectReferenceWiki(t, root)

	for _, page := range wiki.Pages {
		want := domainForPath(page.Path)
		if page.Domain != want {
			t.Fatalf("page %q domain = %q, want %q", page.Slug, page.Domain, want)
		}
	}
}

func TestDetectorAppliesSettingsOverrideToTiers(t *testing.T) {
	root := t.TempDir()
	writeReferenceWiki(t, root)
	writeWikiSettings(t, root, "version: 1\ntaxonomy:\n  tiers: [concepts]\n")

	wiki := detectReferenceWiki(t, root)

	page := pageBySlug(t, wiki, "domains-offer-reference-permissions")
	if page.Tier != "" || page.Area != "offer/reference" {
		t.Fatalf("reference should no longer be a tier: area %q, tier %q", page.Area, page.Tier)
	}
	if concepts := pageBySlug(t, wiki, "domains-offer-concepts-approval"); concepts.Tier != "concepts" {
		t.Fatalf("concepts tier = %q, want concepts", concepts.Tier)
	}
}

// A broken settings file must never blank a wiki.
func TestDetectorIndexesEveryPageWhenSettingsAreMalformed(t *testing.T) {
	root := t.TempDir()
	writeReferenceWiki(t, root)
	writeWikiSettings(t, root, "version: 2\n")

	wiki := detectReferenceWiki(t, root)

	if len(wiki.Pages) != len(referenceWikiPaths()) {
		t.Fatalf("pages = %d, want %d", len(wiki.Pages), len(referenceWikiPaths()))
	}
	if page := pageBySlug(t, wiki, "domains-offer-concepts-approval"); page.Tier != "concepts" {
		t.Fatalf("detection should still apply: tier = %q", page.Tier)
	}
	found := false
	for _, warning := range wiki.Warnings {
		if warning.Path == KnowledgeSettingsFile && warning.Code == WarningInvalidMetadata {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %#v, want a settings warning", wiki.Warnings)
	}
}

func writeWikiSettings(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "wiki", KnowledgeSettingsFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
