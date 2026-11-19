// Package local lists tasks written as Markdown files in a folder, so a
// person can try the harness's pipeline without an issue tracker. Each file
// is one task: its name is the task's key, its first heading the title, and
// the rest the body. It reads only.
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

// maxFiles bounds how many task files one poll reads.
const maxFiles = 200

// maxFileBytes bounds one task file; the poller trims the body further.
const maxFileBytes = 64 << 10

// keyPattern is the file names that are tasks: SANDBOX-1.md, not notes.txt.
var keyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

type Scout struct{}

func New() *Scout { return &Scout{} }

func (s *Scout) Kind() domain.SourceKind { return domain.SourceLocal }

func (s *Scout) Enabled(_ models.WorkspaceConfig, settings domain.WorkspaceSettings) bool {
	return settings.Local != nil && settings.Local.Enabled && strings.TrimSpace(settings.Local.Dir) != ""
}

// Dir is the task folder: an absolute Dir as it is, a relative one inside
// the workspace's checkout.
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

// parse takes the first "# " heading as the title and the rest as the body.
// A file with no heading is titled by its key.
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
