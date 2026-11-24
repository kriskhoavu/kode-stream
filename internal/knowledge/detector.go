package knowledge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/workspace/files"
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
	for _, source := range workspace.SourcesShownIn(models.SourceSurfaceKnowledge) {
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
	cleanSource, resolved, found, err := resolveSourceRoot(workspaceRoot, source)
	if err != nil || !found {
		return KnowledgeWiki{}, false, err
	}
	scan := &sourceScan{ctx: ctx, detector: d, workspace: workspace, workspaceRoot: workspaceRoot, sourcePath: cleanSource, resolvedRoot: resolved, limits: limits}
	return scan.run()
}

// Stat before and after the read, so a file swapped after discovery cannot pass a larger body.
func readStableBounded(ctx context.Context, path string, limit int64) ([]byte, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, "", err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, "file_too_large", nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	data := make([]byte, 0, info.Size())
	buffer := make([]byte, 32<<10)
	for int64(len(data)) < limit {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		want := min(int64(len(buffer)), limit-int64(len(data)))
		n, readErr := file.Read(buffer[:want])
		data = append(data, buffer[:n]...)
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, "", readErr
		}
	}
	after, err := os.Lstat(path)
	if err != nil {
		return nil, "", err
	}
	if !after.Mode().IsRegular() || after.Size() != int64(len(data)) || after.ModTime() != info.ModTime() {
		if after.Size() > limit {
			return nil, "file_too_large", nil
		}
		return nil, "", errors.New("file changed during scan")
	}
	return data, "", ctx.Err()
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
