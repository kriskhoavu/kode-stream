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
	"plan-manager/internal/workspacefiles"
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

type Detector struct {
	Limits ScanLimits
	ignore workspacefiles.IgnoreChecker
}

func NewDetector() *Detector {
	return &Detector{Limits: DefaultScanLimits(), ignore: workspacefiles.NewGitIgnoreChecker()}
}

func (d *Detector) DetectWorkspace(ctx context.Context, workspace models.WorkspaceConfig) ([]KnowledgeWiki, error) {
	if workspace.Knowledge != nil && workspace.Knowledge.Enabled != nil && !*workspace.Knowledge.Enabled {
		return []KnowledgeWiki{}, nil
	}
	wikis := make([]KnowledgeWiki, 0)
	seen := make(map[string]struct{}, len(workspace.Sources))
	for _, source := range workspace.Sources {
		key := filepath.ToSlash(filepath.Clean(strings.TrimSpace(source)))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		wiki, ok, err := d.DetectSource(ctx, workspace, source)
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

func (d *Detector) DetectSource(parent context.Context, workspace models.WorkspaceConfig, source string) (KnowledgeWiki, bool, error) {
	if workspace.Knowledge != nil && workspace.Knowledge.Enabled != nil && !*workspace.Knowledge.Enabled {
		return KnowledgeWiki{}, false, nil
	}
	workspaceRoot, err := filepath.Abs(workspace.Path)
	if err != nil {
		return KnowledgeWiki{}, false, fmt.Errorf("resolve workspace: %w", err)
	}
	workspaceRoot, err = filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		return KnowledgeWiki{}, false, fmt.Errorf("resolve workspace: %w", err)
	}
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

	type candidate struct {
		path string
		rel  string
		size int64
	}
	candidates := make([]candidate, 0)
	workspacePaths := make([]string, 0)
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
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		rel, relErr := filepath.Rel(resolved, current)
		if relErr != nil {
			return relErr
		}
		relWorkspace, relErr := filepath.Rel(workspaceRoot, current)
		if relErr != nil {
			return relErr
		}
		candidates = append(candidates, candidate{path: current, rel: filepath.ToSlash(rel), size: info.Size()})
		workspacePaths = append(workspacePaths, filepath.ToSlash(relWorkspace))
		if len(candidates) > limits.MaxFiles {
			return fmt.Errorf("file budget exceeded")
		}
		return nil
	})
	if err != nil {
		return KnowledgeWiki{}, false, fmt.Errorf("scan %s: %w", cleanSource, err)
	}
	ignored := map[string]bool{}
	if d.ignore != nil {
		ignored, err = d.ignore.Ignored(workspaceRoot, workspacePaths)
		if err != nil {
			return KnowledgeWiki{}, false, fmt.Errorf("scan ignores: %w", err)
		}
	}

	pages := make([]KnowledgePage, 0)
	warnings := make([]KnowledgeWarning, 0)
	files, totalBytes, links := 0, int64(0), 0
	for index, file := range candidates {
		select {
		case <-ctx.Done():
			return KnowledgeWiki{}, false, fmt.Errorf("scan %s: %w", cleanSource, ctx.Err())
		default:
		}
		if ignored[workspacePaths[index]] {
			continue
		}
		files++
		if files > limits.MaxFiles {
			return KnowledgeWiki{}, false, fmt.Errorf("scan %s: file budget exceeded", cleanSource)
		}
		if file.size > limits.MaxFileBytes {
			warnings = append(warnings, KnowledgeWarning{WorkspaceID: workspace.ID, WikiRoot: filepath.ToSlash(cleanSource), Path: file.rel, Code: "file_too_large", Message: "file exceeds scan size limit"})
			continue
		}
		totalBytes += file.size
		if totalBytes > limits.MaxTotalBytes {
			return KnowledgeWiki{}, false, fmt.Errorf("scan %s: byte budget exceeded", cleanSource)
		}
		data, err := os.ReadFile(file.path)
		if err != nil {
			return KnowledgeWiki{}, false, err
		}
		page, pageWarnings, err := ParsePage(file.rel, data)
		for index := range pageWarnings {
			pageWarnings[index].WorkspaceID = workspace.ID
			pageWarnings[index].WikiRoot = filepath.ToSlash(cleanSource)
		}
		warnings = append(warnings, pageWarnings...)
		if err != nil {
			continue
		}
		pages = append(pages, page)
		if len(pages) > limits.MaxPages {
			return KnowledgeWiki{}, false, fmt.Errorf("scan %s: page budget exceeded", cleanSource)
		}
		links += len(page.Links)
		if links > limits.MaxLinks {
			return KnowledgeWiki{}, false, fmt.Errorf("scan %s: link budget exceeded", cleanSource)
		}
	}
	if len(pages) == 0 {
		return KnowledgeWiki{}, false, nil
	}
	pages, relationshipWarnings, _ := ResolveRelationships(pages)
	for index := range relationshipWarnings {
		relationshipWarnings[index].WorkspaceID = workspace.ID
		relationshipWarnings[index].WikiRoot = filepath.ToSlash(cleanSource)
	}
	warnings = append(warnings, relationshipWarnings...)
	sort.Slice(warnings, func(i, j int) bool {
		if warnings[i].Path == warnings[j].Path {
			return warnings[i].Code < warnings[j].Code
		}
		return warnings[i].Path < warnings[j].Path
	})
	return KnowledgeWiki{WorkspaceID: workspace.ID, Root: filepath.ToSlash(cleanSource), DisplayName: displayName(cleanSource), Pages: pages, Warnings: warnings, IndexedAt: time.Now().UTC()}, true, nil
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
