package fileaccess

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"plan-manager/internal/models"
	"plan-manager/internal/security/pathguard"
)

type Access struct{}

func New() *Access {
	return &Access{}
}

func (a *Access) Tree(workspace models.WorkspaceConfig, item models.ItemDetail) ([]models.FileNode, error) {
	root, err := a.safeItemPath(workspace, item)
	if err != nil {
		return nil, err
	}
	return buildTreeFromDir(root, "")
}

func (a *Access) Read(workspace models.WorkspaceConfig, item models.ItemDetail, fileID string) (models.FileContent, error) {
	root, err := a.safeItemPath(workspace, item)
	if err != nil {
		return models.FileContent{}, err
	}
	relPath, full, err := a.resolveFile(workspace, item, root, fileID)
	if err != nil {
		return models.FileContent{}, err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return models.FileContent{}, err
	}
	return fileContent(relPath, data), nil
}

func (a *Access) WriteMarkdown(workspace models.WorkspaceConfig, item models.ItemDetail, input models.FileSaveInput) (models.FileContent, error) {
	root, err := a.safeItemPath(workspace, item)
	if err != nil {
		return models.FileContent{}, err
	}
	relPath, full, err := a.resolveFile(workspace, item, root, input.FileID)
	if err != nil {
		return models.FileContent{}, err
	}
	if language(relPath) != "markdown" {
		return models.FileContent{}, fmt.Errorf("only Markdown files can be edited")
	}
	current, err := os.ReadFile(full)
	if err != nil {
		return models.FileContent{}, err
	}
	if input.ExpectedHash != "" && input.ExpectedHash != contentHash(current) {
		return models.FileContent{}, fmt.Errorf("file content changed since it was loaded")
	}
	if err := os.WriteFile(full, []byte(input.Content), 0o644); err != nil {
		return models.FileContent{}, err
	}
	return fileContent(relPath, []byte(input.Content)), nil
}

func (a *Access) RelativePath(workspace models.WorkspaceConfig, item models.ItemDetail, fileID string) (string, error) {
	root, err := a.safeItemPath(workspace, item)
	if err != nil {
		return "", err
	}
	relPath, _, err := a.resolveFile(workspace, item, root, fileID)
	return relPath, err
}

func (a *Access) safeItemPath(workspace models.WorkspaceConfig, item models.ItemDetail) (string, error) {
	root, err := safeJoin(workspace.Path, item.ItemPath)
	if err != nil {
		return "", err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	allowed := false
	for _, dir := range workspace.Sources {
		allowedRoot, err := filepath.EvalSymlinks(filepath.Join(workspace.Path, filepath.FromSlash(dir)))
		if err != nil {
			continue
		}
		if realRoot == allowedRoot || strings.HasPrefix(realRoot, allowedRoot+string(filepath.Separator)) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("item path is outside configured sources")
	}
	return realRoot, nil
}

func (a *Access) resolveFile(workspace models.WorkspaceConfig, item models.ItemDetail, root, fileID string) (string, string, error) {
	relPath := ""
	for _, node := range flattenTreeMust(a.Tree(workspace, item)) {
		if node.ID == fileID {
			relPath = node.Path
			break
		}
	}
	if relPath == "" {
		for _, doc := range item.Documents {
			if fileIDForPath(doc.Path) == fileID {
				relPath = doc.Path
				break
			}
		}
	}
	if relPath == "" {
		return "", "", fmt.Errorf("file not found")
	}
	full, err := safeJoin(root, relPath)
	if err != nil {
		return "", "", err
	}
	realFile, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", "", err
	}
	if realFile != root && !strings.HasPrefix(realFile, root+string(filepath.Separator)) {
		return "", "", fmt.Errorf("file path escapes item root")
	}
	return relPath, realFile, nil
}

func safeJoin(root, rel string) (string, error) {
	return pathguard.SafeJoin(root, rel)
}

func flattenTreeMust(nodes []models.FileNode, _ error) []models.FileNode {
	var out []models.FileNode
	var walk func([]models.FileNode)
	walk = func(in []models.FileNode) {
		for _, node := range in {
			if node.Type == "file" {
				out = append(out, node)
			}
			walk(node.Children)
		}
	}
	walk(nodes)
	return out
}

func buildTreeFromDir(root, relDir string) ([]models.FileNode, error) {
	fullDir := root
	if relDir != "" {
		fullDir = filepath.Join(root, filepath.FromSlash(relDir))
	}
	entries, err := os.ReadDir(fullDir)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return naturalLess(entries[i].Name(), entries[j].Name())
	})

	nodes := make([]models.FileNode, 0, len(entries))
	for _, entry := range entries {
		path := filepath.ToSlash(filepath.Join(relDir, entry.Name()))
		node := models.FileNode{ID: fileIDForPath(path), Name: entry.Name(), Path: path, Type: "file"}
		if entry.IsDir() {
			children, err := buildTreeFromDir(root, path)
			if err != nil {
				return nil, err
			}
			node.Type = "directory"
			node.Children = children
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func naturalLess(left, right string) bool {
	leftParts := naturalParts(left)
	rightParts := naturalParts(right)
	for i := 0; i < len(leftParts) && i < len(rightParts); i++ {
		a, b := leftParts[i], rightParts[i]
		if a.number && b.number {
			if a.numberValue != b.numberValue {
				return a.numberValue < b.numberValue
			}
			continue
		}
		if a.value != b.value {
			return a.value < b.value
		}
	}
	return len(leftParts) < len(rightParts)
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
		r := rune(input[i])
		isNumber := unicode.IsDigit(r)
		for i < len(input) && unicode.IsDigit(rune(input[i])) == isNumber {
			i++
		}
		value := strings.ToLower(input[start:i])
		part := naturalPart{value: value, number: isNumber}
		if isNumber {
			part.numberValue, _ = strconv.Atoi(value)
		}
		parts = append(parts, part)
	}
	return parts
}

func fileIDForPath(path string) string {
	return strings.NewReplacer("/", "__", ".", "_").Replace(path)
}

func fileContent(relPath string, data []byte) models.FileContent {
	return models.FileContent{
		ID:       fileIDForPath(relPath),
		Path:     relPath,
		Content:  string(data),
		Language: language(relPath),
		Hash:     contentHash(data),
	}
}

func contentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
