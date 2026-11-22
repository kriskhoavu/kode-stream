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

	"kode-stream/internal/common/models"
)

// scanCandidate is one Markdown file found under a wiki source.
type scanCandidate struct {
	path          string // absolute
	rel           string // relative to the wiki root
	workspacePath string // relative to the workspace
}

// sourceScan reads one wiki source inside a workspace within its limits.
type sourceScan struct {
	ctx           context.Context
	detector      *Detector
	workspace     models.WorkspaceConfig
	workspaceRoot string
	source        string // the cleaned source, relative to the workspace
	root          string // the source's resolved absolute path
	limits        ScanLimits

	pages      []KnowledgePage
	warnings   []KnowledgeWarning
	files      int
	totalBytes int64
	links      int
}

// resolveSourceRoot checks the source stays inside the workspace and holds
// an index.md; found is false when it does not exist or is not a wiki.
func resolveSourceRoot(workspaceRoot, source string) (clean, resolved string, found bool, err error) {
	clean = filepath.Clean(strings.TrimSpace(source))
	if clean == "." || clean == "" || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", "", false, fmt.Errorf("unsafe source path %q", source)
	}
	resolved, err = filepath.EvalSymlinks(filepath.Join(workspaceRoot, clean))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return clean, "", false, nil
		}
		return "", "", false, err
	}
	if !withinPath(workspaceRoot, resolved) {
		return "", "", false, fmt.Errorf("source escapes workspace")
	}
	indexInfo, err := os.Lstat(filepath.Join(resolved, "index.md"))
	if err != nil || !indexInfo.Mode().IsRegular() {
		return clean, resolved, false, nil
	}
	return clean, resolved, true, nil
}

// candidates lists the source's Markdown files, skipping hidden directories
// and symlinks, within the file budget.
func (s *sourceScan) candidates() ([]scanCandidate, error) {
	found := make([]scanCandidate, 0)
	err := filepath.WalkDir(s.root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := s.ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			if current != s.root && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}
		if _, err := entry.Info(); err != nil {
			return err
		}
		rel, err := filepath.Rel(s.root, current)
		if err != nil {
			return err
		}
		relWorkspace, err := filepath.Rel(s.workspaceRoot, current)
		if err != nil {
			return err
		}
		found = append(found, scanCandidate{path: current, rel: filepath.ToSlash(rel), workspacePath: filepath.ToSlash(relWorkspace)})
		if len(found) > s.limits.MaxFiles {
			return fmt.Errorf("file budget exceeded")
		}
		return nil
	})
	return found, err
}

// ignored names the candidates the workspace's ignore rules exclude.
func (s *sourceScan) ignored(candidates []scanCandidate) (map[string]bool, error) {
	if s.detector.ignore == nil {
		return map[string]bool{}, nil
	}
	paths := make([]string, len(candidates))
	for index, candidate := range candidates {
		paths[index] = candidate.workspacePath
	}
	return s.detector.ignore.Ignored(s.workspaceRoot, paths)
}

func (s *sourceScan) warn(path string, code WarningCode, message string) {
	s.warnings = append(s.warnings, KnowledgeWarning{WorkspaceID: s.workspace.ID, WikiRoot: filepath.ToSlash(s.source), Path: path, Code: code, Message: message})
}

// adopt stamps warnings from a later step with this scan's workspace and root.
func (s *sourceScan) adopt(warnings []KnowledgeWarning) {
	for index := range warnings {
		warnings[index].WorkspaceID = s.workspace.ID
		warnings[index].WikiRoot = filepath.ToSlash(s.source)
	}
	s.warnings = append(s.warnings, warnings...)
}

// readPage reads and parses one candidate, charging it to the budgets.
func (s *sourceScan) readPage(file scanCandidate) error {
	s.files++
	if s.files > s.limits.MaxFiles {
		return fmt.Errorf("file budget exceeded")
	}
	data, warning, err := readStableBounded(s.ctx, file.path, min(s.limits.MaxFileBytes, s.limits.MaxTotalBytes-s.totalBytes))
	if warning != "" {
		s.warn(file.rel, "file_too_large", "file exceeds scan size limit")
		return nil
	}
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		s.warn(file.rel, "invalid_metadata", "file changed or could not be read during scan")
		return nil
	}
	s.totalBytes += int64(len(data))
	if s.totalBytes > s.limits.MaxTotalBytes { // defensive: the bounded reader must make this unreachable.
		return fmt.Errorf("byte budget exceeded")
	}
	page, pageWarnings, err := ParsePage(file.rel, data)
	s.adopt(pageWarnings)
	if err != nil {
		return nil
	}
	s.pages = append(s.pages, page)
	if len(s.pages) > s.limits.MaxPages {
		return fmt.Errorf("page budget exceeded")
	}
	s.links += len(page.Links)
	if s.links > s.limits.MaxLinks {
		return fmt.Errorf("link budget exceeded")
	}
	return nil
}

// wiki resolves the pages' relationships and taxonomy into the wiki.
func (s *sourceScan) wiki() KnowledgeWiki {
	pages, relationshipWarnings, _ := ResolveRelationships(s.pages)
	s.adopt(relationshipWarnings)
	pages, journeysBucket, taxonomyWarnings := applyTaxonomy(s.root, pages)
	s.adopt(taxonomyWarnings)
	sort.Slice(s.warnings, func(i, j int) bool {
		if s.warnings[i].Path == s.warnings[j].Path {
			return s.warnings[i].Code < s.warnings[j].Code
		}
		return s.warnings[i].Path < s.warnings[j].Path
	})
	return KnowledgeWiki{WorkspaceID: s.workspace.ID, Root: filepath.ToSlash(s.source), DisplayName: displayName(s.source), JourneysBucket: journeysBucket, Pages: pages, Warnings: s.warnings, IndexedAt: time.Now().UTC()}
}

// run scans the source; found is false when it holds no readable page.
func (s *sourceScan) run() (KnowledgeWiki, bool, error) {
	candidates, err := s.candidates()
	if err != nil {
		return KnowledgeWiki{}, false, fmt.Errorf("scan %s: %w", s.source, err)
	}
	ignored, err := s.ignored(candidates)
	if err != nil {
		return KnowledgeWiki{}, false, fmt.Errorf("scan ignores: %w", err)
	}
	s.pages, s.warnings = make([]KnowledgePage, 0), make([]KnowledgeWarning, 0)
	for _, file := range candidates {
		if err := s.ctx.Err(); err != nil {
			return KnowledgeWiki{}, false, fmt.Errorf("scan %s: %w", s.source, err)
		}
		if ignored[file.workspacePath] {
			continue
		}
		if err := s.readPage(file); err != nil {
			return KnowledgeWiki{}, false, fmt.Errorf("scan %s: %w", s.source, err)
		}
	}
	if len(s.pages) == 0 {
		return KnowledgeWiki{}, false, nil
	}
	return s.wiki(), true, nil
}
