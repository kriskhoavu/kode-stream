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
	Plans    []models.PlanDetail
	Warnings []models.ScanWarning
}

func New(git *gitadapter.GitAdapter) *Scanner {
	return &Scanner{git: git}
}

func (s *Scanner) Scan(repo models.RepositoryConfig) (ScanData, error) {
	branch, err := s.git.CurrentBranch(repo.Path)
	if err != nil {
		branch = repo.BaselineBranch
	}
	var out ScanData
	for _, planDir := range repo.PlanDirectories {
		root := filepath.Join(repo.Path, filepath.FromSlash(planDir))
		plans, warnings := s.scanPlanDirectory(repo, branch, planDir, root)
		out.Plans = append(out.Plans, plans...)
		out.Warnings = append(out.Warnings, warnings...)
	}
	sort.Slice(out.Plans, func(i, j int) bool {
		return out.Plans[i].UpdatedAt.After(out.Plans[j].UpdatedAt)
	})
	return out, nil
}

func (s *Scanner) scanPlanDirectory(repo models.RepositoryConfig, branch, planDir, root string) ([]models.PlanDetail, []models.ScanWarning) {
	var plans []models.PlanDetail
	var warnings []models.ScanWarning
	entries, err := os.ReadDir(root)
	if err != nil {
		return plans, []models.ScanWarning{{PlanPath: planDir, Message: err.Error()}}
	}
	settings, hasSettings, settingsWarnings := ReadRepositorySettings(root)
	if hasSettings {
		warnings = append(warnings, settingsWarnings...)
		if len(settingsWarnings) == 0 {
			configuredPlans, configuredWarnings := s.scanConfiguredPlanDirectory(repo, branch, planDir, root, settings)
			warnings = append(warnings, configuredWarnings...)
			if len(configuredPlans) > 0 {
				return configuredPlans, warnings
			}
			warnings = append(warnings, models.ScanWarning{PlanPath: planDir, Message: "repository settings did not match any card directories; using fallback scan"})
		}
	}
	if shouldScanAsDocumentCollection(root, entries) {
		detail, planWarnings, err := s.parsePlan(repo, branch, filepath.Base(planDir), filepath.Base(planDir), filepath.ToSlash(planDir), root)
		if err != nil {
			return plans, []models.ScanWarning{{PlanPath: planDir, Message: err.Error()}}
		}
		detail.MetadataSource = "docs"
		if detail.Title == titleFromTicket(filepath.Base(planDir)) {
			detail.Title = titleFromDocumentRoot(planDir)
		}
		detail.Tags = append(detail.Tags, "docs")
		return []models.PlanDetail{detail}, append(warnings, planWarnings...)
	}
	for _, serviceEntry := range entries {
		if !serviceEntry.IsDir() || strings.HasPrefix(serviceEntry.Name(), ".") {
			continue
		}
		serviceRoot := filepath.Join(root, serviceEntry.Name())
		tickets, err := os.ReadDir(serviceRoot)
		if err != nil {
			warnings = append(warnings, models.ScanWarning{PlanPath: filepath.ToSlash(filepath.Join(planDir, serviceEntry.Name())), Message: err.Error()})
			continue
		}
		for _, ticketEntry := range tickets {
			if !ticketEntry.IsDir() || strings.HasPrefix(ticketEntry.Name(), ".") {
				continue
			}
			planRoot := filepath.Join(serviceRoot, ticketEntry.Name())
			relPlanRoot := filepath.ToSlash(filepath.Join(planDir, serviceEntry.Name(), ticketEntry.Name()))
			planBranch := branch
			if matchedBranch := s.git.BranchForTicket(repo.Path, ticketEntry.Name()); matchedBranch != "" {
				planBranch = matchedBranch
			}
			detail, planWarnings, err := s.parsePlan(repo, planBranch, serviceEntry.Name(), ticketEntry.Name(), relPlanRoot, planRoot)
			if err != nil {
				warnings = append(warnings, models.ScanWarning{PlanPath: relPlanRoot, Message: err.Error()})
				continue
			}
			warnings = append(warnings, planWarnings...)
			plans = append(plans, detail)
		}
	}
	return plans, warnings
}

