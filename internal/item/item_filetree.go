package item

import (
	"path/filepath"
	"sort"
	"strings"

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/fileid"
)

func insertFileNode(nodes []models.FileNode, relPath string) []models.FileNode {
	clean := strings.Trim(filepath.ToSlash(relPath), "/")
	if clean == "" {
		return nodes
	}
	return insertFileNodeWithPrefix(nodes, clean, "")
}

func insertFileNodeWithPrefix(nodes []models.FileNode, relPath, prefix string) []models.FileNode {
	parts := strings.Split(relPath, "/")
	name := parts[0]
	currentPath := name
	if prefix != "" {
		currentPath = filepath.ToSlash(filepath.Join(prefix, name))
	}
	if len(parts) == 1 {
		return append(nodes, models.FileNode{ID: fileid.Encode(currentPath), Name: name, Path: currentPath, Type: "file"})
	}
	remaining := strings.Join(parts[1:], "/")
	for i := range nodes {
		if nodes[i].Type == "directory" && nodes[i].Name == name {
			nodes[i].Children = insertFileNodeWithPrefix(nodes[i].Children, remaining, currentPath)
			return nodes
		}
	}
	children := insertFileNodeWithPrefix(nil, remaining, currentPath)
	return append(nodes, models.FileNode{ID: fileid.Encode(currentPath), Name: name, Path: currentPath, Type: "directory", Children: children})
}

func sortFileNodes(nodes []models.FileNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type == "directory"
		}
		return nodes[i].Name < nodes[j].Name
	})
	for i := range nodes {
		sortFileNodes(nodes[i].Children)
	}
}

func flattenFileNodes(nodes []models.FileNode) []models.FileNode {
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

func fileIDToRelativePath(item models.ItemDetail, fileID string) string {
	path, err := fileid.Decode(fileID)
	if err != nil {
		return ""
	}
	for _, doc := range item.Documents {
		if filepath.ToSlash(filepath.Clean(doc.Path)) == path {
			return path
		}
	}
	return ""
}
