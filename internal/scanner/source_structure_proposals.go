package scanner

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"plan-manager/internal/models"
)

const maxSourceStructurePreviewRows = 6
const maxSourceStructureProposalScanDepth = 5

func SourceStructureProposals(reader SourceReader, root string, settings models.SourceStructureSettings) ([]models.SourceStructureProposal, []models.SourceStructurePreview) {
	currentPreview := PreviewSourceStructureCard(reader, root, firstSourceStructureCard(settings))
	rootMarkdownCount := sourceStructureRootMarkdownCount(reader, root)
	proposals := sourceStructureProposalCards(reader, root)
	out := make([]models.SourceStructureProposal, 0, len(proposals))
	for _, proposal := range proposals {
		preview := PreviewSourceStructureCard(reader, root, proposal.Card)
		proposal.Preview = preview
		if len(preview) == 0 {
			proposal.Summary = "No matching card directories found yet."
		} else {
			proposal.Summary = sourceStructureProposalSummary(preview, rootMarkdownCount)
		}
		out = append(out, proposal)
	}
	return out, currentPreview
}

func PreviewSourceStructureCard(reader SourceReader, root string, card models.SourceStructureCard) []models.SourceStructurePreview {
	if reader == nil || strings.TrimSpace(root) == "" || strings.TrimSpace(card.PathPattern) == "" {
		return []models.SourceStructurePreview{}
	}
	segments, err := parsePathPattern(card.PathPattern)
	if err != nil {
		return []models.SourceStructurePreview{}
	}
	matches := matchPatternDirectories(reader, root, segments)
	if len(matches) > maxSourceStructurePreviewRows {
		matches = matches[:maxSourceStructurePreviewRows]
	}
	rows := make([]models.SourceStructurePreview, 0, len(matches))
	for _, match := range matches {
		source := strings.TrimSpace(renderSettingsTemplate(firstNonEmpty(card.Fields.Source, card.Fields.Scope), match.captures))
		item := strings.TrimSpace(renderSettingsTemplate(firstNonEmpty(card.Fields.Item, card.Fields.Identifier), match.captures))
		if source == "" || item == "" {
			continue
		}
		rows = append(rows, models.SourceStructurePreview{
			Path:       filepath.ToSlash(match.path),
			Source:     source,
			Item:       item,
			Scope:      source,
			Identifier: item,
			Title:      previewTitle(reader, match.path, item, card.Fields.Title, match.captures),
			Status:     NormalizeStatus(renderSettingsTemplate(card.Fields.Status, match.captures)),
			Tags:       previewTags(card.Fields.Tags, match.captures),
		})
	}
	return rows
}

func sourceStructureProposalCards(reader SourceReader, root string) []models.SourceStructureProposal {
	sourceName := filepath.Base(filepath.ToSlash(root))
	if sourceName == "." || sourceName == "/" || sourceName == "" {
		sourceName = "source"
	}
	samples := sourceStructureSampleDirectories(reader, root)
	if len(samples) == 0 {
		return []models.SourceStructureProposal{}
	}

	candidates := sourceStructureCandidatesFromSamples(root, samples)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].count != candidates[j].count {
			return candidates[i].count > candidates[j].count
		}
		if candidates[i].depth != candidates[j].depth {
			return candidates[i].depth < candidates[j].depth
		}
		return naturalLess(candidates[i].pattern, candidates[j].pattern)
	})

	proposals := make([]models.SourceStructureProposal, 0, len(candidates))
	for _, candidate := range candidates {
		fields := models.SourceStructureFields{
			Source: sourceName,
			Item:   "{item}",
			Title:  "readme_heading",
			Status: "draft",
			Tags:   []string{sourceName},
		}
		proposals = append(proposals, models.SourceStructureProposal{
			ID:         "actual-" + proposalIDFromPattern(candidate.pattern),
			Label:      proposalLabelFromPattern(candidate.pattern),
			Confidence: proposalConfidence(candidate.count, len(samples)),
			Card: models.SourceStructureCard{
				PathPattern: candidate.pattern,
				Fields:      fields,
			},
		})
	}
	return proposals
}

type sourceStructureCandidate struct {
	pattern string
	count   int
	depth   int
}

func sourceStructureSampleDirectories(reader SourceReader, root string) []string {
	if reader == nil || strings.TrimSpace(root) == "" {
		return []string{}
	}
	seen := map[string]bool{}
	_ = reader.WalkDir(root, func(path string, d DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		cleanPath := filepath.ToSlash(path)
		name := d.Name()
		if sourceStructureHasHiddenSegment(root, cleanPath) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if sourceStructureRelativeDepth(root, cleanPath) > maxSourceStructureProposalScanDepth {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			return nil
		}
		parent := filepath.ToSlash(filepath.Dir(cleanPath))
		if strings.EqualFold(name, "README.md") {
			addSourceStructureSample(seen, root, parent)
			return nil
		}
		if sourceStructureRelativeDepth(root, parent) == 1 {
			addSourceStructureSample(seen, root, parent)
		}
		return nil
	})
	samples := make([]string, 0, len(seen))
	for sample := range seen {
		samples = append(samples, sample)
	}
	sort.Slice(samples, func(i, j int) bool {
		return naturalLess(samples[i], samples[j])
	})
	return samples
}

