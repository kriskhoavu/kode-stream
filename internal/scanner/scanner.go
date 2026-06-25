package scanner

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"plan-manager/internal/gitadapter"
	"plan-manager/internal/models"
)

type Scanner struct {
	git *gitadapter.GitAdapter
}

type ScanData struct {
	Items    []models.ItemDetail
	Warnings []models.ScanWarning
}

type ScanRequest struct {
	Workspace  models.WorkspaceConfig
	Branch     string
	BranchRef  string
	Commit     string
	SourceMode string
	Editable   bool
	Reader     SourceReader
}

func New(git *gitadapter.GitAdapter) *Scanner {
	return &Scanner{git: git}
}

func (s *Scanner) Scan(workspace models.WorkspaceConfig) (ScanData, error) {
	branch, err := s.git.CurrentBranch(workspace.Path)
	if err != nil {
		branch = workspace.BaselineBranch
	}
	return s.ScanWithRequest(ScanRequest{
		Workspace:  workspace,
		Branch:     branch,
		SourceMode: "working_tree",
		Editable:   true,
		Reader:     NewFilesystemSourceReader(workspace.Path),
	})
}

func (s *Scanner) ScanWithRequest(request ScanRequest) (ScanData, error) {
	workspace := request.Workspace
	branch := request.Branch
	if strings.TrimSpace(branch) == "" {
		branch = workspace.BaselineBranch
	}
	reader := request.Reader
	if reader == nil {
		reader = NewFilesystemSourceReader(workspace.Path)
	}
	sourceMode := strings.TrimSpace(request.SourceMode)
	if sourceMode == "" {
		sourceMode = "working_tree"
	}
	request.SourceMode = sourceMode
	request.Branch = branch
	var out ScanData
	for _, source := range workspace.Sources {
		items, warnings := s.scanItemDirectory(request, reader, branch, sourceMode, source)
		out.Items = append(out.Items, items...)
		out.Warnings = append(out.Warnings, warnings...)
	}
	sort.Slice(out.Items, func(i, j int) bool {
		return out.Items[i].UpdatedAt.After(out.Items[j].UpdatedAt)
	})
	return out, nil
}

func (s *Scanner) scanItemDirectory(request ScanRequest, reader SourceReader, branch, sourceMode, source string) ([]models.ItemDetail, []models.ScanWarning) {
	var items []models.ItemDetail
	var warnings []models.ScanWarning
	entries, err := reader.ReadDir(source)
	if err != nil {
		return items, []models.ScanWarning{{ItemPath: source, Message: err.Error()}}
	}
	settings, hasSettings, settingsWarnings := ReadSourceStructureSettingsFromReader(reader, source)
	if hasSettings {
		warnings = append(warnings, settingsWarnings...)
		if len(settingsWarnings) == 0 {
			configuredItems, configuredWarnings := s.scanConfiguredItemDirectory(request, reader, branch, source, settings)
			warnings = append(warnings, configuredWarnings...)
			if len(configuredItems) > 0 {
				return configuredItems, warnings
			}
			warnings = append(warnings, models.ScanWarning{ItemPath: source, Message: "workspace settings did not match any card directories; using fallback scan"})
		}
	}
	if shouldScanAsDocumentCollection(reader, source, entries) {
		detail, itemWarnings, err := s.parseItem(request, reader, branch, filepath.Base(source), filepath.Base(source), filepath.ToSlash(source), source)
		if err != nil {
			return items, []models.ScanWarning{{ItemPath: source, Message: err.Error()}}
		}
		detail.MetadataSource = "docs"
		detail.Status = models.StatusUnsorted
		if detail.Title == titleFromIdentifier(filepath.Base(source)) {
			detail.Title = titleFromDocumentRoot(source)
		}
		detail.Tags = append(detail.Tags, "docs")
		return []models.ItemDetail{detail}, append(warnings, itemWarnings...)
	}
	for _, scopeEntry := range entries {
		if !scopeEntry.IsDir() || strings.HasPrefix(scopeEntry.Name(), ".") {
			continue
		}
		scopeRoot := filepath.ToSlash(filepath.Join(source, scopeEntry.Name()))
		tickets, err := reader.ReadDir(scopeRoot)
		if err != nil {
			warnings = append(warnings, models.ScanWarning{ItemPath: filepath.ToSlash(filepath.Join(source, scopeEntry.Name())), Message: err.Error()})
			continue
		}
		for _, identifierEntry := range tickets {
			if !identifierEntry.IsDir() || strings.HasPrefix(identifierEntry.Name(), ".") {
				continue
			}
			itemRoot := filepath.ToSlash(filepath.Join(scopeRoot, identifierEntry.Name()))
			relItemPath := filepath.ToSlash(filepath.Join(source, scopeEntry.Name(), identifierEntry.Name()))
			detail, itemWarnings, err := s.parseItem(request, reader, branch, scopeEntry.Name(), identifierEntry.Name(), relItemPath, itemRoot)
			if err != nil {
				warnings = append(warnings, models.ScanWarning{ItemPath: relItemPath, Message: err.Error()})
				continue
			}
			warnings = append(warnings, itemWarnings...)
			items = append(items, detail)
		}
	}
	return items, warnings
}

