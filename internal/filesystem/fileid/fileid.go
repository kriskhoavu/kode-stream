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

// Encode returns a collision-free, URL-path-safe identity preserving every
// byte of the supplied valid workspace-relative path. Validation belongs at
// decode/use time so a literal filename is never rewritten into another one.
func Encode(value string) string {
	return prefix + base64.RawURLEncoding.EncodeToString([]byte(value))
}

// Decode returns the exact canonical relative path carried by an ID. Legacy
// lossy IDs are deliberately rejected because they cannot be resolved safely.
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
	// Do not accept aliases such as a/../b. This validates without rewriting
	// whitespace, Unicode, or literal backslashes in a filename.
	return path.Clean(value) == value && value != "." && value != ".." && !strings.HasPrefix(value, "../")
}
