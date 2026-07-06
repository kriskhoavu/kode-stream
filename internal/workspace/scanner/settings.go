package scanner

// Package scanner discovers and parses Workspace sources.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"plan-manager/internal/common/models"
)

const SourceStructureSettingsFile = "workspace-settings.yaml"

var settingVariablePattern = regexp.MustCompile(`^\{([A-Za-z][A-Za-z0-9_]*)\}$`)

func DefaultSourceStructureSettings() models.SourceStructureSettings {
	return models.SourceStructureSettings{
		Version: 1,
		Cards: []models.SourceStructureCard{{
			PathPattern: "{folder}/feature/{item}",
			Fields: models.SourceStructureFields{
				Source: "docs",
				Item:   "{item}",
				Title:  "readme_heading",
				Status: "draft",
				Tags:   []string{"docs"},
			},
		}},
	}
}

func BuiltInStructuredSettings() models.SourceStructureSettings {
	return models.SourceStructureSettings{
		Version: 1,
		Cards: []models.SourceStructureCard{{
			PathPattern: "{folder}/{item}",
			Fields: models.SourceStructureFields{
				Source: "items",
				Item:   "{item}",
				Title:  "readme_heading",
				Status: "draft",
				Tags:   []string{"items"},
			},
		}},
	}
}

func ReadSourceStructureSettings(root string) (models.SourceStructureSettings, bool, []models.ScanWarning) {
	return ReadSourceStructureSettingsFromReader(NewFilesystemSourceReader(filepath.Dir(root)), filepath.Base(root))
}

func ReadSourceStructureSettingsFromReader(reader SourceReader, root string) (models.SourceStructureSettings, bool, []models.ScanWarning) {
	path := filepath.ToSlash(filepath.Join(root, SourceStructureSettingsFile))
	data, err := reader.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultSourceStructureSettings(), false, nil
	}
	if err != nil {
		return DefaultSourceStructureSettings(), false, []models.ScanWarning{{ItemPath: filepath.ToSlash(path), Message: err.Error()}}
	}
	var settings models.SourceStructureSettings
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return DefaultSourceStructureSettings(), true, []models.ScanWarning{{ItemPath: SourceStructureSettingsFile, Message: "invalid workspace settings: " + err.Error()}}
	}
	warnings := ValidateSourceStructureSettings(settings)
	return settings, true, warnings
}

func SourceSettingsMode(root string) string {
	return SourceSettingsModeFromReader(NewFilesystemSourceReader(filepath.Dir(root)), filepath.Base(root))
}

func SourceSettingsModeFromReader(reader SourceReader, root string) string {
	entries, err := reader.ReadDir(root)
	if err != nil {
		return "unknown"
	}
	if hasStructuredItemChildren(reader, root, entries) {
		return "structured"
	}
	if hasMarkdownFiles(reader, root) {
		return "unstructured"
	}
	return "empty"
}

func WriteSourceStructureSettings(root string, settings models.SourceStructureSettings) error {
	settings = normalizeSourceStructureSettingsForWrite(settings)
	if warnings := ValidateSourceStructureSettings(settings); len(warnings) > 0 {
		return errors.New(warnings[0].Message)
	}
	data, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, SourceStructureSettingsFile), data, 0o644)
}

func normalizeSourceStructureSettingsForWrite(settings models.SourceStructureSettings) models.SourceStructureSettings {
	for i := range settings.Cards {
		fields := &settings.Cards[i].Fields
		fields.Source = firstNonEmpty(fields.Source, fields.Scope)
		fields.Item = firstNonEmpty(fields.Item, fields.Identifier)
		fields.Scope = ""
		fields.Identifier = ""
	}
	return settings
}

