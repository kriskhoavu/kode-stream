package knowledge

import (
	"bytes"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const KnowledgeSettingsFile = "knowledge-settings.yaml"

const RoleJourneys = "journeys"

const taxonomySettingsVersion = 1

// An unknown role is dropped, so a typo cannot silently disable journey lookup.
var knownTaxonomyRoles = map[string]bool{RoleJourneys: true}

type TaxonomySettings struct {
	Version  int             `yaml:"version"`
	Taxonomy TaxonomySection `yaml:"taxonomy"`
}

type TaxonomySection struct {
	Tiers   []string         `yaml:"tiers"`
	Buckets []TaxonomyBucket `yaml:"buckets"`
}

type TaxonomyBucket struct {
	Path string `yaml:"path"`
	Role string `yaml:"role"`
}

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

// Declared tiers replace detection, so a curator can remove a wrongly detected tier.
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

// Warn, not fail: the curator may create the directory after editing the settings.
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
