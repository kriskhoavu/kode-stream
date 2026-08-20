package knowledge

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeSettings(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, KnowledgeSettingsFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func warningCodes(warnings []KnowledgeWarning) []WarningCode {
	codes := make([]WarningCode, 0, len(warnings))
	for _, warning := range warnings {
		codes = append(codes, warning.Code)
	}
	return codes
}

func TestReadTaxonomySettingsAbsentFileIsNotAnError(t *testing.T) {
	settings, present, warnings := ReadTaxonomySettings(t.TempDir())

	if present {
		t.Fatalf("present = true for a directory with no settings file")
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
	if len(settings.Taxonomy.Tiers) != 0 || len(settings.Taxonomy.Buckets) != 0 {
		t.Fatalf("settings = %#v, want zero value", settings)
	}
}

func TestReadTaxonomySettingsReadsTiersAndRoles(t *testing.T) {
	root := writeSettings(t, `version: 1
taxonomy:
  tiers: [concepts, reference]
  buckets:
    - path: journeys
      role: journeys
`)

	settings, present, warnings := ReadTaxonomySettings(root)

	if !present {
		t.Fatalf("present = false for a valid settings file")
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
	if want := []string{"concepts", "reference"}; !reflect.DeepEqual(settings.Taxonomy.Tiers, want) {
		t.Fatalf("tiers = %#v, want %#v", settings.Taxonomy.Tiers, want)
	}
	if len(settings.Taxonomy.Buckets) != 1 || settings.Taxonomy.Buckets[0].Path != "journeys" || settings.Taxonomy.Buckets[0].Role != RoleJourneys {
		t.Fatalf("buckets = %#v", settings.Taxonomy.Buckets)
	}
}

// Every defect degrades to detection and reports why. A broken configuration
// must never blank a Wiki.
func TestReadTaxonomySettingsDegradesOnFileLevelDefects(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"malformed yaml", "version: 1\ntaxonomy: [this is not a mapping\n"},
		{"wrong version", "version: 2\ntaxonomy:\n  tiers: [concepts]\n"},
		{"unknown field", "version: 1\ntaxonomy:\n  tiers: [concepts]\nnonsense: true\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, present, warnings := ReadTaxonomySettings(writeSettings(t, test.body))

			if present {
				t.Fatalf("present = true; a file-level defect must fall back to detection")
			}
			if got := warningCodes(warnings); len(got) == 0 {
				t.Fatalf("warnings = %#v, want at least one", warnings)
			} else if got[0] != WarningInvalidMetadata {
				t.Fatalf("warning code = %q, want %q", got[0], WarningInvalidMetadata)
			}
		})
	}
}

func TestReadTaxonomySettingsDropsUnknownRoleAndKeepsTheRest(t *testing.T) {
	root := writeSettings(t, `version: 1
taxonomy:
  buckets:
    - path: nonsense
      role: not-a-role
    - path: journeys
      role: journeys
`)

	settings, present, warnings := ReadTaxonomySettings(root)

	if !present {
		t.Fatalf("present = false; an entry-level defect must keep the valid entries")
	}
	if len(warnings) != 1 || warnings[0].Code != WarningInvalidMetadata {
		t.Fatalf("warnings = %#v, want one invalid_metadata", warnings)
	}
	if len(settings.Taxonomy.Buckets) != 1 || settings.Taxonomy.Buckets[0].Path != "journeys" {
		t.Fatalf("buckets = %#v, want only the journeys entry", settings.Taxonomy.Buckets)
	}
}

func TestReadTaxonomySettingsWarnsOnDuplicateBucketPathAndKeepsTheLast(t *testing.T) {
	root := writeSettings(t, `version: 1
taxonomy:
  buckets:
    - path: e2e-testing
      role: journeys
    - path: e2e-testing
      role: journeys
`)

	settings, _, warnings := ReadTaxonomySettings(root)

	if len(warnings) != 1 || warnings[0].Code != WarningInvalidMetadata {
		t.Fatalf("warnings = %#v, want one invalid_metadata", warnings)
	}
	if len(settings.Taxonomy.Buckets) != 1 {
		t.Fatalf("buckets = %#v, want the duplicate collapsed", settings.Taxonomy.Buckets)
	}
}

