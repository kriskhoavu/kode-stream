package scanner

import (
	"os"
	"path/filepath"
)

func osMkdirAll(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

func osWriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func osReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}
