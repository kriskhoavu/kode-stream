package knowledge

import (
	"bytes"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// KnowledgeSettingsFile is the optional per-Wiki-Root override read from the
// same directory as index.md.
const KnowledgeSettingsFile = "knowledge-settings.yaml"

// RoleJourneys marks the bucket holding reusable E2E journeys.
const RoleJourneys = "journeys"

// taxonomySettingsVersion is the only supported schema version.
const taxonomySettingsVersion = 1

// knownTaxonomyRoles gates the role values a settings file may declare. An
// unrecognised role is dropped rather than stored, so a typo cannot silently
// disable journey lookup.
var knownTaxonomyRoles = map[string]bool{RoleJourneys: true}

// TaxonomySettings is the parsed knowledge-settings.yaml.
type TaxonomySettings struct {
	Version  int             `yaml:"version"`
	Taxonomy TaxonomySection `yaml:"taxonomy"`
}

// TaxonomySection carries the declared tier names and bucket roles.
type TaxonomySection struct {
	Tiers   []string         `yaml:"tiers"`
	Buckets []TaxonomyBucket `yaml:"buckets"`
}

// TaxonomyBucket attaches a role to a bucket directory.
type TaxonomyBucket struct {
	Path string `yaml:"path"`
	Role string `yaml:"role"`
}

// ReadTaxonomySettings loads the override for one Wiki Root. An absent file is
// the normal case and yields no warning. Every defect degrades rather than
// failing: a file-level problem falls back to pure detection, and an
// entry-level problem drops just that entry.
func ReadTaxonomySettings(root string) (TaxonomySettings, bool, []KnowledgeWarning) {
	data, err := os.ReadFile(filepath.Join(root, KnowledgeSettingsFile))
	if os.IsNotExist(err) {
		return TaxonomySettings{}, false, nil
	}
	if err != nil {
		return TaxonomySettings{}, false, []KnowledgeWarning{settingsWarning("settings file could not be read: " + err.Error())}
	}
	var settings TaxonomySettings
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&settings); err != nil {
		return TaxonomySettings{}, false, []KnowledgeWarning{settingsWarning("invalid knowledge settings: " + err.Error())}
	}
	if settings.Version != taxonomySettingsVersion {
		return TaxonomySettings{}, false, []KnowledgeWarning{settingsWarning("knowledge settings version must be 1")}
	}
	buckets := make([]TaxonomyBucket, 0, len(settings.Taxonomy.Buckets))
	positionByPath := make(map[string]int, len(settings.Taxonomy.Buckets))
	warnings := make([]KnowledgeWarning, 0)
	for _, bucket := range settings.Taxonomy.Buckets {
		if !knownTaxonomyRoles[bucket.Role] {
			warnings = append(warnings, settingsWarning("bucket "+bucket.Path+" declares unknown role "+bucket.Role))
			continue
		}
		if position, exists := positionByPath[bucket.Path]; exists {
			warnings = append(warnings, settingsWarning("bucket "+bucket.Path+" is declared more than once; the last entry wins"))
			buckets[position] = bucket
			continue
		}
		positionByPath[bucket.Path] = len(buckets)
		buckets = append(buckets, bucket)
	}
	settings.Taxonomy.Buckets = buckets
	return settings, true, warnings
}

// ApplyTaxonomySettings folds a settings override into a detected taxonomy.
// Declared tiers replace the detected set outright, because a curator listing
// tiers is correcting detection; merging would make a wrongly detected tier
// impossible to remove. Roles have nothing to merge with and are simply added.
func ApplyTaxonomySettings(taxonomy Taxonomy, settings TaxonomySettings) Taxonomy {
	applied := Taxonomy{
		Tiers:   taxonomy.Tiers,
		Buckets: taxonomy.Buckets,
		Roles:   make(map[string]string, len(settings.Taxonomy.Buckets)),
	}
	if len(settings.Taxonomy.Tiers) > 0 {
		applied.Tiers = make(map[string]bool, len(settings.Taxonomy.Tiers))
		for _, tier := range settings.Taxonomy.Tiers {
			applied.Tiers[tier] = true
		}
	}
	for _, bucket := range settings.Taxonomy.Buckets {
		applied.Roles[bucket.Path] = bucket.Role
	}
	return applied
}

// ValidateTaxonomyBuckets reports declared buckets that the tree does not
// contain. This warns rather than fails: the ordering between editing settings
// and creating the directory is the curator's to choose.
func ValidateTaxonomyBuckets(taxonomy Taxonomy, settings TaxonomySettings) []KnowledgeWarning {
	warnings := make([]KnowledgeWarning, 0)
	for _, bucket := range settings.Taxonomy.Buckets {
		if !taxonomy.Buckets[bucket.Path] {
			warnings = append(warnings, settingsWarning("bucket "+bucket.Path+" is declared but absent from the wiki"))
		}
	}
	return warnings
}

func settingsWarning(message string) KnowledgeWarning {
	return KnowledgeWarning{Path: KnowledgeSettingsFile, Code: WarningInvalidMetadata, Message: message}
}
