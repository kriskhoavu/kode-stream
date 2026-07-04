package knowledge

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	wikiLinkPattern     = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
	markdownLinkPattern = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+\.md(?:#[^)]*)?)\)`)
	inlineCodePattern   = regexp.MustCompile("`+[^`\n]*`+")
)

type frontMatter struct {
	Slug        string    `yaml:"slug"`
	Title       string    `yaml:"title"`
	PageType    string    `yaml:"pageType"`
	Roles       yaml.Node `yaml:"roles"`
	Topics      yaml.Node `yaml:"topics"`
	Summary     string    `yaml:"summary"`
	SourceRef   yaml.Node `yaml:"sourceRef"`
	SourceCount yaml.Node `yaml:"sourceCount"`
}

func ParsePage(relativePath string, data []byte) (KnowledgePage, []KnowledgeWarning, error) {
	relativePath = normalizePath(relativePath)
	metadata, body, err := splitFrontMatter(data)
	if err != nil {
		return KnowledgePage{}, []KnowledgeWarning{{Path: relativePath, Code: WarningInvalidFrontMatter, Message: err.Error()}}, err
	}
	var raw frontMatter
	if err := yaml.Unmarshal(metadata, &raw); err != nil {
		return KnowledgePage{}, []KnowledgeWarning{{Path: relativePath, Code: WarningInvalidFrontMatter, Message: "front matter is not valid YAML"}}, fmt.Errorf("parse front matter: %w", err)
	}
	if strings.TrimSpace(raw.Slug) == "" || strings.TrimSpace(raw.Title) == "" {
		err := fmt.Errorf("front matter requires slug and title")
		return KnowledgePage{}, []KnowledgeWarning{{Path: relativePath, Code: WarningMissingIdentity, Message: err.Error()}}, err
	}

	warnings := make([]KnowledgeWarning, 0)
	roles, err := normalizeStringList(raw.Roles)
	if err != nil {
		warnings = append(warnings, metadataWarning(relativePath, raw.Slug, "roles", err))
	}
	topics, err := normalizeStringList(raw.Topics)
	if err != nil {
		warnings = append(warnings, metadataWarning(relativePath, raw.Slug, "topics", err))
	}
	sourceRefs, err := normalizeStringList(raw.SourceRef)
	if err != nil {
		warnings = append(warnings, metadataWarning(relativePath, raw.Slug, "sourceRef", err))
	}
	sourceCount, err := normalizeInt(raw.SourceCount)
	if err != nil {
		warnings = append(warnings, metadataWarning(relativePath, raw.Slug, "sourceCount", err))
	}

	page := KnowledgePage{
		Slug: strings.TrimSpace(raw.Slug), Title: strings.TrimSpace(raw.Title), Path: relativePath,
		Domain: domainForPath(relativePath), PageType: strings.TrimSpace(raw.PageType),
		Roles: roles, Topics: topics, Summary: strings.TrimSpace(raw.Summary),
		SourceRefs: sourceRefs, SourceCount: sourceCount,
	}
	page.Links = extractLinks(page.Slug, body)
	return page, warnings, nil
}

func splitFrontMatter(data []byte) ([]byte, string, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, "", fmt.Errorf("missing YAML front matter")
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		return nil, "", fmt.Errorf("unterminated YAML front matter")
	}
	end += 4
	return []byte(text[4:end]), text[end+5:], nil
}

func normalizeStringList(node yaml.Node) ([]string, error) {
	if node.Kind == 0 || node.Tag == "!!null" {
		return []string{}, nil
	}
	values := make([]string, 0)
	switch node.Kind {
	case yaml.ScalarNode:
		for _, line := range strings.Split(node.Value, "\n") {
			for _, value := range strings.Split(line, ",") {
				if value = strings.TrimSpace(value); value != "" {
					values = append(values, value)
				}
			}
		}
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if item.Kind != yaml.ScalarNode {
				return []string{}, fmt.Errorf("must contain only strings")
			}
			if value := strings.TrimSpace(item.Value); value != "" {
				values = append(values, value)
			}
		}
	default:
		return []string{}, fmt.Errorf("must be a string or list")
	}
	return uniqueStrings(values), nil
}

func normalizeInt(node yaml.Node) (int, error) {
	if node.Kind == 0 || node.Tag == "!!null" || strings.TrimSpace(node.Value) == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(node.Value))
	if err != nil || value < 0 {
		return 0, fmt.Errorf("must be a non-negative integer")
	}
	return value, nil
}

func extractLinks(sourceSlug, body string) []KnowledgeLink {
	body = stripCode(body)
	links := make([]KnowledgeLink, 0)
	for _, match := range wikiLinkPattern.FindAllStringSubmatch(body, -1) {
		links = append(links, KnowledgeLink{SourceSlug: sourceSlug, RawTarget: strings.TrimSpace(match[1]), Label: strings.TrimSpace(match[2]), Resolution: LinkUnresolved})
	}
	for _, match := range markdownLinkPattern.FindAllStringSubmatch(body, -1) {
		target := strings.TrimSpace(strings.SplitN(match[2], "#", 2)[0])
		links = append(links, KnowledgeLink{SourceSlug: sourceSlug, RawTarget: target, Label: strings.TrimSpace(match[1]), Resolution: LinkUnresolved})
	}
	return links
}

func stripCode(body string) string {
	lines := strings.Split(body, "\n")
	inFence := false
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			lines[index] = ""
			continue
		}
		if inFence {
			lines[index] = ""
			continue
		}
		lines[index] = inlineCodePattern.ReplaceAllString(line, "")
	}
	return strings.Join(lines, "\n")
}

func metadataWarning(path, slug, field string, err error) KnowledgeWarning {
	return KnowledgeWarning{Path: path, Slug: slug, Code: WarningInvalidMetadata, Message: field + " " + err.Error()}
}

func domainForPath(relativePath string) string {
	directory := path.Dir(relativePath)
	if directory == "." {
		return "root"
	}
	return directory
}

func normalizePath(value string) string {
	return strings.TrimPrefix(path.Clean(strings.ReplaceAll(value, "\\", "/")), "./")
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
