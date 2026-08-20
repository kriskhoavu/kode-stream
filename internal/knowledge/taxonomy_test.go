package knowledge

import (
	"reflect"
	"sort"
	"testing"
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
