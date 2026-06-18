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

func New(git *gitadapter.GitAdapter) *Scanner {
	return &Scanner{git: git}
}

func (s *Scanner) Scan(workspace models.WorkspaceConfig) (ScanData, error) {
	branch, err := s.git.CurrentBranch(workspace.Path)
	if err != nil {
		branch = workspace.BaselineBranch
	}
	var out ScanData
	for _, source := range workspace.Sources {
		root := filepath.Join(workspace.Path, filepath.FromSlash(source))
		items, warnings := s.scanItemDirectory(workspace, branch, source, root)
		out.Items = append(out.Items, items...)
		out.Warnings = append(out.Warnings, warnings...)
	}
	sort.Slice(out.Items, func(i, j int) bool {
		return out.Items[i].UpdatedAt.After(out.Items[j].UpdatedAt)
	})
	return out, nil
}

func (s *Scanner) scanItemDirectory(workspace models.WorkspaceConfig, branch, source, root string) ([]models.ItemDetail, []models.ScanWarning) {
	var items []models.ItemDetail
	var warnings []models.ScanWarning
	entries, err := os.ReadDir(root)
	if err != nil {
		return items, []models.ScanWarning{{ItemPath: source, Message: err.Error()}}
	}
	settings, hasSettings, settingsWarnings := ReadSourceStructureSettings(root)
	if hasSettings {
		warnings = append(warnings, settingsWarnings...)
		if len(settingsWarnings) == 0 {
			configuredItems, configuredWarnings := s.scanConfiguredItemDirectory(workspace, branch, source, root, settings)
			warnings = append(warnings, configuredWarnings...)
			if len(configuredItems) > 0 {
				return configuredItems, warnings
			}
			warnings = append(warnings, models.ScanWarning{ItemPath: source, Message: "workspace settings did not match any card directories; using fallback scan"})
		}
	}
	if shouldScanAsDocumentCollection(root, entries) {
		detail, itemWarnings, err := s.parseItem(workspace, branch, filepath.Base(source), filepath.Base(source), filepath.ToSlash(source), root)
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
		scopeRoot := filepath.Join(root, scopeEntry.Name())
		tickets, err := os.ReadDir(scopeRoot)
		if err != nil {
			warnings = append(warnings, models.ScanWarning{ItemPath: filepath.ToSlash(filepath.Join(source, scopeEntry.Name())), Message: err.Error()})
			continue
		}
		for _, identifierEntry := range tickets {
			if !identifierEntry.IsDir() || strings.HasPrefix(identifierEntry.Name(), ".") {
				continue
			}
			itemRoot := filepath.Join(scopeRoot, identifierEntry.Name())
			relItemPath := filepath.ToSlash(filepath.Join(source, scopeEntry.Name(), identifierEntry.Name()))
			itemBranch := branch
			if matchedBranch := s.git.BranchForIdentifier(workspace.Path, identifierEntry.Name()); matchedBranch != "" {
				itemBranch = matchedBranch
			}
			detail, itemWarnings, err := s.parseItem(workspace, itemBranch, scopeEntry.Name(), identifierEntry.Name(), relItemPath, itemRoot)
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

func (s *Scanner) scanConfiguredItemDirectory(workspace models.WorkspaceConfig, branch, source, root string, settings models.SourceStructureSettings) ([]models.ItemDetail, []models.ScanWarning) {
	var items []models.ItemDetail
	var warnings []models.ScanWarning
	seen := map[string]bool{}
	for _, card := range settings.Cards {
		segments, err := parsePathPattern(card.PathPattern)
		if err != nil {
			warnings = append(warnings, models.ScanWarning{ItemPath: source, Message: err.Error()})
			continue
		}
		for _, match := range matchPatternDirectories(root, segments) {
			if seen[match.path] {
				continue
			}
			seen[match.path] = true
			scope := renderSettingsTemplate(card.Fields.Scope, match.captures)
			identifier := renderSettingsTemplate(card.Fields.Identifier, match.captures)
			if strings.TrimSpace(scope) == "" || strings.TrimSpace(identifier) == "" {
				warnings = append(warnings, models.ScanWarning{ItemPath: filepath.ToSlash(match.path), Message: "workspace settings produced an empty scope or identifier"})
				continue
			}
			relFromRoot, err := filepath.Rel(root, match.path)
			if err != nil {
				warnings = append(warnings, models.ScanWarning{ItemPath: filepath.ToSlash(match.path), Message: err.Error()})
				continue
			}
			relItemPath := filepath.ToSlash(filepath.Join(source, relFromRoot))
			itemBranch := branch
			if matchedBranch := s.git.BranchForIdentifier(workspace.Path, identifier); matchedBranch != "" {
				itemBranch = matchedBranch
			}
			detail, itemWarnings, err := s.parseItem(workspace, itemBranch, scope, identifier, relItemPath, match.path)
			if err != nil {
				warnings = append(warnings, models.ScanWarning{ItemPath: relItemPath, Message: err.Error()})
				continue
			}
			warnings = append(warnings, itemWarnings...)
			if detail.MetadataSource != "item.yaml" {
				applySourceStructureSettings(&detail, card, match.captures)
			}
			items = append(items, detail)
		}
	}
	return items, warnings
}

type workspaceSettingsMatch struct {
	path     string
	captures map[string]string
}

func matchPatternDirectories(root string, segments []pathPatternSegment) []workspaceSettingsMatch {
	var matches []workspaceSettingsMatch
	var walk func(path string, depth int, captures map[string]string)
	walk = func(path string, depth int, captures map[string]string) {
		if depth == len(segments) {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				copied := map[string]string{}
				for key, value := range captures {
					copied[key] = value
				}
				matches = append(matches, workspaceSettingsMatch{path: path, captures: copied})
			}
			return
		}
		segment := segments[depth]
		if segment.literal != "" {
			next := filepath.Join(path, segment.literal)
			if info, err := os.Stat(next); err == nil && info.IsDir() {
				walk(next, depth+1, captures)
			}
			return
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			captures[segment.variable] = entry.Name()
			walk(filepath.Join(path, entry.Name()), depth+1, captures)
			delete(captures, segment.variable)
		}
	}
	walk(root, 0, map[string]string{})
	sort.Slice(matches, func(i, j int) bool {
		return naturalLess(filepath.ToSlash(matches[i].path), filepath.ToSlash(matches[j].path))
	})
	return matches
}

func applySourceStructureSettings(detail *models.ItemDetail, card models.SourceStructureCard, captures map[string]string) {
	fields := card.Fields
	detail.MetadataSource = "workspace-settings"
	detail.Scope = renderSettingsTemplate(fields.Scope, captures)
	detail.Identifier = renderSettingsTemplate(fields.Identifier, captures)
	if title := strings.TrimSpace(renderSettingsTemplate(fields.Title, captures)); title != "" && title != "readme_heading" {
		detail.Title = title
	}
	if status := strings.TrimSpace(renderSettingsTemplate(fields.Status, captures)); status != "" {
		detail.Status = NormalizeStatus(status)
	}
	if owner := strings.TrimSpace(renderSettingsTemplate(fields.Owner, captures)); owner != "" {
		detail.Owner = owner
		if detail.Author == "" {
			detail.Author = owner
		}
	}
	if fields.Tags != nil {
		tags := make([]string, 0, len(fields.Tags))
		seen := map[string]bool{}
		for _, tag := range fields.Tags {
			tag = strings.TrimSpace(renderSettingsTemplate(tag, captures))
			if tag != "" && !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
		detail.Tags = tags
	}
	if detail.Metadata == nil {
		detail.Metadata = map[string]any{}
	}
	detail.Metadata["workspaceSettings"] = map[string]any{
		"pathPattern": card.PathPattern,
		"captures":    captures,
	}
}

func renderSettingsTemplate(value string, captures map[string]string) string {
	out := value
	for key, replacement := range captures {
		out = strings.ReplaceAll(out, "{"+key+"}", replacement)
	}
	return strings.TrimSpace(out)
}

func shouldScanAsDocumentCollection(root string, entries []fs.DirEntry) bool {
	if hasMarkdownFiles(root) && !hasStructuredItemChildren(root, entries) {
		return true
	}
	return false
}

func hasStructuredItemChildren(root string, entries []fs.DirEntry) bool {
	for _, scopeEntry := range entries {
		if !scopeEntry.IsDir() || strings.HasPrefix(scopeEntry.Name(), ".") {
			continue
		}
		tickets, err := os.ReadDir(filepath.Join(root, scopeEntry.Name()))
		if err != nil {
			continue
		}
		for _, identifierEntry := range tickets {
			if identifierEntry.IsDir() && !strings.HasPrefix(identifierEntry.Name(), ".") && isItemFolder(filepath.Join(root, scopeEntry.Name(), identifierEntry.Name()), identifierEntry.Name()) {
				return true
			}
		}
	}
	return false
}

func isItemFolder(path, name string) bool {
	if _, err := os.Stat(filepath.Join(path, "item.yaml")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(path, "plan.yaml")); err == nil {
		return true
	}
	return regexp.MustCompile(`^[A-Z]+-\d+$`).MatchString(strings.ToUpper(name))
}

type planYAML struct {
	Item struct {
		Identifier string   `yaml:"identifier"`
		Title      string   `yaml:"title"`
		Scope      string   `yaml:"scope"`
		Status     string   `yaml:"status"`
		Owner      string   `yaml:"owner"`
		Tags       []string `yaml:"tags"`
	} `yaml:"item"`
	Documents []models.ItemDocument `yaml:"documents"`
}

func (s *Scanner) parseItem(workspace models.WorkspaceConfig, branch, scope, identifier, relItemPath, itemRoot string) (models.ItemDetail, []models.ScanWarning, error) {
	var warnings []models.ScanWarning
	metaSource := "fallback"
	title := titleFromIdentifier(identifier)
	status := models.StatusDraft
	owner := ""
	tags := []string{}
	documents := []models.ItemDocument{}
	metadata := map[string]any{}

	if data, source, err := readItemYAML(itemRoot); err == nil {
		parsed := parseItemYAML(string(data))
		metaSource = source
		if parsed.Item.Identifier != "" {
			identifier = parsed.Item.Identifier
		}
		if parsed.Item.Scope != "" {
			scope = parsed.Item.Scope
		}
		if parsed.Item.Title != "" {
			title = parsed.Item.Title
		}
		owner = parsed.Item.Owner
		status = NormalizeStatus(parsed.Item.Status)
		if parsed.Item.Tags != nil {
			tags = parsed.Item.Tags
		}
		documents = normalizeDocuments(parsed.Documents)
		metadata["item"] = parsed.Item
	} else if !errors.Is(err, os.ErrNotExist) {
		warnings = append(warnings, models.ScanWarning{ItemPath: relItemPath, Message: err.Error()})
	}

	readme := filepath.Join(itemRoot, "README.md")
	description := ""
	if data, err := os.ReadFile(readme); err == nil {
		if metaSource == "fallback" {
			if h := firstHeading(string(data)); h != "" {
				title = h
			}
			status = inferStatus(itemRoot)
		}
		description = firstParagraph(string(data))
	}
	if len(documents) == 0 {
		documents = fallbackDocuments(itemRoot)
	}
	fileCount := countMarkdownFiles(itemRoot)
	relForGit := filepath.ToSlash(relItemPath)
	updated := s.git.LastUpdate(workspace.Path, relForGit)
	if updated.IsZero() {
		updated = latestModTime(itemRoot)
	}

	summary := models.ItemSummary{
		ID:             stablePlanID(workspace.ID, branch, relItemPath),
		WorkspaceID:    workspace.ID,
		WorkspaceName:  workspace.Name,
		Branch:         branch,
		Scope:          scope,
		Identifier:     identifier,
		Title:          title,
		Status:         status,
		Owner:          owner,
		Author:         s.git.LastAuthor(workspace.Path, relForGit),
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

func readItemYAML(root string) ([]byte, string, error) {
	if data, err := os.ReadFile(filepath.Join(root, "item.yaml")); err == nil {
		return data, "item.yaml", nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, "", err
	}
	data, err := os.ReadFile(filepath.Join(root, "plan.yaml"))
	if err != nil {
		return nil, "", err
	}
	return data, "plan.yaml", nil
}

func NormalizeStatus(raw string) models.ItemStatus {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	switch s {
	case "unsorted", "unstructured":
		return models.StatusUnsorted
	case "ideas", "idea", "backlog":
		return models.StatusIdeas
	case "in_progress", "progress", "doing", "active":
		return models.StatusInProgress
	case "review", "in_review":
		return models.StatusReview
	case "done", "complete", "completed", "closed":
		return models.StatusDone
	default:
		return models.StatusDraft
	}
}

func normalizeDocuments(docs []models.ItemDocument) []models.ItemDocument {
	for i := range docs {
		if docs[i].ID == "" {
			docs[i].ID = fileID(docs[i].Path)
		}
		if docs[i].Label == "" {
			docs[i].Label = labelFromPath(docs[i].Path)
		}
	}
	sort.SliceStable(docs, func(i, j int) bool { return naturalLess(docs[i].Path, docs[j].Path) })
	return docs
}

func parseItemYAML(data string) planYAML {
	var parsed planYAML
	section := ""
	var current *models.ItemDocument
	for _, raw := range strings.Split(data, "\n") {
		if strings.TrimSpace(raw) == "" || strings.HasPrefix(strings.TrimSpace(raw), "#") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		line := strings.TrimSpace(raw)
		switch line {
		case "item:", "plan:":
			section = "item"
			continue
		case "documents:":
			section = "documents"
			continue
		}
		if section == "item" && indent >= 2 {
			key, value, ok := splitYAMLPair(line)
			if !ok {
				continue
			}
			switch key {
			case "identifier", "ticket":
				parsed.Item.Identifier = value
			case "title":
				parsed.Item.Title = value
			case "scope", "service":
				parsed.Item.Scope = value
			case "status":
				parsed.Item.Status = value
			case "owner":
				parsed.Item.Owner = value
			case "tags":
				parsed.Item.Tags = parseYAMLList(value)
			}
			continue
		}
		if section == "documents" && strings.HasPrefix(line, "- ") {
			parsed.Documents = append(parsed.Documents, models.ItemDocument{})
			current = &parsed.Documents[len(parsed.Documents)-1]
			line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
			if key, value, ok := splitYAMLPair(line); ok {
				assignDocumentField(current, key, value)
			}
			continue
		}
		if section == "documents" && current != nil && indent >= 4 {
			if key, value, ok := splitYAMLPair(line); ok {
				assignDocumentField(current, key, value)
			}
		}
	}
	return parsed
}

func splitYAMLPair(line string) (string, string, bool) {
	key, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	return strings.TrimSpace(key), trimYAMLScalar(value), true
}

func trimYAMLScalar(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	if value == "null" {
		return ""
	}
	return value
}

func parseYAMLList(value string) []string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		if value == "" {
			return nil
		}
		return []string{trimYAMLScalar(value)}
	}
	value = strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
	var out []string
	for _, item := range strings.Split(value, ",") {
		item = trimYAMLScalar(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func assignDocumentField(doc *models.ItemDocument, key, value string) {
	switch key {
	case "id":
		doc.ID = value
	case "role":
		doc.Role = value
	case "track":
		doc.Track = value
	case "path":
		doc.Path = value
	case "label":
		doc.Label = value
	}
}

func fallbackDocuments(itemRoot string) []models.ItemDocument {
	docs := []models.ItemDocument{}
	_ = filepath.WalkDir(itemRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(itemRoot, path)
		role := "other"
		relSlash := filepath.ToSlash(rel)
		if relSlash == "README.md" {
			role = "overview"
		} else if strings.HasPrefix(relSlash, "scenario/") {
			role = "scenario"
		} else if strings.HasPrefix(relSlash, "design/") {
			role = "design"
		} else if relSlash == "implementation-item.md" {
			role = "implementation"
		}
		docs = append(docs, models.ItemDocument{
			ID: fileID(relSlash), Role: role, Path: relSlash, Label: labelFromPath(relSlash),
		})
		return nil
	})
	sort.Slice(docs, func(i, j int) bool { return naturalLess(docs[i].Path, docs[j].Path) })
	return docs
}

func inferStatus(itemRoot string) models.ItemStatus {
	data, err := os.ReadFile(filepath.Join(itemRoot, "implementation-item.md"))
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

func countMarkdownFiles(root string) int {
	count := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			count++
		}
		return nil
	})
	return count
}

func hasMarkdownFiles(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func latestModTime(root string) time.Time {
	var latest time.Time
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
