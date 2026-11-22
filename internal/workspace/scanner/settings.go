package scanner

// Package scanner discovers and parses Workspace sources.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"kode-stream/internal/common/models"
)

const SourceStructureSettingsFile = "workspace-settings.yaml"

// RepositorySettingsDir holds Kode Stream's own files at the workspace root.
// Its workspace-settings.yaml holds the card rules of every source of the
// repository; a rule belongs to the source its fields.source names, by the
// source's path or its last part. A source that no rule there names reads
// workspace-settings.yaml inside the source itself.
const RepositorySettingsDir = ".kode-stream"

// RepositorySettingsPath is the repository's settings file, relative to the
// workspace root.
const RepositorySettingsPath = RepositorySettingsDir + "/" + SourceStructureSettingsFile

func sourceSettingsFile(source string) string {
	return filepath.ToSlash(filepath.Join(filepath.Clean(source), SourceStructureSettingsFile))
}

// cardNamesSource says whether a rule of the repository's file belongs to
// source.
func cardNamesSource(card models.SourceStructureCard, source string) bool {
	name := strings.TrimSpace(card.Fields.Source)
	source = filepath.ToSlash(filepath.Clean(source))
	return name != "" && (name == source || name == filepath.Base(source))
}

func decodeSourceStructureSettings(data []byte) (models.SourceStructureSettings, error) {
	var settings models.SourceStructureSettings
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	err := decoder.Decode(&settings)
	return settings, err
}

// splitCards parts the rules of the repository's file into those of source
// and the others.
func splitCards(settings models.SourceStructureSettings, source string) (own, others []models.SourceStructureCard) {
	for _, card := range settings.Cards {
		if cardNamesSource(card, source) {
			own = append(own, card)
		} else {
			others = append(others, card)
		}
	}
	return own, others
}

// readRepositorySettings reads the repository's file from the workspace at
// workspaceRoot. A missing file is empty settings.
func readRepositorySettings(workspaceRoot string) (models.SourceStructureSettings, bool, error) {
	data, err := os.ReadFile(filepath.Join(workspaceRoot, filepath.FromSlash(RepositorySettingsPath)))
	if errors.Is(err, os.ErrNotExist) {
		return models.SourceStructureSettings{Version: 1}, false, nil
	}
	if err != nil {
		return models.SourceStructureSettings{}, false, err
	}
	settings, err := decodeSourceStructureSettings(data)
	if err != nil {
		return models.SourceStructureSettings{}, true, fmt.Errorf("%s: %w", RepositorySettingsPath, err)
	}
	return settings, true, nil
}

// SourceSettingsPath is the file that holds the settings of source in the
// workspace at workspaceRoot: the repository's file when a rule there names
// the source, else the file inside the source when it exists, else the
// repository's file, which saving creates or extends. The second result says
// whether the source has settings in that file.
func SourceSettingsPath(workspaceRoot, source string) (string, bool) {
	repository := filepath.Join(workspaceRoot, filepath.FromSlash(RepositorySettingsPath))
	if settings, _, err := readRepositorySettings(workspaceRoot); err == nil {
		if own, _ := splitCards(settings, source); len(own) > 0 {
			return repository, true
		}
	}
	inSource := filepath.Join(workspaceRoot, filepath.FromSlash(sourceSettingsFile(source)))
	if _, err := os.Lstat(inSource); err == nil {
		return inSource, true
	}
	return repository, false
}

var settingVariablePattern = regexp.MustCompile(`^\{([A-Za-z][A-Za-z0-9_]*)\}$`)