func RemoveSourceStructureSettings(root string) error {
	err := os.Remove(filepath.Join(root, SourceStructureSettingsFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func ValidateSourceStructureSettings(settings models.SourceStructureSettings) []models.ScanWarning {
	var warnings []models.ScanWarning
	if settings.Version != 1 {
		warnings = append(warnings, models.ScanWarning{ItemPath: SourceStructureSettingsFile, Message: "source structure settings version must be 1"})
	}
	if len(settings.Cards) == 0 {
		warnings = append(warnings, models.ScanWarning{ItemPath: SourceStructureSettingsFile, Message: "source structure settings must define at least one card rule"})
	}
	for i, card := range settings.Cards {
		prefix := fmt.Sprintf("card rule %d", i+1)
		if strings.TrimSpace(card.PathPattern) == "" {
			warnings = append(warnings, models.ScanWarning{ItemPath: SourceStructureSettingsFile, Message: prefix + " pathPattern is required"})
			continue
		}
		segments, err := parsePathPattern(card.PathPattern)
		if err != nil {
			warnings = append(warnings, models.ScanWarning{ItemPath: SourceStructureSettingsFile, Message: prefix + " " + err.Error()})
			continue
		}
		variableNames := map[string]bool{}
		for _, segment := range segments {
			if segment.variable != "" {
				variableNames[segment.variable] = true
			}
		}
		sourceField := firstNonEmpty(card.Fields.Source, card.Fields.Scope)
		itemField := firstNonEmpty(card.Fields.Item, card.Fields.Identifier)
		if strings.TrimSpace(sourceField) == "" {
			warnings = append(warnings, models.ScanWarning{ItemPath: SourceStructureSettingsFile, Message: prefix + " fields.source is required"})
		} else if unknown := unknownTemplateVariable(sourceField, variableNames); unknown != "" {
			warnings = append(warnings, models.ScanWarning{ItemPath: SourceStructureSettingsFile, Message: prefix + " fields.source references unknown variable " + unknown})
		}
		if strings.TrimSpace(itemField) == "" {
			warnings = append(warnings, models.ScanWarning{ItemPath: SourceStructureSettingsFile, Message: prefix + " fields.item is required"})
		} else if unknown := unknownTemplateVariable(itemField, variableNames); unknown != "" {
			warnings = append(warnings, models.ScanWarning{ItemPath: SourceStructureSettingsFile, Message: prefix + " fields.item references unknown variable " + unknown})
		}
		for _, value := range append([]string{card.Fields.Title, card.Fields.Status, card.Fields.Owner}, card.Fields.Tags...) {
			if unknown := unknownTemplateVariable(value, variableNames); unknown != "" {
				warnings = append(warnings, models.ScanWarning{ItemPath: SourceStructureSettingsFile, Message: prefix + " references unknown variable " + unknown})
			}
		}
	}
	return warnings
}

type pathPatternSegment struct {
	literal  string
	variable string
}

func parsePathPattern(pattern string) ([]pathPatternSegment, error) {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(pattern)))
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." || strings.HasPrefix(clean, "/") || filepath.IsAbs(clean) {
		return nil, fmt.Errorf("pathPattern must be a relative path")
	}
	rawSegments := strings.Split(clean, "/")
	segments := make([]pathPatternSegment, 0, len(rawSegments))
	for _, raw := range rawSegments {
		raw = strings.TrimSpace(raw)
		if raw == "" || raw == "." || raw == ".." {
			return nil, fmt.Errorf("pathPattern contains an invalid segment")
		}
		if match := settingVariablePattern.FindStringSubmatch(raw); match != nil {
			segments = append(segments, pathPatternSegment{variable: match[1]})
			continue
		}
		if strings.ContainsAny(raw, "{}*?") {
			return nil, fmt.Errorf("pathPattern segment %q must be literal text or a {variable}", raw)
		}
		segments = append(segments, pathPatternSegment{literal: raw})
	}
	return segments, nil
}

func unknownTemplateVariable(value string, known map[string]bool) string {
	for _, match := range regexp.MustCompile(`\{([A-Za-z][A-Za-z0-9_]*)\}`).FindAllStringSubmatch(value, -1) {
		if !known[match[1]] {
			return "{" + match[1] + "}"
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
