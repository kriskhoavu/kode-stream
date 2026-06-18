package scanner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"plan-manager/internal/models"
)

const RepositorySettingsFile = "repository-settings.yaml"

var settingVariablePattern = regexp.MustCompile(`^\{([A-Za-z][A-Za-z0-9_]*)\}$`)

func DefaultRepositorySettings() models.RepositorySettings {
	return models.RepositorySettings{
		Version: 1,
		Cards: []models.RepositorySettingsCard{{
			PathPattern: "{service}/feature/{ticket}",
			Fields: models.RepositorySettingsFields{
				Service: "{service}",
				Ticket:  "{ticket}",
				Title:   "readme_heading",
				Status:  "draft",
				Tags:    []string{"docs"},
			},
		}},
	}
}

func ReadRepositorySettings(root string) (models.RepositorySettings, bool, []models.ScanWarning) {
	path := filepath.Join(root, RepositorySettingsFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultRepositorySettings(), false, nil
	}
	if err != nil {
		return DefaultRepositorySettings(), false, []models.ScanWarning{{PlanPath: filepath.ToSlash(path), Message: err.Error()}}
	}
	var settings models.RepositorySettings
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return DefaultRepositorySettings(), true, []models.ScanWarning{{PlanPath: RepositorySettingsFile, Message: "invalid repository settings: " + err.Error()}}
	}
	warnings := ValidateRepositorySettings(settings)
	return settings, true, warnings
}

func WriteRepositorySettings(root string, settings models.RepositorySettings) error {
	if warnings := ValidateRepositorySettings(settings); len(warnings) > 0 {
		return errors.New(warnings[0].Message)
	}
	data, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, RepositorySettingsFile), data, 0o644)
}

func ValidateRepositorySettings(settings models.RepositorySettings) []models.ScanWarning {
	var warnings []models.ScanWarning
	if settings.Version != 1 {
		warnings = append(warnings, models.ScanWarning{PlanPath: RepositorySettingsFile, Message: "repository settings version must be 1"})
	}
	if len(settings.Cards) == 0 {
		warnings = append(warnings, models.ScanWarning{PlanPath: RepositorySettingsFile, Message: "repository settings must define at least one card rule"})
	}
	for i, card := range settings.Cards {
		prefix := fmt.Sprintf("card rule %d", i+1)
		if strings.TrimSpace(card.PathPattern) == "" {
			warnings = append(warnings, models.ScanWarning{PlanPath: RepositorySettingsFile, Message: prefix + " pathPattern is required"})
			continue
		}
		segments, err := parsePathPattern(card.PathPattern)
		if err != nil {
			warnings = append(warnings, models.ScanWarning{PlanPath: RepositorySettingsFile, Message: prefix + " " + err.Error()})
			continue
		}
		variableNames := map[string]bool{}
		for _, segment := range segments {
			if segment.variable != "" {
				variableNames[segment.variable] = true
			}
		}
		if strings.TrimSpace(card.Fields.Service) == "" {
			warnings = append(warnings, models.ScanWarning{PlanPath: RepositorySettingsFile, Message: prefix + " fields.service is required"})
		} else if unknown := unknownTemplateVariable(card.Fields.Service, variableNames); unknown != "" {
			warnings = append(warnings, models.ScanWarning{PlanPath: RepositorySettingsFile, Message: prefix + " fields.service references unknown variable " + unknown})
		}
		if strings.TrimSpace(card.Fields.Ticket) == "" {
			warnings = append(warnings, models.ScanWarning{PlanPath: RepositorySettingsFile, Message: prefix + " fields.ticket is required"})
		} else if unknown := unknownTemplateVariable(card.Fields.Ticket, variableNames); unknown != "" {
			warnings = append(warnings, models.ScanWarning{PlanPath: RepositorySettingsFile, Message: prefix + " fields.ticket references unknown variable " + unknown})
		}
		for _, value := range append([]string{card.Fields.Title, card.Fields.Status, card.Fields.Owner}, card.Fields.Tags...) {
			if unknown := unknownTemplateVariable(value, variableNames); unknown != "" {
				warnings = append(warnings, models.ScanWarning{PlanPath: RepositorySettingsFile, Message: prefix + " references unknown variable " + unknown})
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
