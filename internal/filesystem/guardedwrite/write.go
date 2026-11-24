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

	"kode-stream/internal/filesystem/atomicfile"
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

func (h failureHooks) atomic() atomicfile.Hooks {
	return atomicfile.Hooks{AfterWrite: h.afterWrite, AfterSync: h.afterSync, AfterClose: h.afterClose, AfterRename: h.afterRename, AfterDirectorySync: h.afterDirectory}
}

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
	temporaryPath, err := atomicfile.WriteTemp(directory, ".kode-stream-write-*", initial.Mode().Perm(), content, hooks.atomic())
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
	if err := atomicfile.SyncRename(directory, hooks.atomic()); err != nil {
		if restoreErr := restore(full, current, initial.Mode().Perm()); restoreErr != nil {
			return Result{}, fmt.Errorf("write failed: %v; original restoration failed: %w", err, restoreErr)
		}
		return Result{}, err
	}
	return Result{Path: full, Hash: hash(content), Mode: initial.Mode().Perm()}, nil
}

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

func restore(target string, data []byte, mode os.FileMode) error {
	return atomicfile.Replace(target, ".kode-stream-restore-*", data, mode)
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
