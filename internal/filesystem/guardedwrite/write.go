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

var pathLocks sync.Map

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
	lockValue, _ := pathLocks.LoadOrStore(full, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	initial, err := os.Lstat(full)
	if err != nil {
		return Result{}, err
	}
	if !initial.Mode().IsRegular() || initial.Mode()&os.ModeSymlink != 0 {
		return Result{}, ErrUnsafePath
	}
	if initial.Size() > maxBytes {
		return Result{}, ErrTooLarge
	}
	file, err := os.Open(full)
	if err != nil {
		return Result{}, err
	}
	current, readErr := io.ReadAll(io.LimitReader(file, maxBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		return Result{}, readErr
	}
	if closeErr != nil {
		return Result{}, closeErr
	}
	if int64(len(current)) > maxBytes {
		return Result{}, ErrTooLarge
	}
	if hash(current) != expectedHash {
		return Result{}, ErrStale
	}

	directory := filepath.Dir(full)
	realDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return Result{}, err
	}
	temporary, err := os.CreateTemp(directory, ".kode-stream-write-*")
	if err != nil {
		return Result{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	fail := func(cause error) (Result, error) {
		_ = temporary.Close()
		return Result{}, cause
	}
	if err := temporary.Chmod(initial.Mode().Perm()); err != nil {
		return fail(err)
	}
	if _, err := temporary.Write(content); err != nil {
		return fail(err)
	}
	if hooks.afterWrite != nil {
		if err := hooks.afterWrite(); err != nil {
			return fail(err)
		}
	}
	if err := temporary.Sync(); err != nil {
		return fail(err)
	}
	if hooks.afterSync != nil {
		if err := hooks.afterSync(); err != nil {
			return fail(err)
		}
	}
	if err := temporary.Close(); err != nil {
		return Result{}, err
	}
	if hooks.afterClose != nil {
		if err := hooks.afterClose(); err != nil {
			return Result{}, err
		}
	}
	currentInfo, err := os.Lstat(full)
	if err != nil || !os.SameFile(initial, currentInfo) || currentInfo.Mode()&os.ModeSymlink != 0 {
		return Result{}, fmt.Errorf("%w: target changed during write", ErrUnsafePath)
	}
	currentDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil || currentDirectory != realDirectory {
		return Result{}, fmt.Errorf("%w: parent changed during write", ErrUnsafePath)
	}
	if err := os.Rename(temporaryPath, full); err != nil {
		return Result{}, err
	}
	rollback := func(cause error) (Result, error) {
		if restoreErr := restore(full, current, initial.Mode().Perm()); restoreErr != nil {
			return Result{}, fmt.Errorf("write failed: %v; original restoration failed: %w", cause, restoreErr)
		}
		return Result{}, cause
	}
	if hooks.afterRename != nil {
		if err := hooks.afterRename(); err != nil {
			return rollback(err)
		}
	}
	dir, err := os.Open(directory)
	if err != nil {
		return rollback(err)
	}
	if err := dir.Sync(); err != nil {
		_ = dir.Close()
		return rollback(err)
	}
	if hooks.afterDirectory != nil {
		if err := hooks.afterDirectory(); err != nil {
			_ = dir.Close()
			return rollback(err)
		}
	}
	if err := dir.Close(); err != nil {
		return rollback(err)
	}
	return Result{Path: full, Hash: hash(content), Mode: initial.Mode().Perm()}, nil
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