func (s *Scanner) scanConfiguredItemDirectory(request ScanRequest, reader SourceReader, branch, source string, settings models.SourceStructureSettings) ([]models.ItemDetail, []models.ScanWarning) {
	var items []models.ItemDetail
	var warnings []models.ScanWarning
	seen := map[string]bool{}
	for _, card := range settings.Cards {
		segments, err := parsePathPattern(card.PathPattern)
		if err != nil {
			warnings = append(warnings, models.ScanWarning{ItemPath: source, Message: err.Error()})
			continue
		}
		for _, match := range matchPatternDirectories(reader, source, segments) {
			if seen[match.path] {
				continue
			}
			seen[match.path] = true
			scope := renderSettingsTemplate(firstNonEmpty(card.Fields.Source, card.Fields.Scope), match.captures)
			identifier := renderSettingsTemplate(firstNonEmpty(card.Fields.Item, card.Fields.Identifier), match.captures)
			if strings.TrimSpace(scope) == "" || strings.TrimSpace(identifier) == "" {
				warnings = append(warnings, models.ScanWarning{ItemPath: filepath.ToSlash(match.path), Message: "workspace settings produced an empty scope or identifier"})
				continue
			}
			relFromRoot, err := filepath.Rel(filepath.FromSlash(source), filepath.FromSlash(match.path))
			if err != nil {
				warnings = append(warnings, models.ScanWarning{ItemPath: filepath.ToSlash(match.path), Message: err.Error()})
				continue
			}
			relItemPath := filepath.ToSlash(filepath.Join(source, relFromRoot))
			detail, itemWarnings, err := s.parseItem(request, reader, branch, scope, identifier, relItemPath, match.path)
			if err != nil {
				warnings = append(warnings, models.ScanWarning{ItemPath: relItemPath, Message: err.Error()})
				continue
			}
			warnings = append(warnings, itemWarnings...)
			if detail.MetadataSource != "plan.yaml" {
				applySourceStructureSettings(&detail, card, match.captures)
			}
			items = append(items, detail)
		}
	}
	return items, warnings
}

func shouldScanAsDocumentCollection(reader SourceReader, root string, entries []DirEntry) bool {
	if hasMarkdownFiles(reader, root) && !hasStructuredItemChildren(reader, root, entries) {
		return true
	}
	return false
}

func hasStructuredItemChildren(reader SourceReader, root string, entries []DirEntry) bool {
	for _, scopeEntry := range entries {
		if !scopeEntry.IsDir() || strings.HasPrefix(scopeEntry.Name(), ".") {
			continue
		}
		tickets, err := reader.ReadDir(filepath.ToSlash(filepath.Join(root, scopeEntry.Name())))
		if err != nil {
			continue
		}
		for _, identifierEntry := range tickets {
			if identifierEntry.IsDir() && !strings.HasPrefix(identifierEntry.Name(), ".") && isItemFolder(reader, filepath.ToSlash(filepath.Join(root, scopeEntry.Name(), identifierEntry.Name())), identifierEntry.Name()) {
				return true
			}
		}
	}
	return false
}

func isItemFolder(reader SourceReader, path, name string) bool {
	if _, err := reader.Stat(filepath.ToSlash(filepath.Join(path, "plan.yaml"))); err == nil {
		return true
	}
	return regexp.MustCompile(`^[A-Z]+-\d+$`).MatchString(strings.ToUpper(name))
}