func (s *Scanner) scanConfiguredPlanDirectory(repo models.RepositoryConfig, branch, planDir, root string, settings models.RepositorySettings) ([]models.PlanDetail, []models.ScanWarning) {
	var plans []models.PlanDetail
	var warnings []models.ScanWarning
	seen := map[string]bool{}
	for _, card := range settings.Cards {
		segments, err := parsePathPattern(card.PathPattern)
		if err != nil {
			warnings = append(warnings, models.ScanWarning{PlanPath: planDir, Message: err.Error()})
			continue
		}
		for _, match := range matchPatternDirectories(root, segments) {
			if seen[match.path] {
				continue
			}
			seen[match.path] = true
			service := renderSettingsTemplate(card.Fields.Service, match.captures)
			ticket := renderSettingsTemplate(card.Fields.Ticket, match.captures)
			if strings.TrimSpace(service) == "" || strings.TrimSpace(ticket) == "" {
				warnings = append(warnings, models.ScanWarning{PlanPath: filepath.ToSlash(match.path), Message: "repository settings produced an empty service or ticket"})
				continue
			}
			relFromRoot, err := filepath.Rel(root, match.path)
			if err != nil {
				warnings = append(warnings, models.ScanWarning{PlanPath: filepath.ToSlash(match.path), Message: err.Error()})
				continue
			}
			relPlanRoot := filepath.ToSlash(filepath.Join(planDir, relFromRoot))
			planBranch := branch
			if matchedBranch := s.git.BranchForTicket(repo.Path, ticket); matchedBranch != "" {
				planBranch = matchedBranch
			}
			detail, planWarnings, err := s.parsePlan(repo, planBranch, service, ticket, relPlanRoot, match.path)
			if err != nil {
				warnings = append(warnings, models.ScanWarning{PlanPath: relPlanRoot, Message: err.Error()})
				continue
			}
			warnings = append(warnings, planWarnings...)
			if detail.MetadataSource != "plan.yaml" {
				applyRepositorySettings(&detail, card, match.captures)
			}
			plans = append(plans, detail)
		}
	}
	return plans, warnings
}

type repositorySettingsMatch struct {
	path     string
	captures map[string]string
}

