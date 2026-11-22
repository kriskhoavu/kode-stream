// Package guardedwrite provides atomic optimistic replacement below a trusted root.
package guardedwrite

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"kode-stream/internal/filesystem/pathguard"
)

var (
	ErrHashRequired = errors.New("expected hash is required")
	ErrStale        = errors.New("file content changed since it was loaded")
	ErrTooLarge     = errors.New("file exceeds the editable size limit")
	ErrUnsafePath   = errors.New("file path is not safe for replacement")
)

type pathLock struct {
	mu    sync.Mutex
	users int
}

var pathLocks = struct {
	mu      sync.Mutex
	entries map[string]*pathLock
}{entries: make(map[string]*pathLock)}

func lockPath(path string) func() {
	pathLocks.mu.Lock()
	entry := pathLocks.entries[path]
	if entry == nil {
		entry = &pathLock{}
		pathLocks.entries[path] = entry
	}
	entry.users++
	pathLocks.mu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		pathLocks.mu.Lock()
		entry.users--
		if entry.users == 0 {
			delete(pathLocks.entries, path)
		}
		pathLocks.mu.Unlock()
	}
}

type Result struct {
	Path string
	Hash string
	Mode os.FileMode
}

type failureHooks struct {
	afterWrite     func() error
	afterSync      func() error
	afterClose     func() error
	afterRename    func() error
	afterDirectory func() error
}

// ReplaceExisting atomically replaces a regular file. It bounds the existing
// read before allocation, preserves permissions, fsyncs the file and parent,
// and revalidates the target immediately before publication.
func ReplaceExisting(root, relative string, content []byte, expectedHash string, maxBytes int64) (Result, error) {
	return replaceExisting(root, relative, content, expectedHash, maxBytes, failureHooks{})
}

func replaceExisting(root, relative string, content []byte, expectedHash string, maxBytes int64, hooks failureHooks) (Result, error) {
	if strings.TrimSpace(expectedHash) == "" {
		return Result{}, ErrHashRequired
	}
	if int64(len(content)) > maxBytes {
		return Result{}, ErrTooLarge
	}
	full, err := pathguard.SafeJoin(root, relative)
	if err != nil {
		return Result{}, err
	}
	defer lockPath(full)()
	initial, current, err := readExisting(full, maxBytes)
	if err != nil {
		return Result{}, err
	}
	if hash(current) != expectedHash {
		return Result{}, ErrStale
	}

	directory := filepath.Dir(full)
	realDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return Result{}, err
	}
	temporaryPath, err := writeTemporary(directory, initial.Mode().Perm(), content, hooks)
	if temporaryPath != "" {
		defer os.Remove(temporaryPath)
	}
	if err != nil {
		return Result{}, err
	}
	if err := requireUnchanged(full, initial, directory, realDirectory); err != nil {
		return Result{}, err
	}
	if err := os.Rename(temporaryPath, full); err != nil {
		return Result{}, err
	}
	if err := publishRename(directory, hooks); err != nil {
		if restoreErr := restore(full, current, initial.Mode().Perm()); restoreErr != nil {
			return Result{}, fmt.Errorf("write failed: %v; original restoration failed: %w", err, restoreErr)
		}
		return Result{}, err
	}
	return Result{Path: full, Hash: hash(content), Mode: initial.Mode().Perm()}, nil
}

// runHook calls a failure hook when one is set.
func runHook(hook func() error) error {
	if hook == nil {
		return nil
	}
	return hook()
}

// readExisting reads a regular, non-symlink file no larger than maxBytes,
// bounding the read before it allocates.
func readExisting(full string, maxBytes int64) (os.FileInfo, []byte, error) {
	initial, err := os.Lstat(full)
	if err != nil {
		return nil, nil, err
	}
	if !initial.Mode().IsRegular() || initial.Mode()&os.ModeSymlink != 0 {
		return nil, nil, ErrUnsafePath
	}
	if initial.Size() > maxBytes {
		return nil, nil, ErrTooLarge
	}
	file, err := os.Open(full)
	if err != nil {
		return nil, nil, err
	}
	current, readErr := io.ReadAll(io.LimitReader(file, maxBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, nil, readErr
	}
	if closeErr != nil {
		return nil, nil, closeErr
	}
	if int64(len(current)) > maxBytes {
		return nil, nil, ErrTooLarge
	}
	return initial, current, nil
}

// writeTemporary writes content to a synced, closed temporary file beside
// the target. It returns the temporary path whenever one was created, so the
// caller removes it on every path.
func writeTemporary(directory string, mode os.FileMode, content []byte, hooks failureHooks) (string, error) {
	temporary, err := os.CreateTemp(directory, ".kode-stream-write-*")
	if err != nil {
		return "", err
	}
	path := temporary.Name()
	steps := []func() error{
		func() error { return temporary.Chmod(mode) },
		func() error { _, err := temporary.Write(content); return err },
		func() error { return runHook(hooks.afterWrite) },
		temporary.Sync,
		func() error { return runHook(hooks.afterSync) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			_ = temporary.Close()
			return path, err
		}
	}
	if err := temporary.Close(); err != nil {
		return path, err
	}
	return path, runHook(hooks.afterClose)
}

// requireUnchanged refuses publication when the target or its parent changed
// since the target was read.
func requireUnchanged(full string, initial os.FileInfo, directory, realDirectory string) error {
	currentInfo, err := os.Lstat(full)
	if err != nil || !os.SameFile(initial, currentInfo) || currentInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: target changed during write", ErrUnsafePath)
	}
	currentDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil || currentDirectory != realDirectory {
		return fmt.Errorf("%w: parent changed during write", ErrUnsafePath)
	}
	return nil
}

// publishRename makes a completed rename durable by syncing the parent
// directory. An error means the caller restores the original content.
func publishRename(directory string, hooks failureHooks) error {
	if err := runHook(hooks.afterRename); err != nil {
		return err
	}
	dir, err := os.Open(directory)
	if err != nil {
		return err
	}
	if err := dir.Sync(); err != nil {
		_ = dir.Close()
		return err
	}
	if err := runHook(hooks.afterDirectory); err != nil {
		_ = dir.Close()
		return err
	}
	return dir.Close()
}

func restore(target string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(target)
	temporary, err := os.CreateTemp(directory, ".kode-stream-restore-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return err
	}
	dir, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