func (s *Scanner) parseItem(request ScanRequest, reader SourceReader, branch, scope, identifier, relItemPath, itemRoot string) (models.ItemDetail, []models.ScanWarning, error) {
	workspace := request.Workspace
	var warnings []models.ScanWarning
	metaSource := "fallback"
	title := titleFromIdentifier(identifier)
	status := models.StatusDraft
	owner := ""
	tags := []string{}
	documents := []models.ItemDocument{}
	metadata := map[string]any{}

	if data, source, err := readPlanYAML(reader, itemRoot); err == nil {
		parsed, parseErr := parsePlanYAML(string(data))
		if parseErr != nil {
			warnings = append(warnings, models.ScanWarning{ItemPath: relItemPath, Message: parseErr.Error()})
		} else {
			metaSource = source
			if parsed.Plan.Identifier != "" {
				identifier = parsed.Plan.Identifier
			}
			if parsed.Plan.Scope != "" {
				scope = parsed.Plan.Scope
			}
			if parsed.Plan.Title != "" {
				title = parsed.Plan.Title
			}
			owner = parsed.Plan.Owner
			status = NormalizeStatus(parsed.Plan.Status)
			if parsed.Plan.Tags != nil {
				tags = parsed.Plan.Tags
			}
			documents = resolveDocuments(reader, itemRoot, parsed.Documents)
			metadata["plan"] = parsed.Plan
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		warnings = append(warnings, models.ScanWarning{ItemPath: relItemPath, Message: err.Error()})
	}

	description := ""
	if data, err := reader.ReadFile(filepath.ToSlash(filepath.Join(itemRoot, "README.md"))); err == nil {
		if inferredTitle := titleFromHeading(firstHeading(string(data)), identifier); inferredTitle != "" && title == titleFromIdentifier(identifier) {
			title = inferredTitle
		}
		if metaSource == "fallback" {
			status = inferStatus(reader, itemRoot)
		}
		description = firstParagraph(string(data))
	}
	if len(documents) == 0 {
		documents = fallbackDocuments(reader, itemRoot)
	}
	fileCount := countMarkdownFiles(reader, itemRoot)
	relForGit := filepath.ToSlash(relItemPath)
	updated := time.Time{}
	author := ""
	if request.BranchRef != "" {
		updated = s.git.LastUpdateAtRef(workspace.Path, request.BranchRef, relForGit)
		author = s.git.LastAuthorAtRef(workspace.Path, request.BranchRef, relForGit)
	} else {
		updated = s.git.LastUpdate(workspace.Path, relForGit)
		author = s.git.LastAuthor(workspace.Path, relForGit)
	}
	if updated.IsZero() {
		updated = latestModTime(reader, itemRoot)
	}

	summary := models.ItemSummary{
		ID:             stablePlanID(workspace.ID, branch, relItemPath),
		WorkspaceID:    workspace.ID,
		WorkspaceName:  workspace.Name,
		Branch:         branch,
		BranchRef:      request.BranchRef,
		Commit:         request.Commit,
		SourceMode:     request.SourceMode,
		Editable:       request.Editable,
		Scope:          scope,
		Identifier:     identifier,
		Title:          title,
		Status:         status,
		Owner:          owner,
		Author:         author,
		Tags:           tags,
		UpdatedAt:      updated,
		Description:    description,
		MetadataSource: metaSource,
		ItemPath:       relItemPath,
	}
	if summary.Author == "" && owner != "" {
		summary.Author = owner
	}
	return models.ItemDetail{
		ItemSummary: summary,
		Documents:   documents,
		Metadata:    metadata,
		Warnings:    warnings,
		Counts:      models.ItemWorkspaceCounts{Files: fileCount},
	}, warnings, nil
}

func fallbackDocuments(reader SourceReader, itemRoot string) []models.ItemDocument {
	docs := []models.ItemDocument{}
	_ = reader.WalkDir(itemRoot, func(path string, d DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(filepath.FromSlash(itemRoot), filepath.FromSlash(path))
		relSlash := filepath.ToSlash(rel)
		docs = append(docs, inferDocumentAt(reader, itemRoot, relSlash))
		return nil
	})
	sort.Slice(docs, func(i, j int) bool { return documentLess(docs[i], docs[j]) })
	return docs
}

func InferDocuments(itemRoot string) []models.ItemDocument {
	return fallbackDocuments(NewFilesystemSourceReader(filepath.Dir(itemRoot)), filepath.Base(itemRoot))
}

func resolveDocuments(reader SourceReader, itemRoot string, overrides []models.ItemDocument) []models.ItemDocument {
	docs := fallbackDocuments(reader, itemRoot)
	byPath := make(map[string]int, len(docs))
	for i := range docs {
		byPath[filepath.ToSlash(docs[i].Path)] = i
	}
	for _, override := range overrides {
		override.Path = filepath.ToSlash(strings.TrimSpace(override.Path))
		if override.Path == "" {
			continue
		}
		index, found := byPath[override.Path]
		if !found {
			docs = append(docs, normalizeDocuments([]models.ItemDocument{override})[0])
			byPath[override.Path] = len(docs) - 1
			continue
		}
		if override.ID != "" {
			docs[index].ID = override.ID
		}
		if override.Role != "" {
			docs[index].Role = override.Role
		}
		if override.Track != "" {
			docs[index].Track = override.Track
		}
		if override.Label != "" {
			docs[index].Label = override.Label
		}
	}
	sort.SliceStable(docs, func(i, j int) bool { return documentLess(docs[i], docs[j]) })
	return docs
}

func inferStatus(reader SourceReader, itemRoot string) models.ItemStatus {
	data, err := reader.ReadFile(filepath.ToSlash(filepath.Join(itemRoot, "implementation-plan.md")))
	if os.IsNotExist(err) {
		data, err = reader.ReadFile(filepath.ToSlash(filepath.Join(itemRoot, "implementation-item.md")))
	}
	if err != nil {
		return models.StatusDraft
	}
	text := strings.ToLower(string(data))
	switch {
	case strings.Contains(text, "✅") || strings.Contains(text, "[x]"):
		return models.StatusInProgress
	default:
		return models.StatusDraft
	}
}

func firstHeading(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

func InferPlanTitle(itemRoot, identifier string) string {
	reader := NewFilesystemSourceReader(filepath.Dir(itemRoot))
	data, err := reader.ReadFile(filepath.ToSlash(filepath.Join(filepath.Base(itemRoot), "README.md")))
	if err != nil {
		return ""
	}
	return titleFromHeading(firstHeading(string(data)), identifier)
}

func titleFromHeading(heading, identifier string) string {
	heading = strings.TrimSpace(heading)
	identifier = strings.TrimSpace(identifier)
	if heading == "" {
		return ""
	}
	if identifier != "" && strings.HasPrefix(strings.ToLower(heading), strings.ToLower(identifier)) {
		trimmed := strings.TrimSpace(heading[len(identifier):])
		trimmed = strings.TrimLeft(trimmed, ":-–— ")
		if trimmed != "" {
			return trimmed
		}
	}
	return heading
}

func firstParagraph(markdown string) string {
	for _, block := range strings.Split(markdown, "\n\n") {
		clean := strings.TrimSpace(block)
		if clean == "" || strings.HasPrefix(clean, "#") || strings.HasPrefix(clean, "|") {
			continue
		}
		clean = regexp.MustCompile(`\s+`).ReplaceAllString(clean, " ")
		return clean
	}
	return ""
}

func titleFromIdentifier(identifier string) string {
	return strings.ReplaceAll(identifier, "-", " ")
}

func titleFromDocumentRoot(source string) string {
	base := filepath.Base(filepath.Clean(filepath.FromSlash(source)))
	if base == "." || base == string(filepath.Separator) {
		return "Documentation"
	}
	return strings.Title(strings.ReplaceAll(strings.ReplaceAll(base, "-", " "), "_", " "))
}

func naturalLess(left, right string) bool {
	leftParts := naturalParts(left)
	rightParts := naturalParts(right)
	for i := 0; i < len(leftParts) && i < len(rightParts); i++ {
		a, b := leftParts[i], rightParts[i]
		if a.number && b.number {
			if a.numberValue != b.numberValue {
				return a.numberValue < b.numberValue
			}
			continue
		}
		if a.value != b.value {
			return a.value < b.value
		}
	}
	return len(leftParts) < len(rightParts)
}

type naturalPart struct {
	value       string
	number      bool
	numberValue int
}

func naturalParts(input string) []naturalPart {
	var parts []naturalPart
	for i := 0; i < len(input); {
		start := i
		isNumber := unicode.IsDigit(rune(input[i]))
		for i < len(input) && unicode.IsDigit(rune(input[i])) == isNumber {
			i++
		}
		value := strings.ToLower(input[start:i])
		part := naturalPart{value: value, number: isNumber}
		if isNumber {
			part.numberValue, _ = strconv.Atoi(value)
		}
		parts = append(parts, part)
	}
	return parts
}

func stablePlanID(repoID, branch, relItemPath string) string {
	key := repoID + "|" + branch + "|" + relItemPath
	var h uint32 = 2166136261
	for _, b := range []byte(key) {
		h ^= uint32(b)
		h *= 16777619
	}
	return fmt.Sprintf("%s-%08x", repoID, h)
}

func fileID(path string) string {
	return strings.NewReplacer("/", "__", ".", "_").Replace(path)
}

func labelFromPath(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.ReplaceAll(base, "_", " ")
	return strings.Title(base)
}

func inferDocument(path string) models.ItemDocument {
	path = filepath.ToSlash(path)
	lower := strings.ToLower(path)
	doc := models.ItemDocument{ID: fileID(path), Role: "other", Path: path, Label: labelFromPath(path)}
	switch {
	case lower == "readme.md":
		doc.Role = "overview"
		doc.Label = "Overview"
	case strings.HasPrefix(lower, "scenario/"):
		doc.Role = "scenario"
		name := stripDocumentSequence(path, "scenario")
		if strings.EqualFold(name, "overview") {
			doc.Label = "Scenario Overview"
		} else {
			doc.Label = strings.Title(titleFromIdentifier(name))
		}
	case strings.HasPrefix(lower, "design/"):
		doc.Role = "design"
		name := stripDocumentSequence(path, "design")
		doc.Track = inferDocumentTrack(name)
		if doc.Track != "" && strings.EqualFold(name, doc.Track) {
			doc.Label = strings.Title(doc.Track) + " Design"
		} else {
			doc.Label = strings.Title(titleFromIdentifier(name))
		}
	case lower == "implementation-plan.md", lower == "implementation-item.md":
		doc.Role = "implementation"
		doc.Label = "Implementation Plan"
	}
	return doc
}

func inferDocumentAt(reader SourceReader, itemRoot, relPath string) models.ItemDocument {
	doc := inferDocument(relPath)
	if doc.Role != "design" || doc.Track != "" {
		return doc
	}
	data, err := reader.ReadFile(filepath.ToSlash(filepath.Join(itemRoot, filepath.FromSlash(relPath))))
	if err != nil {
		return doc
	}
	heading := strings.ToLower(firstHeading(string(data)))
	for _, track := range []string{"backend", "frontend", "infrastructure", "pipeline"} {
		if strings.Contains(heading, track) {
			doc.Track = track
			break
		}
	}
	return doc
}

func stripDocumentSequence(path, prefix string) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	pattern := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(prefix) + `-\d+-`)
	return pattern.ReplaceAllString(name, "")
}