func TestReadTaxonomySettingsWarningsCarryTheSettingsFilePath(t *testing.T) {
	_, _, warnings := ReadTaxonomySettings(writeSettings(t, "version: 9\n"))

	if len(warnings) == 0 || warnings[0].Path != KnowledgeSettingsFile {
		t.Fatalf("warnings = %#v, want Path = %q", warnings, KnowledgeSettingsFile)
	}
}

// A curator listing tiers explicitly is correcting detection, so the declared
// list replaces the detected one. Merging would make a wrongly detected tier
// impossible to remove.
func TestApplyTaxonomySettingsReplacesDetectedTiers(t *testing.T) {
	detected := DetectTaxonomy(referenceWikiPaths())
	if !detected.Tiers["reference"] {
		t.Fatalf("precondition: reference should be detected as a tier")
	}

	applied := ApplyTaxonomySettings(detected, TaxonomySettings{
		Version:  1,
		Taxonomy: TaxonomySection{Tiers: []string{"concepts"}},
	})

	if want := []string{"concepts"}; !reflect.DeepEqual(sortedTiers(applied), want) {
		t.Fatalf("tiers = %#v, want %#v", sortedTiers(applied), want)
	}
}

func TestApplyTaxonomySettingsWithoutTiersKeepsDetection(t *testing.T) {
	detected := DetectTaxonomy(referenceWikiPaths())

	applied := ApplyTaxonomySettings(detected, TaxonomySettings{
		Version:  1,
		Taxonomy: TaxonomySection{Buckets: []TaxonomyBucket{{Path: "e2e-testing", Role: RoleJourneys}}},
	})

	if want := []string{"concepts", "reference"}; !reflect.DeepEqual(sortedTiers(applied), want) {
		t.Fatalf("tiers = %#v, want detection preserved %#v", sortedTiers(applied), want)
	}
	if applied.Roles["e2e-testing"] != RoleJourneys {
		t.Fatalf("roles = %#v", applied.Roles)
	}
}

func TestDetectTaxonomyRecordsBucketsSeen(t *testing.T) {
	taxonomy := DetectTaxonomy(referenceWikiPaths())

	for _, bucket := range []string{"domains", "cross-functional", "platform", "e2e-testing"} {
		if !taxonomy.Buckets[bucket] {
			t.Fatalf("bucket %q not recorded; buckets = %#v", bucket, taxonomy.Buckets)
		}
	}
	if taxonomy.Buckets[""] {
		t.Fatalf("root-level pages must not register an empty bucket")
	}
}

// The ordering between editing settings and creating the directory is not ours
// to dictate, so a declared bucket missing from the tree warns rather than
// failing.
func TestApplyTaxonomySettingsWarnsWhenDeclaredBucketIsAbsent(t *testing.T) {
	detected := DetectTaxonomy(referenceWikiPaths())

	warnings := ValidateTaxonomyBuckets(detected, TaxonomySettings{
		Version:  1,
		Taxonomy: TaxonomySection{Buckets: []TaxonomyBucket{{Path: "not-in-the-tree", Role: RoleJourneys}}},
	})

	if len(warnings) != 1 || warnings[0].Code != WarningInvalidMetadata {
		t.Fatalf("warnings = %#v, want one invalid_metadata", warnings)
	}
}

func TestApplyTaxonomySettingsAcceptsDeclaredBucketPresentInTree(t *testing.T) {
	detected := DetectTaxonomy(referenceWikiPaths())

	warnings := ValidateTaxonomyBuckets(detected, TaxonomySettings{
		Version:  1,
		Taxonomy: TaxonomySection{Buckets: []TaxonomyBucket{{Path: "e2e-testing", Role: RoleJourneys}}},
	})

	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
}
