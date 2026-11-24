package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/domain"
)

const maxFiles = 200

const maxFileBytes = 64 << 10

var keyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

type Scout struct{}

func New() *Scout { return &Scout{} }

func (s *Scout) Kind() domain.SourceKind { return domain.SourceLocal }

func (s *Scout) Enabled(_ models.WorkspaceConfig, settings domain.WorkspaceSettings) bool {
	return settings.Local != nil && settings.Local.Enabled && strings.TrimSpace(settings.Local.Dir) != ""
}

func Dir(workspace models.WorkspaceConfig, settings domain.WorkspaceSettings) string {
	dir := strings.TrimSpace(settings.Local.Dir)
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	return filepath.Join(workspace.Path, dir)
}

func (s *Scout) Poll(ctx context.Context, workspace models.WorkspaceConfig, settings domain.WorkspaceSettings) ([]domain.Candidate, error) {
	if !s.Enabled(workspace, settings) {
		return nil, fmt.Errorf("workspace %s has no local task folder", workspace.ID)
	}
	dir := Dir(workspace, settings)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read the task folder %s: %w", dir, err)
	}
	names := []string{}
	for _, entry := range entries {
		key := strings.TrimSuffix(entry.Name(), ".md")
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".md") && keyPattern.MatchString(key) {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) > maxFiles {
		names = names[:maxFiles]
	}
	candidates := make([]domain.Candidate, 0, len(names))
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil || info.Size() > maxFileBytes {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		key := strings.TrimSuffix(name, ".md")
		title, body := parse(string(data), key)
		updated := info.ModTime().UTC()
		source := domain.Source{Kind: domain.SourceLocal, Key: key, Status: "Open", Labels: []string{}, UpdatedAt: &updated}
		candidates = append(candidates, domain.Candidate{WorkspaceID: workspace.ID, Source: source, Title: title, Body: body})
	}
	return candidates, nil
}

func parse(text, key string) (string, string) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for index, line := range lines {
		if title, ok := strings.CutPrefix(strings.TrimSpace(line), "# "); ok && strings.TrimSpace(title) != "" {
			rest := append(append([]string{}, lines[:index]...), lines[index+1:]...)
			return strings.TrimSpace(title), strings.TrimSpace(strings.Join(rest, "\n"))
		}
	}
	return key, strings.TrimSpace(text)
}
