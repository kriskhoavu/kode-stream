package scanner

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"plan-manager/internal/gitadapter"
)

type DirEntry interface {
	Name() string
	IsDir() bool
	Type() fs.FileMode
	Info() (FileInfo, error)
}

type FileInfo = fs.FileInfo

type WalkFunc func(path string, d DirEntry, err error) error

type SourceReader interface {
	ReadDir(path string) ([]DirEntry, error)
	ReadFile(path string) ([]byte, error)
	WalkDir(root string, fn WalkFunc) error
	Stat(path string) (FileInfo, error)
}

type FilesystemSourceReader struct {
	root string
}

func NewFilesystemSourceReader(root string) *FilesystemSourceReader {
	return &FilesystemSourceReader{root: root}
}

func (r *FilesystemSourceReader) ReadDir(relPath string) ([]DirEntry, error) {
	entries, err := os.ReadDir(r.fullPath(relPath))
	if err != nil {
		return nil, err
	}
	out := make([]DirEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry)
	}
	return out, nil
}

func (r *FilesystemSourceReader) ReadFile(relPath string) ([]byte, error) {
	return os.ReadFile(r.fullPath(relPath))
}

func (r *FilesystemSourceReader) WalkDir(root string, fn WalkFunc) error {
	base := r.fullPath(root)
	return filepath.WalkDir(base, func(current string, d fs.DirEntry, err error) error {
		rel := root
		if err == nil {
			if fromBase, relErr := filepath.Rel(r.root, current); relErr == nil {
				rel = filepath.ToSlash(fromBase)
			}
		}
		return fn(rel, d, err)
	})
}

func (r *FilesystemSourceReader) Stat(relPath string) (FileInfo, error) {
	return os.Stat(r.fullPath(relPath))
}

func (r *FilesystemSourceReader) fullPath(relPath string) string {
	if strings.TrimSpace(relPath) == "" {
		return r.root
	}
	return filepath.Join(r.root, filepath.FromSlash(relPath))
}

type GitTreeSourceReader struct {
	workspacePath string
	ref           string
	git           *gitadapter.GitAdapter
}

func NewGitTreeSourceReader(workspacePath, ref string, git *gitadapter.GitAdapter) *GitTreeSourceReader {
	return &GitTreeSourceReader{workspacePath: workspacePath, ref: ref, git: git}
}

func (r *GitTreeSourceReader) ReadDir(relPath string) ([]DirEntry, error) {
	entries, err := r.git.TreeReadDir(r.workspacePath, r.ref, relPath)
	if err != nil {
		return nil, err
	}
	out := make([]DirEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, gitTreeDirEntry{entry: entry})
	}
	return out, nil
}

func (r *GitTreeSourceReader) ReadFile(relPath string) ([]byte, error) {
	return r.git.TreeReadFile(r.workspacePath, r.ref, relPath)
}

func (r *GitTreeSourceReader) WalkDir(root string, fn WalkFunc) error {
	cleanRoot := cleanSourcePath(root)
	if err := fn(cleanRoot, gitTreeDirEntry{entry: gitadapter.TreeEntry{Name: path.Base(cleanRoot), Path: cleanRoot, Type: fs.ModeDir}}, nil); err != nil {
		if err == fs.SkipDir || err == fs.SkipAll {
			return nil
		}
		return err
	}
	entries, err := r.git.TreeWalk(r.workspacePath, r.ref, cleanRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := fn(entry.Path, gitTreeDirEntry{entry: entry}, nil); err != nil {
			if err == fs.SkipDir {
				continue
			}
			if err == fs.SkipAll {
				return nil
			}
			return err
		}
	}
	return nil
}

func (r *GitTreeSourceReader) Stat(relPath string) (FileInfo, error) {
	clean := cleanSourcePath(relPath)
	if clean == "" {
		return gitTreeFileInfo{name: ".", mode: fs.ModeDir}, nil
	}
	parent := path.Dir(clean)
	if parent == "." {
		parent = ""
	}
	entries, err := r.git.TreeReadDir(r.workspacePath, r.ref, parent)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Path == clean {
			return gitTreeFileInfo{name: entry.Name, size: entry.Size, mode: entry.Type, modTime: entry.ModTime}, nil
		}
	}
	return nil, fs.ErrNotExist
}

type gitTreeDirEntry struct {
	entry gitadapter.TreeEntry
}

func (e gitTreeDirEntry) Name() string {
	return e.entry.Name
}

func (e gitTreeDirEntry) IsDir() bool {
	return e.entry.Type.IsDir()
}

func (e gitTreeDirEntry) Type() fs.FileMode {
	return e.entry.Type
}

func (e gitTreeDirEntry) Info() (FileInfo, error) {
	return gitTreeFileInfo{name: e.entry.Name, size: e.entry.Size, mode: e.entry.Type, modTime: e.entry.ModTime}, nil
}

type gitTreeFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
}

func (i gitTreeFileInfo) Name() string {
	return i.name
}

func (i gitTreeFileInfo) Size() int64 {
	return i.size
}

func (i gitTreeFileInfo) Mode() fs.FileMode {
	return i.mode
}

func (i gitTreeFileInfo) ModTime() time.Time {
	return i.modTime
}

func (i gitTreeFileInfo) IsDir() bool {
	return i.mode.IsDir()
}

func (i gitTreeFileInfo) Sys() any {
	return nil
}

func cleanSourcePath(relPath string) string {
	clean := path.Clean(strings.TrimSpace(filepath.ToSlash(relPath)))
	if clean == "." {
		return ""
	}
	return clean
}
