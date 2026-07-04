package workspacefiles

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"plan-manager/internal/fileaccess"
	"plan-manager/internal/models"
)

var (
	ErrInvalidPath   = errors.New("invalid workspace path")
	ErrProtectedPath = errors.New("protected workspace path")
	ErrOutsideRoot   = errors.New("workspace path escapes root")
	ErrHashRequired  = errors.New("expected hash is required")
	ErrStaleContent  = errors.New("file content changed since it was loaded")
)

type IgnoreChecker interface {
	Ignored(workspaceRoot string, paths []string) (map[string]bool, error)
}

type Access struct {
	ignore            IgnoreChecker
	searchResultLimit int
	searchEntryLimit  int
}

func New() *Access {
	return &Access{ignore: gitIgnoreChecker{}, searchResultLimit: MaxSearchResults, searchEntryLimit: MaxSearchEntries}
}

func NewWithIgnoreChecker(ignore IgnoreChecker) *Access {
	return &Access{ignore: ignore, searchResultLimit: MaxSearchResults, searchEntryLimit: MaxSearchEntries}
}

func (a *Access) List(workspace models.WorkspaceConfig, path string, includeIgnored bool) (models.WorkspaceDirectoryListing, error) {
	clean, full, err := resolve(workspace.Path, path, true)
	if err != nil {
		return models.WorkspaceDirectoryListing{}, err
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return models.WorkspaceDirectoryListing{}, err
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, joinRelative(clean, entry.Name()))
	}
	ignored := map[string]bool{}
	if a.ignore != nil && len(paths) > 0 {
		ignored, err = a.ignore.Ignored(workspace.Path, paths)
		if err != nil {
			return models.WorkspaceDirectoryListing{}, err
		}
	}

	listing := models.WorkspaceDirectoryListing{WorkspaceID: workspace.ID, Path: clean, Entries: []models.WorkspaceTreeEntry{}}
	for i, entry := range entries {
		relPath := paths[i]
		if protected(relPath) {
			listing.HiddenCount++
			continue
		}
		isIgnored := ignored[relPath]
		if isIgnored && !includeIgnored {
			listing.HiddenCount++
			continue
		}
		node, ok := treeEntry(workspace.Path, relPath, entry, isIgnored)
		if !ok {
			listing.HiddenCount++
			continue
		}
		listing.Entries = append(listing.Entries, node)
	}
	sort.SliceStable(listing.Entries, func(i, j int) bool {
		if listing.Entries[i].Type != listing.Entries[j].Type {
			return listing.Entries[i].Type == "directory"
		}
		return naturalLess(listing.Entries[i].Name, listing.Entries[j].Name)
	})
	return listing, nil
}

func (a *Access) ResolveFile(workspace models.WorkspaceConfig, path string) (string, string, error) {
	return resolve(workspace.Path, path, false)
}

func (a *Access) Read(workspace models.WorkspaceConfig, path string) (models.FileContent, error) {
	clean, full, err := a.ResolveFile(workspace, path)
	if err != nil {
		return models.FileContent{}, err
	}
	return fileaccess.ReadFileContent(clean, full)
}

