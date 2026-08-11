package scanner

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

type budgetReader struct {
	ctx     context.Context
	inner   SourceReader
	budget  ScanBudget
	mu      sync.Mutex
	entries int
	bytes   int64
	err     error
}

func newBudgetReader(ctx context.Context, inner SourceReader, budget ScanBudget) *budgetReader {
	return &budgetReader{ctx: ctx, inner: inner, budget: budget}
}

func (r *budgetReader) Err() error { r.mu.Lock(); defer r.mu.Unlock(); return r.err }
func (r *budgetReader) fail(err error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err == nil {
		r.err = err
	}
	return err
}
func (r *budgetReader) check() error {
	if err := r.ctx.Err(); err != nil {
		return r.fail(err)
	}
	if err := r.Err(); err != nil {
		return err
	}
	return nil
}

func (r *budgetReader) ReadDir(path string) ([]DirEntry, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	entries, err := r.inner.ReadDir(path)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.entries += len(entries)
	exceeded := r.entries > r.budget.MaxEntries
	r.mu.Unlock()
	if exceeded {
		return nil, r.fail(fmt.Errorf("%w: entry budget", ErrScanLimit))
	}
	if depth(path) > r.budget.MaxDepth {
		return nil, r.fail(fmt.Errorf("%w: depth budget", ErrScanLimit))
	}
	return entries, nil
}

func (r *budgetReader) ReadFile(path string) ([]byte, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	info, err := r.inner.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > r.budget.MaxFileBytes {
		return nil, r.fail(fmt.Errorf("%w: file %q exceeds byte budget", ErrScanLimit, path))
	}
	r.mu.Lock()
	r.bytes += info.Size()
	exceeded := r.bytes > r.budget.MaxTotalBytes
	r.mu.Unlock()
	if exceeded {
		return nil, r.fail(fmt.Errorf("%w: total byte budget", ErrScanLimit))
	}
	return r.inner.ReadFile(path)
}

func (r *budgetReader) WalkDir(root string, fn WalkFunc) error {
	if err := r.check(); err != nil {
		return err
	}
	return r.inner.WalkDir(root, func(path string, entry DirEntry, err error) error {
		if checkErr := r.check(); checkErr != nil {
			return checkErr
		}
		if depth(path)-depth(root) > r.budget.MaxDepth {
			return r.fail(fmt.Errorf("%w: depth budget", ErrScanLimit))
		}
		r.mu.Lock()
		r.entries++
		exceeded := r.entries > r.budget.MaxEntries
		r.mu.Unlock()
		if exceeded {
			return r.fail(fmt.Errorf("%w: entry budget", ErrScanLimit))
		}
		return fn(path, entry, err)
	})
}

func (r *budgetReader) Stat(path string) (FileInfo, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	return r.inner.Stat(path)
}

func depth(value string) int {
	clean := filepath.ToSlash(filepath.Clean(value))
	if clean == "." || clean == "" {
		return 0
	}
	return len(strings.Split(clean, "/"))
}

var _ SourceReader = (*budgetReader)(nil)
