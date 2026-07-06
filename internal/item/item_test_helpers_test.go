package item

// Shared Item domain test helpers.

import "os"

func osMkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}

func osWriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