func inferDocumentTrack(name string) string {
	lower := strings.ToLower(name)
	for _, track := range []string{"backend", "frontend", "infrastructure", "pipeline"} {
		if lower == track || strings.HasPrefix(lower, track+"-") || strings.HasSuffix(lower, "-"+track) {
			return track
		}
	}
	return ""
}

func documentLess(left, right models.ItemDocument) bool {
	ranks := map[string]int{"overview": 0, "scenario": 1, "design": 2, "implementation": 3, "other": 4}
	leftRank, leftOK := ranks[left.Role]
	if !leftOK {
		leftRank = 4
	}
	rightRank, rightOK := ranks[right.Role]
	if !rightOK {
		rightRank = 4
	}
	if leftRank != rightRank {
		return leftRank < rightRank
	}
	return naturalLess(left.Path, right.Path)
}

func countMarkdownFiles(reader SourceReader, root string) int {
	count := 0
	_ = reader.WalkDir(root, func(path string, d DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			count++
		}
		return nil
	})
	return count
}

func hasMarkdownFiles(reader SourceReader, root string) bool {
	found := false
	_ = reader.WalkDir(root, func(path string, d DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func latestModTime(reader SourceReader, root string) time.Time {
	var latest time.Time
	_ = reader.WalkDir(root, func(path string, d DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	return latest.UTC()
}
