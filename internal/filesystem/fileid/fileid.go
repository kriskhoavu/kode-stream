// Package fileid owns the stable identity used for files in HTTP and UI contracts.
package fileid

import (
	"encoding/base64"
	"errors"
	"path"
	"path/filepath"
	"strings"
)

const prefix = "p_"

var ErrInvalid = errors.New("invalid file id")

// Validation happens at decode time, so a literal filename is never rewritten.
func Encode(value string) string {
	return prefix + base64.RawURLEncoding.EncodeToString([]byte(value))
}

// Legacy lossy IDs are rejected because they cannot be resolved safely.
func Decode(id string) (string, error) {
	if !strings.HasPrefix(id, prefix) {
		return "", ErrInvalid
	}
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, prefix))
	if err != nil {
		return "", ErrInvalid
	}
	value := string(data)
	if !validRelativePath(value) || Encode(value) != id {
		return "", ErrInvalid
	}
	return value, nil
}

func validRelativePath(value string) bool {
	if value == "" || strings.IndexByte(value, 0) >= 0 || filepath.IsAbs(value) || strings.HasPrefix(value, "/") {
		return false
	}
	// Rejects aliases such as a/../b, without rewriting whitespace, Unicode or backslashes.
	return path.Clean(value) == value && value != "." && value != ".." && !strings.HasPrefix(value, "../")
}