func sourceStructureRootMarkdownCount(reader SourceReader, root string) int {
	if reader == nil || strings.TrimSpace(root) == "" {
		return 0
	}
	entries, err := reader.ReadDir(root)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			count++
		}
	}
	return count
}

func addSourceStructureSample(seen map[string]bool, root, path string) {
	path = filepath.ToSlash(filepath.Clean(path))
	root = filepath.ToSlash(filepath.Clean(root))
	if path == "." || path == root || path == "" || strings.HasPrefix(path, "../") {
		return
	}
	if sourceStructureRelativeDepth(root, path) > maxSourceStructureProposalScanDepth {
		return
	}
	seen[path] = true
}

func sourceStructureCandidatesFromSamples(root string, samples []string) []sourceStructureCandidate {
	byDepth := map[int][][]string{}
	for _, sample := range samples {
		segments := sourceStructureRelativeSegments(root, sample)
		if len(segments) == 0 {
			continue
		}
		byDepth[len(segments)] = append(byDepth[len(segments)], segments)
	}
	candidates := make([]sourceStructureCandidate, 0, len(byDepth))
	for depth, grouped := range byDepth {
		patternSegments := make([]string, depth)
		for index := 0; index < depth; index++ {
			switch {
			case depth == 1:
				patternSegments[index] = "{item}"
			case index == 0:
				patternSegments[index] = "{folder}"
			case index == depth-1:
				patternSegments[index] = "{item}"
			default:
				patternSegments[index] = sourceStructureMiddleSegmentPattern(grouped, index)
			}
		}
		candidates = append(candidates, sourceStructureCandidate{
			pattern: strings.Join(patternSegments, "/"),
			count:   len(grouped),
			depth:   depth,
		})
	}
	return candidates
}

func sourceStructureMiddleSegmentPattern(grouped [][]string, index int) string {
	values := map[string]bool{}
	for _, segments := range grouped {
		if index < len(segments) {
			values[segments[index]] = true
		}
	}
	if len(values) == 1 {
		for value := range values {
			return value
		}
	}
	return fmt.Sprintf("{segment%d}", index+1)
}

func sourceStructureRelativeSegments(root, path string) []string {
	root = filepath.ToSlash(filepath.Clean(root))
	path = filepath.ToSlash(filepath.Clean(path))
	rel, err := filepath.Rel(filepath.FromSlash(root), filepath.FromSlash(path))
	if err != nil {
		return nil
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == "" || strings.HasPrefix(rel, "../") {
		return nil
	}
	return strings.Split(rel, "/")
}

func sourceStructureRelativeDepth(root, path string) int {
	return len(sourceStructureRelativeSegments(root, path))
}

func sourceStructureHasHiddenSegment(root, path string) bool {
	for _, segment := range sourceStructureRelativeSegments(root, path) {
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}

func proposalIDFromPattern(pattern string) string {
	id := strings.NewReplacer("{", "", "}", "", "/", "-", "_", "-").Replace(pattern)
	id = strings.Trim(id, "-")
	if id == "" {
		return "source"
	}
	return id
}

func proposalLabelFromPattern(pattern string) string {
	switch pattern {
	case "{item}":
		return "Top-level folders"
	case "{folder}/{item}":
		return "Nested item folders"
	case "{folder}/feature/{item}":
		return "Feature item folders"
	}
	parts := strings.Split(pattern, "/")
	labels := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, "{}")
		labels = append(labels, strings.Title(strings.ReplaceAll(part, "-", " ")))
	}
	return strings.Join(labels, " / ")
}

func proposalConfidence(count, total int) string {
	if count >= 2 || count == total {
		return "high"
	}
	return "medium"
}

func sourceStructureProposalSummary(preview []models.SourceStructurePreview, rootMarkdownCount int) string {
	summary := fmt.Sprintf("Shows %d matching card%s, for example %s.", len(preview), pluralSuffix(len(preview)), preview[0].Path)
	if rootMarkdownCount > 0 {
		summary += fmt.Sprintf(" %d root Markdown file%s will stay outside this split.", rootMarkdownCount, pluralSuffix(rootMarkdownCount))
	}
	return summary
}

func firstSourceStructureCard(settings models.SourceStructureSettings) models.SourceStructureCard {
	if len(settings.Cards) == 0 {
		return models.SourceStructureCard{}
	}
	return settings.Cards[0]
}

func previewTitle(reader SourceReader, root, identifier, titleTemplate string, captures map[string]string) string {
	title := strings.TrimSpace(renderSettingsTemplate(titleTemplate, captures))
	if title != "" && title != "readme_heading" {
		return title
	}
	if data, err := reader.ReadFile(filepath.ToSlash(filepath.Join(root, "README.md"))); err == nil {
		if headingTitle := titleFromHeading(firstHeading(string(data)), identifier); headingTitle != "" {
			return headingTitle
		}
	}
	return titleFromIdentifier(identifier)
}

func previewTags(values []string, captures map[string]string) []string {
	if values == nil {
		return []string{}
	}
	tags := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		tag := strings.TrimSpace(renderSettingsTemplate(value, captures))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	return tags
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