func (a *Access) WriteMarkdown(workspace models.WorkspaceConfig, input models.WorkspaceFileSaveInput) (models.FileContent, error) {
	if strings.TrimSpace(input.ExpectedHash) == "" {
		return models.FileContent{}, ErrHashRequired
	}
	clean, full, err := a.ResolveFile(workspace, input.Path)
	if err != nil {
		return models.FileContent{}, err
	}
	current, err := os.ReadFile(full)
	if err != nil {
		return models.FileContent{}, err
	}
	if err := fileaccess.ValidateEditableContent(current, []byte(input.Content)); err != nil {
		return models.FileContent{}, err
	}
	if fileaccess.ContentHash(current) != input.ExpectedHash {
		return models.FileContent{}, ErrStaleContent
	}
	info, err := os.Stat(full)
	if err != nil {
		return models.FileContent{}, err
	}
	temp, err := os.CreateTemp(filepath.Dir(full), ".plan-manager-*")
	if err != nil {
		return models.FileContent{}, err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(info.Mode().Perm()); err != nil {
		temp.Close()
		return models.FileContent{}, err
	}
	if _, err := temp.WriteString(input.Content); err != nil {
		temp.Close()
		return models.FileContent{}, err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return models.FileContent{}, err
	}
	if err := temp.Close(); err != nil {
		return models.FileContent{}, err
	}
	if err := os.Rename(tempName, full); err != nil {
		return models.FileContent{}, err
	}
	return fileaccess.ReadFileContent(clean, full)
}

func treeEntry(root, relPath string, entry os.DirEntry, ignored bool) (models.WorkspaceTreeEntry, bool) {
	_, full, err := resolve(root, relPath, entry.IsDir())
	if err != nil {
		return models.WorkspaceTreeEntry{}, false
	}
	info, err := os.Stat(full)
	if err != nil {
		return models.WorkspaceTreeEntry{}, false
	}
	node := models.WorkspaceTreeEntry{
		ID:      workspaceFileID(relPath),
		Name:    entry.Name(),
		Path:    relPath,
		Type:    "file",
		Ignored: ignored,
		Hidden:  strings.HasPrefix(entry.Name(), "."),
	}
	if info.IsDir() {
		node.Type = "directory"
		node.HasChildren = hasVisibleChild(full)
		return node, true
	}
	classification := fileaccess.ClassifyPath(relPath)
	node.Kind = classification.Kind
	node.Language = classification.Language
	node.SizeBytes = info.Size()
	node.Editable = fileaccess.IsEditableKind(classification.Kind)
	return node, true
}

func resolve(root, path string, wantDirectory bool) (string, string, error) {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if clean == "." {
		clean = ""
	}
	if filepath.IsAbs(path) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", "", fmt.Errorf("%w: %q", ErrInvalidPath, path)
	}
	if protected(clean) {
		return "", "", fmt.Errorf("%w: %q", ErrProtectedPath, path)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", "", err
	}
	full := realRoot
	if clean != "" {
		full = filepath.Join(realRoot, filepath.FromSlash(clean))
	}
	realPath, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", "", err
	}
	if realPath != realRoot && !strings.HasPrefix(realPath, realRoot+string(filepath.Separator)) {
		return "", "", fmt.Errorf("%w: %q", ErrOutsideRoot, path)
	}
	info, err := os.Stat(realPath)
	if err != nil {
		return "", "", err
	}
	if wantDirectory && !info.IsDir() {
		return "", "", fmt.Errorf("%w: path is not a directory", ErrInvalidPath)
	}
	if !wantDirectory && !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("%w: path is not a file", ErrInvalidPath)
	}
	return clean, realPath, nil
}

func protected(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.EqualFold(part, ".git") {
			return true
		}
	}
	return false
}

func joinRelative(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "/" + name
}

func hasVisibleChild(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), ".git") {
			return true
		}
	}
	return false
}

func workspaceFileID(path string) string {
	return strings.NewReplacer("/", "__", ".", "_").Replace(path)
}

type gitIgnoreChecker struct{}

func (gitIgnoreChecker) Ignored(workspaceRoot string, paths []string) (map[string]bool, error) {
	cmd := exec.Command("git", "check-ignore", "-z", "--stdin")
	cmd.Dir = workspaceRoot
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\x00") + "\x00")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("git check-ignore: %s", strings.TrimSpace(stderr.String()))
	}
	result := map[string]bool{}
	for _, path := range bytes.Split(stdout.Bytes(), []byte{0}) {
		if len(path) > 0 {
			result[string(path)] = true
		}
	}
	return result, nil
}

func naturalLess(left, right string) bool {
	lp, rp := naturalParts(left), naturalParts(right)
	for i := 0; i < len(lp) && i < len(rp); i++ {
		if lp[i].number && rp[i].number && lp[i].numberValue != rp[i].numberValue {
			return lp[i].numberValue < rp[i].numberValue
		}
		if lp[i].value != rp[i].value {
			return lp[i].value < rp[i].value
		}
	}
	return len(lp) < len(rp)
}

type naturalPart struct {
	value       string
	number      bool
	numberValue int
}

func naturalParts(input string) []naturalPart {
	var parts []naturalPart
	for i := 0; i < len(input); {
		start := i
		number := unicode.IsDigit(rune(input[i]))
		for i < len(input) && unicode.IsDigit(rune(input[i])) == number {
			i++
		}
		value := strings.ToLower(input[start:i])
		part := naturalPart{value: value, number: number}
		if number {
			part.numberValue, _ = strconv.Atoi(value)
		}
		parts = append(parts, part)
	}
	return parts
}
