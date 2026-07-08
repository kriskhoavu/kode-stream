package scanner

// Package scanner discovers and parses Workspace sources.

import (
	"path"
	"path/filepath"
	"sort"
	"strings"

	"kode-stream/internal/common/models"
)

type workspaceSettingsMatch struct {
	path     string
	captures map[string]string
}

func matchPatternDirectories(reader SourceReader, root string, segments []pathPatternSegment) []workspaceSettingsMatch {
	var matches []workspaceSettingsMatch
	var walk func(current string, depth int, captures map[string]string)
	walk = func(current string, depth int, captures map[string]string) {
		if depth == len(segments) {
			if info, err := reader.Stat(current); err == nil && info.IsDir() {
				copied := map[string]string{}
				for key, value := range captures {
					copied[key] = value
				}
				matches = append(matches, workspaceSettingsMatch{path: current, captures: copied})
			}
			return
		}
		segment := segments[depth]
		if segment.literal != "" {
			next := path.Join(current, segment.literal)
			if info, err := reader.Stat(next); err == nil && info.IsDir() {
				walk(next, depth+1, captures)
			}
			return
		}
		entries, err := reader.ReadDir(current)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			captures[segment.variable] = entry.Name()
			walk(path.Join(current, entry.Name()), depth+1, captures)
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
	detail.Scope = renderSettingsTemplate(firstNonEmpty(fields.Source, fields.Scope), captures)
	detail.Identifier = renderSettingsTemplate(firstNonEmpty(fields.Item, fields.Identifier), captures)
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