func DefaultSourceStructureSettings(sourceRoot ...string) models.SourceStructureSettings {
	source := "wiki"
	if len(sourceRoot) > 0 && strings.TrimSpace(filepath.Base(sourceRoot[0])) != "" {
		source = strings.TrimSpace(filepath.Base(sourceRoot[0]))
	}
	return models.SourceStructureSettings{
		Version: 1,
		Cards: []models.SourceStructureCard{{
			PathPattern: "{folder}/feature/{item}",
			Fields: models.SourceStructureFields{
				Source: source,
				Item:   "{item}",
				Title:  "readme_heading",
				Status: "draft",
				Tags:   []string{source},
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

// ReadSourceStructureSettings reads the settings of source, a path relative
// to the workspace at workspaceRoot.
func ReadSourceStructureSettings(workspaceRoot, source string) (models.SourceStructureSettings, bool, []models.ScanWarning) {
	return ReadSourceStructureSettingsFromReader(NewFilesystemSourceReader(workspaceRoot), source)
}

// ReadSourceStructureSettingsFromReader reads the settings of root, a source
// path relative to the reader, which is rooted at the workspace.
func ReadSourceStructureSettingsFromReader(reader SourceReader, root string) (models.SourceStructureSettings, bool, []models.ScanWarning) {
	if data, err := reader.ReadFile(RepositorySettingsPath); err == nil {
		settings, decodeErr := decodeSourceStructureSettings(data)
		if decodeErr != nil {
			return DefaultSourceStructureSettings(root), true, []models.ScanWarning{{ItemPath: RepositorySettingsPath, Message: "invalid workspace settings: " + decodeErr.Error()}}
		}
		if own, _ := splitCards(settings, root); len(own) > 0 {
			settings.Cards = own
			return settings, true, ValidateSourceStructureSettings(settings)
		}
	} else if !os.IsNotExist(err) {
		return DefaultSourceStructureSettings(root), false, []models.ScanWarning{{ItemPath: RepositorySettingsPath, Message: err.Error()}}
	}
	path := sourceSettingsFile(root)
	data, err := reader.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultSourceStructureSettings(root), false, nil
	}
	if err != nil {
		return DefaultSourceStructureSettings(root), false, []models.ScanWarning{{ItemPath: path, Message: err.Error()}}
	}
	settings, err := decodeSourceStructureSettings(data)
	if err != nil {
		return DefaultSourceStructureSettings(root), true, []models.ScanWarning{{ItemPath: path, Message: "invalid workspace settings: " + err.Error()}}
	}
	return settings, true, ValidateSourceStructureSettings(settings)
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

// WriteSourceStructureSettings saves the settings of source in the workspace
// at workspaceRoot to the file SourceSettingsPath names. In the repository's
// file it replaces the source's rules and keeps every other source's, and
// each saved rule must name the source in fields.source.
func WriteSourceStructureSettings(workspaceRoot, source string, settings models.SourceStructureSettings) error {
	if warnings := ValidateSourceStructureSettings(settings); len(warnings) > 0 {
		return errors.New(warnings[0].Message)
	}
	target, _ := SourceSettingsPath(workspaceRoot, source)
	if target != filepath.Join(workspaceRoot, filepath.FromSlash(RepositorySettingsPath)) {
		return marshalSettings(target, settings)
	}
	for i, card := range settings.Cards {
		if !cardNamesSource(card, source) {
			return fmt.Errorf("card rule %d fields.source must be %s, so %s knows the rule is this source's", i+1, filepath.Base(source), RepositorySettingsPath)
		}
	}
	repository, _, err := readRepositorySettings(workspaceRoot)
	if err != nil {
		return err
	}
	_, others := splitCards(repository, source)
	repository.Version = settings.Version
	repository.Cards = append(others, settings.Cards...)
	return marshalSettings(target, repository)
}

func marshalSettings(target string, settings models.SourceStructureSettings) error {
	data, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}
	return atomicWriteSettings(target, data)
}

// ensureSettingsDirectory creates the folders that hold target, refusing a
// symbolic link on the way so a write cannot leave the workspace.
func ensureSettingsDirectory(target string) error {
	directory := filepath.Dir(target)
	var missing []string
	for current := directory; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return errors.New("workspace settings folder must be a regular folder")
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		missing = append(missing, current)
		if filepath.Base(current) == RepositorySettingsDir || filepath.Dir(current) == current {
			break
		}
	}
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], 0o755); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	return nil
}

func atomicWriteSettings(target string, data []byte) error {
	if err := ensureSettingsDirectory(target); err != nil {
		return err
	}
	info, err := os.Lstat(target)
	mode := os.FileMode(0o644)
	if err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("workspace settings must be a regular file")
		}
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".workspace-settings-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(target))
	if err != nil {
		return err
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return err
	}
	return directory.Close()
}

// ReadSourceStructureSettingsFile reads the file at path as it is, to restore
// it if a save fails.
func ReadSourceStructureSettingsFile(path string) ([]byte, os.FileMode, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, 0, false, errors.New("workspace settings must be a regular file")
	}
	data, err := os.ReadFile(path)
	return data, info.Mode().Perm(), true, err
}

func RestoreSourceStructureSettings(path string, data []byte, existed bool) error {
	if !existed {
		return removeSettingsFile(path)
	}
	return atomicWriteSettings(path, data)
}

// RemoveSourceStructureSettings removes the settings of source: its rules
// from the repository's file, deleting the file when no other source's rule
// is left, or else the file inside the source.
func RemoveSourceStructureSettings(workspaceRoot, source string) error {
	target, exists := SourceSettingsPath(workspaceRoot, source)
	if !exists {
		return nil
	}
	if target == filepath.Join(workspaceRoot, filepath.FromSlash(RepositorySettingsPath)) {
		repository, _, err := readRepositorySettings(workspaceRoot)
		if err != nil {
			return err
		}
		if _, others := splitCards(repository, source); len(others) > 0 {
			repository.Cards = others
			return marshalSettings(target, repository)
		}
	}
	return removeSettingsFile(target)
}

func removeSettingsFile(path string) error {
	err := os.Remove(path)
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
		if strings.TrimSpace(card.Fields.Scope) != "" || strings.TrimSpace(card.Fields.Identifier) != "" {
			warnings = append(warnings, models.ScanWarning{ItemPath: SourceStructureSettingsFile, Message: prefix + " uses legacy fields.scope/fields.identifier; convert them explicitly to fields.source/fields.item"})
			continue
		}
		sourceField := card.Fields.Source
		itemField := card.Fields.Item
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