func matchPatternDirectories(root string, segments []pathPatternSegment) []repositorySettingsMatch {
	var matches []repositorySettingsMatch
	var walk func(path string, depth int, captures map[string]string)
	walk = func(path string, depth int, captures map[string]string) {
		if depth == len(segments) {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				copied := map[string]string{}
				for key, value := range captures {
					copied[key] = value
				}
				matches = append(matches, repositorySettingsMatch{path: path, captures: copied})
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

func applyRepositorySettings(detail *models.PlanDetail, card models.RepositorySettingsCard, captures map[string]string) {
	fields := card.Fields
	detail.MetadataSource = "repository-settings"
	detail.Service = renderSettingsTemplate(fields.Service, captures)
	detail.Ticket = renderSettingsTemplate(fields.Ticket, captures)
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
	detail.Metadata["repositorySettings"] = map[string]any{
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
	if hasMarkdownFiles(root) && !hasStructuredPlanChildren(root, entries) {
		return true
	}
	return false
}

func hasStructuredPlanChildren(root string, entries []fs.DirEntry) bool {
	for _, serviceEntry := range entries {
		if !serviceEntry.IsDir() || strings.HasPrefix(serviceEntry.Name(), ".") {
			continue
		}
		tickets, err := os.ReadDir(filepath.Join(root, serviceEntry.Name()))
		if err != nil {
			continue
		}
		for _, ticketEntry := range tickets {
			if ticketEntry.IsDir() && !strings.HasPrefix(ticketEntry.Name(), ".") && isPlanFolder(filepath.Join(root, serviceEntry.Name(), ticketEntry.Name()), ticketEntry.Name()) {
				return true
			}
		}
	}
	return false
}

func isPlanFolder(path, name string) bool {
	if _, err := os.Stat(filepath.Join(path, "plan.yaml")); err == nil {
		return true
	}
	return regexp.MustCompile(`^[A-Z]+-\d+$`).MatchString(strings.ToUpper(name))
}

type planYAML struct {
	Plan struct {
		Ticket  string   `yaml:"ticket"`
		Title   string   `yaml:"title"`
		Service string   `yaml:"service"`
		Status  string   `yaml:"status"`
		Owner   string   `yaml:"owner"`
		Tags    []string `yaml:"tags"`
	} `yaml:"plan"`
	Documents []models.PlanDocument `yaml:"documents"`
}

func (s *Scanner) parsePlan(repo models.RepositoryConfig, branch, service, ticket, relPlanRoot, planRoot string) (models.PlanDetail, []models.ScanWarning, error) {
	var warnings []models.ScanWarning
	metaSource := "fallback"
	title := titleFromTicket(ticket)
	status := models.StatusDraft
	owner := ""
	tags := []string{}
	documents := []models.PlanDocument{}
	metadata := map[string]any{}

	if data, err := os.ReadFile(filepath.Join(planRoot, "plan.yaml")); err == nil {
		parsed := parsePlanYAML(string(data))
		metaSource = "plan.yaml"
		if parsed.Plan.Ticket != "" {
			ticket = parsed.Plan.Ticket
		}
		if parsed.Plan.Service != "" {
			service = parsed.Plan.Service
		}
		if parsed.Plan.Title != "" {
			title = parsed.Plan.Title
		}
		owner = parsed.Plan.Owner
		status = NormalizeStatus(parsed.Plan.Status)
		if parsed.Plan.Tags != nil {
			tags = parsed.Plan.Tags
		}
		documents = normalizeDocuments(parsed.Documents)
		metadata["plan"] = parsed.Plan
	} else if !errors.Is(err, os.ErrNotExist) {
		warnings = append(warnings, models.ScanWarning{PlanPath: relPlanRoot, Message: err.Error()})
	}

	readme := filepath.Join(planRoot, "README.md")
	description := ""
	if data, err := os.ReadFile(readme); err == nil {
		if metaSource == "fallback" {
			if h := firstHeading(string(data)); h != "" {
				title = h
			}
			status = inferStatus(planRoot)
		}
		description = firstParagraph(string(data))
	}
	if len(documents) == 0 {
		documents = fallbackDocuments(planRoot)
	}
	fileCount := countMarkdownFiles(planRoot)
	relForGit := filepath.ToSlash(relPlanRoot)
	updated := s.git.LastUpdate(repo.Path, relForGit)
	if updated.IsZero() {
		updated = latestModTime(planRoot)
	}

	summary := models.PlanSummary{
		ID:             stablePlanID(repo.ID, branch, relPlanRoot),
		RepositoryID:   repo.ID,
		RepositoryName: repo.Name,
		Branch:         branch,
		Service:        service,
		Ticket:         ticket,
		Title:          title,
		Status:         status,
		Owner:          owner,
		Author:         s.git.LastAuthor(repo.Path, relForGit),
		Tags:           tags,
		UpdatedAt:      updated,
		Description:    description,
		MetadataSource: metaSource,
		PlanRoot:       relPlanRoot,
	}
	if summary.Author == "" && owner != "" {
		summary.Author = owner
	}
	return models.PlanDetail{
		PlanSummary: summary,
		Documents:   documents,
		Metadata:    metadata,
		Warnings:    warnings,
		Counts:      models.PlanWorkspaceCounts{Files: fileCount},
	}, warnings, nil
}

func NormalizeStatus(raw string) models.PlanStatus {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	switch s {
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

func normalizeDocuments(docs []models.PlanDocument) []models.PlanDocument {
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

func parsePlanYAML(data string) planYAML {
	var parsed planYAML
	section := ""
	var current *models.PlanDocument
	for _, raw := range strings.Split(data, "\n") {
		if strings.TrimSpace(raw) == "" || strings.HasPrefix(strings.TrimSpace(raw), "#") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		line := strings.TrimSpace(raw)
		switch line {
		case "plan:":
			section = "plan"
			continue
		case "documents:":
			section = "documents"
			continue
		}
		if section == "plan" && indent >= 2 {
			key, value, ok := splitYAMLPair(line)
			if !ok {
				continue
			}
			switch key {
			case "ticket":
				parsed.Plan.Ticket = value
			case "title":
				parsed.Plan.Title = value
			case "service":
				parsed.Plan.Service = value
			case "status":
				parsed.Plan.Status = value
			case "owner":
				parsed.Plan.Owner = value
			case "tags":
				parsed.Plan.Tags = parseYAMLList(value)
			}
			continue
		}
		if section == "documents" && strings.HasPrefix(line, "- ") {
			parsed.Documents = append(parsed.Documents, models.PlanDocument{})
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

func assignDocumentField(doc *models.PlanDocument, key, value string) {
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

func fallbackDocuments(planRoot string) []models.PlanDocument {
	docs := []models.PlanDocument{}
	_ = filepath.WalkDir(planRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(planRoot, path)
		role := "other"
		relSlash := filepath.ToSlash(rel)
		if relSlash == "README.md" {
			role = "overview"
		} else if strings.HasPrefix(relSlash, "scenario/") {
			role = "scenario"
		} else if strings.HasPrefix(relSlash, "design/") {
			role = "design"
		} else if relSlash == "implementation-plan.md" {
			role = "implementation"
		}
		docs = append(docs, models.PlanDocument{
			ID: fileID(relSlash), Role: role, Path: relSlash, Label: labelFromPath(relSlash),
		})
		return nil
	})
	sort.Slice(docs, func(i, j int) bool { return naturalLess(docs[i].Path, docs[j].Path) })
	return docs
}

func inferStatus(planRoot string) models.PlanStatus {
	data, err := os.ReadFile(filepath.Join(planRoot, "implementation-plan.md"))
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

func titleFromTicket(ticket string) string {
	return strings.ReplaceAll(ticket, "-", " ")
}

func titleFromDocumentRoot(planDir string) string {
	base := filepath.Base(filepath.Clean(filepath.FromSlash(planDir)))
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

func stablePlanID(repoID, branch, relPlanRoot string) string {
	key := repoID + "|" + branch + "|" + relPlanRoot
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
