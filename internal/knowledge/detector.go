package knowledge

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"plan-manager/internal/models"
)

const (
	DefaultMaxFiles      = 10_000
	DefaultMaxFileBytes  = int64(2 << 20)
	DefaultMaxTotalBytes = int64(64 << 20)
	DefaultMaxPages      = 5_000
	DefaultMaxLinks      = 100_000
	DefaultScanTimeout   = 15 * time.Second
)

type ScanLimits struct {
	MaxFiles      int
	MaxFileBytes  int64
	MaxTotalBytes int64
	MaxPages      int
	MaxLinks      int
	Timeout       time.Duration
}

func DefaultScanLimits() ScanLimits {
	return ScanLimits{DefaultMaxFiles, DefaultMaxFileBytes, DefaultMaxTotalBytes, DefaultMaxPages, DefaultMaxLinks, DefaultScanTimeout}
}

type Detector struct{ Limits ScanLimits }

func NewDetector() *Detector { return &Detector{Limits: DefaultScanLimits()} }

func (d *Detector) DetectWorkspace(ctx context.Context, workspace models.WorkspaceConfig) ([]KnowledgeWiki, error) {
	if workspace.Knowledge != nil && workspace.Knowledge.Enabled != nil && !*workspace.Knowledge.Enabled {
		return []KnowledgeWiki{}, nil
	}
	root, err := filepath.Abs(workspace.Path)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace: %w", err)
	}

	wikis := make([]KnowledgeWiki, 0)
	for _, source := range workspace.Sources {
		wiki, ok, err := d.detectSource(ctx, workspace.ID, root, source)
		if err != nil {
			return nil, err
		}
		if ok {
			wikis = append(wikis, wiki)
		}
	}
	sort.Slice(wikis, func(i, j int) bool { return wikis[i].Root < wikis[j].Root })
	return wikis, nil
}

func (d *Detector) detectSource(parent context.Context, workspaceID, workspaceRoot, source string) (KnowledgeWiki, bool, error) {
	limits := d.Limits
	if limits.Timeout <= 0 {
		limits = DefaultScanLimits()
	}
	ctx, cancel := context.WithTimeout(parent, limits.Timeout)
	defer cancel()
	cleanSource := filepath.Clean(strings.TrimSpace(source))
	if cleanSource == "." || cleanSource == "" || filepath.IsAbs(cleanSource) || strings.HasPrefix(cleanSource, "..") {
		return KnowledgeWiki{}, false, fmt.Errorf("unsafe source path %q", source)
	}
	root := filepath.Join(workspaceRoot, cleanSource)
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return KnowledgeWiki{}, false, nil
		}
		return KnowledgeWiki{}, false, err
	}
	if !withinPath(workspaceRoot, resolved) {
		return KnowledgeWiki{}, false, fmt.Errorf("source escapes workspace")
	}
	indexInfo, err := os.Lstat(filepath.Join(resolved, "index.md"))
	if err != nil || !indexInfo.Mode().IsRegular() {
		return KnowledgeWiki{}, false, nil
	}

	pages := make([]KnowledgePage, 0)
	warnings := make([]KnowledgeWarning, 0)
	files, totalBytes, links := 0, int64(0), 0
	err = filepath.WalkDir(resolved, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if entry.IsDir() {
			if current != resolved && (entry.Name() == ".git" || strings.HasPrefix(entry.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}
		files++
		if files > limits.MaxFiles {
			return fmt.Errorf("file budget exceeded")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(resolved, current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if info.Size() > limits.MaxFileBytes {
			warnings = append(warnings, KnowledgeWarning{WorkspaceID: workspaceID, WikiRoot: filepath.ToSlash(cleanSource), Path: rel, Code: "file_too_large", Message: "file exceeds scan size limit"})
			return nil
		}
		totalBytes += info.Size()
		if totalBytes > limits.MaxTotalBytes {
			return fmt.Errorf("byte budget exceeded")
		}
		data, err := os.ReadFile(current)
		if err != nil {
			return err
		}
		page, pageWarnings, err := ParsePage(rel, data)
		for index := range pageWarnings {
			pageWarnings[index].WorkspaceID = workspaceID
			pageWarnings[index].WikiRoot = filepath.ToSlash(cleanSource)
		}
		warnings = append(warnings, pageWarnings...)
		if err != nil {
			return nil
		}
		pages = append(pages, page)
		if len(pages) > limits.MaxPages {
			return fmt.Errorf("page budget exceeded")
		}
		links += len(page.Links)
		if links > limits.MaxLinks {
			return fmt.Errorf("link budget exceeded")
		}
		return nil
	})
	if err != nil {
		return KnowledgeWiki{}, false, fmt.Errorf("scan %s: %w", cleanSource, err)
	}
	if len(pages) == 0 {
		return KnowledgeWiki{}, false, nil
	}
	pages, relationshipWarnings, _ := ResolveRelationships(pages)
	for index := range relationshipWarnings {
		relationshipWarnings[index].WorkspaceID = workspaceID
		relationshipWarnings[index].WikiRoot = filepath.ToSlash(cleanSource)
	}
	warnings = append(warnings, relationshipWarnings...)
	sort.Slice(warnings, func(i, j int) bool {
		if warnings[i].Path == warnings[j].Path {
			return warnings[i].Code < warnings[j].Code
		}
		return warnings[i].Path < warnings[j].Path
	})
	return KnowledgeWiki{WorkspaceID: workspaceID, Root: filepath.ToSlash(cleanSource), DisplayName: displayName(cleanSource), Pages: pages, Warnings: warnings, IndexedAt: time.Now().UTC()}, true, nil
}

func withinPath(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func displayName(source string) string {
	name := filepath.Base(source)
	if name == "." || name == "" {
		return "Knowledge"
	}
	return strings.ToUpper(name[:1]) + name[1:]
}
