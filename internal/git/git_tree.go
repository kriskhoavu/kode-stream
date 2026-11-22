package git

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type TreeEntry struct {
	Name    string
	Path    string
	Type    fs.FileMode
	Mode    string
	Object  string
	Size    int64
	ModTime time.Time
}

func (g *GitAdapter) TreeReadDir(workspacePath, ref, relPath string) ([]TreeEntry, error) {
	clean, err := cleanGitTreePath(relPath)
	if err != nil {
		return nil, err
	}
	out, err := g.run(workspacePath, "ls-tree", "-z", "-l", treeish(ref, clean))
	if err != nil {
		return nil, err
	}
	return parseTreeEntries(clean, out)
}

func (g *GitAdapter) TreeReadFile(workspacePath, ref, relPath string) ([]byte, error) {
	clean, err := cleanGitTreePath(relPath)
	if err != nil {
		return nil, err
	}
	out, err := g.run(workspacePath, "show", treeish(ref, clean))
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

func (g *GitAdapter) TreeWalk(workspacePath, ref, root string) ([]TreeEntry, error) {
	clean, err := cleanGitTreePath(root)
	if err != nil {
		return nil, err
	}
	out, err := g.run(workspacePath, "ls-tree", "-r", "-t", "-z", "-l", treeish(ref, clean))
	if err != nil {
		return nil, err
	}
	return parseTreeEntries(clean, out)
}

// TreeWalkBounded lists a review tree without allowing git output or the
// returned model to grow without bound. The caller receives a stable limit
// error instead of a partial tree with unknown omissions.
func (g *GitAdapter) TreeWalkBounded(ctx context.Context, workspacePath, ref, root string, maxEntries, maxDepth int) ([]TreeEntry, error) {
	clean, err := cleanGitTreePath(root)
	if err != nil {
		return nil, err
	}
	if maxEntries <= 0 || maxDepth < 0 {
		return nil, ErrTreeLimit
	}
	// ls-tree records are small, but cap stdout too so a pathological tree is
	// terminated before an unbounded buffer allocation.
	outputLimit := int64(maxEntries*1024 + 4096)
	out, err := g.runContextLimited(ctx, workspacePath, outputLimit, "ls-tree", "-r", "-t", "-z", "-l", treeish(ref, clean))
	if err != nil {
		if errors.Is(err, ErrOutputLimit) {
			return nil, ErrTreeLimit
		}
		return nil, err
	}
	entries, err := parseTreeEntries(clean, string(out))
	if err != nil {
		return nil, err
	}
	if len(entries) > maxEntries {
		return nil, ErrTreeLimit
	}
	for _, entry := range entries {
		rel := strings.TrimPrefix(entry.Path, clean)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" || strings.HasPrefix(rel, "../") {
			return nil, ErrTreeLimit
		}
		if depth := strings.Count(rel, "/"); depth > maxDepth {
			return nil, ErrTreeLimit
		}
	}
	return entries, nil
}

// TreeReadFileBounded checks the Git object size before streaming a bounded
// prefix. Images are rejected when oversized; text callers can display the
// bounded prefix with an explicit truncation flag.
func (g *GitAdapter) TreeReadFileBounded(ctx context.Context, workspacePath, ref, relPath string, maxBytes int64) ([]byte, int64, bool, error) {
	clean, err := cleanGitTreePath(relPath)
	if err != nil {
		return nil, 0, false, err
	}
	if maxBytes <= 0 {
		return nil, 0, false, ErrBlobLimit
	}
	sizeOut, err := g.runContextLimited(ctx, workspacePath, 128, "cat-file", "-s", treeish(ref, clean))
	if err != nil {
		return nil, 0, false, err
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(sizeOut)), 10, 64)
	if err != nil {
		return nil, 0, false, err
	}
	if size > maxBytes {
		return nil, size, true, ErrBlobLimit
	}
	out, err := g.runContextLimited(ctx, workspacePath, maxBytes+1, "show", treeish(ref, clean))
	if err != nil {
		return nil, 0, false, err
	}
	return out, size, size > maxBytes, nil
}

func cleanGitTreePath(relPath string) (string, error) {
	clean := path.Clean(strings.TrimSpace(filepath.ToSlash(relPath)))
	if clean == "." {
		return "", nil
	}
	if strings.HasPrefix(clean, "../") || clean == ".." || strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("invalid git tree path %q", relPath)
	}
	return clean, nil
}

func treeish(ref, relPath string) string {
	if strings.TrimSpace(relPath) == "" {
		return ref + ":"
	}
	return ref + ":" + relPath
}

func parseTreeEntries(root, out string) ([]TreeEntry, error) {
	if strings.Trim(out, "\x00\n\t ") == "" {
		return []TreeEntry{}, nil
	}
	records := strings.Split(out, "\x00")
	entries := make([]TreeEntry, 0, len(records))
	for _, record := range records {
		if record == "" {
			continue
		}
		meta, entryPath, ok := strings.Cut(record, "\t")
		if !ok {
			return nil, fmt.Errorf("invalid ls-tree record %q", record)
		}
		fields := strings.Fields(meta)
		if len(fields) < 4 {
			return nil, fmt.Errorf("invalid ls-tree metadata %q", meta)
		}
		size := int64(0)
		if fields[3] != "-" {
			parsed, err := strconv.ParseInt(fields[3], 10, 64)
			if err != nil {
				return nil, err
			}
			size = parsed
		}
		fullPath := path.Clean(path.Join(root, entryPath))
		if root == "" {
			fullPath = path.Clean(entryPath)
		}
		mode := fs.FileMode(0)
		if fields[1] == "tree" {
			mode = fs.ModeDir
		}
		entries = append(entries, TreeEntry{
			Name:   path.Base(fullPath),
			Path:   fullPath,
			Type:   mode,
			Mode:   fields[0],
			Object: fields[2],
			Size:   size,
		})
	}
	return entries, nil
}
