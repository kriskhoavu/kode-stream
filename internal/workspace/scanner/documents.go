package scanner

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/fileid"
)

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
	doc := models.ItemDocument{ID: fileid.Encode(path), Role: "other", Path: path, Label: labelFromPath(path)}
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
	case strings.HasPrefix(lower, "design/"), strings.HasPrefix(lower, "solution/"):
		// Both folders carry the technical design role: `solution/` is the
		// current feature-planning layout, `design/` the earlier one that
		// stays valid for existing plans and for UI/UX design documents.
		doc.Role = "design"
		name := stripDocumentSequence(path, strings.SplitN(lower, "/", 2)[0])
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
