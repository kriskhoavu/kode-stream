package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJourneysBucketDefaultsToE2ETesting(t *testing.T) {
	if got := (KnowledgeWiki{}).journeysBucket(); got != DefaultJourneysBucket {
		t.Fatalf("journeysBucket() = %q, want %q", got, DefaultJourneysBucket)
	}
	if got := (KnowledgeWiki{JourneysBucket: "journeys"}).journeysBucket(); got != "journeys" {
		t.Fatalf("journeysBucket() = %q, want the declared bucket", got)
	}
}

func TestPageIsJourneyMatchesTheResolvedBucket(t *testing.T) {
	tests := []struct {
		name    string
		wiki    KnowledgeWiki
		page    KnowledgePage
		journey bool
	}{
		{
			name:    "default bucket",
			page:    KnowledgePage{Bucket: "e2e-testing", Path: "e2e-testing/offer/approval.md"},
			journey: true,
		},
		{
			name:    "another bucket is not a journey",
			page:    KnowledgePage{Bucket: "domains", Path: "domains/offer/concepts/approval.md"},
			journey: false,
		},
		{
			name:    "declared bucket replaces the default",
			wiki:    KnowledgeWiki{JourneysBucket: "journeys"},
			page:    KnowledgePage{Bucket: "journeys", Path: "journeys/offer/approval.md"},
			journey: true,
		},
		{
			name:    "default no longer matches once a bucket is declared",
			wiki:    KnowledgeWiki{JourneysBucket: "journeys"},
			page:    KnowledgePage{Bucket: "e2e-testing", Path: "e2e-testing/offer/approval.md"},
			journey: false,
		},
		{
			name:    "sibling bucket sharing the name as a prefix",
			page:    KnowledgePage{Bucket: "e2e-testing-archive", Path: "e2e-testing-archive/old.md"},
			journey: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := pageIsJourney(test.wiki, test.page); got != test.journey {
				t.Fatalf("pageIsJourney() = %v, want %v", got, test.journey)
			}
		})
	}
}

// An index written before PM-040 carries no bucket. Journey lookup must keep
// working against it until the next rescan, so it falls back to the path.
func TestPageIsJourneyFallsBackToPathForPrePM040Index(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		journey bool
	}{
		{"bucket root", "e2e-testing", true},
		{"nested under the bucket", "e2e-testing/offer", true},
		{"unrelated domain", "domains/offer/concepts", false},
		{"sibling sharing the name as a prefix", "e2e-testing-archive", false},
		{"sibling nested", "e2e-testing-archive/old", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page := KnowledgePage{Domain: test.domain}
			if got := pageIsJourney(KnowledgeWiki{}, page); got != test.journey {
				t.Fatalf("pageIsJourney(domain=%q) = %v, want %v", test.domain, got, test.journey)
			}
		})
	}
}

func TestDetectorRecordsDefaultJourneysBucketWithoutSettings(t *testing.T) {
	root := t.TempDir()
	writeReferenceWiki(t, root)

	wiki := detectReferenceWiki(t, root)

	if wiki.JourneysBucket != DefaultJourneysBucket {
		t.Fatalf("JourneysBucket = %q, want %q", wiki.JourneysBucket, DefaultJourneysBucket)
	}
}

func TestDetectorRecordsDeclaredJourneysBucket(t *testing.T) {
	root := t.TempDir()
	writeReferenceWiki(t, root)
	if err := os.MkdirAll(filepath.Join(root, "wiki", "journeys", "offer"), 0o755); err != nil {
		t.Fatal(err)
	}
	writePage(t, root, "wiki/journeys/offer/approval.md", "journeys-offer-approval", "Approval", "")
	writeWikiSettings(t, root, "version: 1\ntaxonomy:\n  buckets:\n    - path: journeys\n      role: journeys\n")

	wiki := detectReferenceWiki(t, root)

	if wiki.JourneysBucket != "journeys" {
		t.Fatalf("JourneysBucket = %q, want journeys", wiki.JourneysBucket)
	}
	if page := pageBySlug(t, wiki, "journeys-offer-approval"); !pageIsJourney(wiki, page) {
		t.Fatalf("page in the declared bucket is not treated as a journey: %#v", page)
	}
	if page := pageBySlug(t, wiki, "e2e-testing-offer-approval"); pageIsJourney(wiki, page) {
		t.Fatalf("page in the default bucket should no longer match once a bucket is declared")
	}
}
