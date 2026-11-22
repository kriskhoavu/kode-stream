package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (g *GitAdapter) Clone(remoteURL, destination string) error {
	_, err := g.CloneWithLog(remoteURL, destination)
	return err
}

func (g *GitAdapter) CloneWithLog(remoteURL, destination string) (string, error) {
	var builder strings.Builder
	err := g.CloneWithProgress(remoteURL, destination, func(chunk string) {
		builder.WriteString(chunk)
	})
	return strings.TrimSpace(builder.String()), err
}

func (g *GitAdapter) CloneWithProgress(remoteURL, destination string, onChunk func(string)) error {
	cleanRemote := strings.TrimSpace(remoteURL)
	cleanDestination := strings.TrimSpace(destination)
	if cleanRemote == "" {
		return fmt.Errorf("remote URL is required")
	}
	if cleanDestination == "" {
		return fmt.Errorf("clone destination is required")
	}
	parent := filepath.Dir(cleanDestination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	stdout := &limitedBuffer{limit: 4 << 20}
	stderr := &limitedBuffer{limit: 1 << 20}
	stream := &chunkStreamWriter{onChunk: onChunk}
	ctx, cancel := context.WithTimeout(context.Background(), g.cloneTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", "--progress", "--", cleanRemote, filepath.Base(cleanDestination))
	cmd.Dir = parent
	cmd.Stdout = io.MultiWriter(stdout, stream)
	cmd.Stderr = io.MultiWriter(stderr, stream)
	err := cmd.Run()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("git command timed out after %s", g.cloneTimeout)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func (g *GitAdapter) run(dir string, args ...string) (string, error) {
	return g.runIn(dir, args...)
}

func (g *GitAdapter) runIn(dir string, args ...string) (string, error) {
	return g.runWithTimeout(dir, g.timeout, args...)
}

func (g *GitAdapter) runWithTimeout(dir string, timeout time.Duration, args ...string) (string, error) {
	stdout, _, err := g.runWithTimeoutDetailed(dir, timeout, args...)
	if err != nil {
		return "", err
	}
	return stdout, nil
}

func (g *GitAdapter) runWithTimeoutDetailed(dir string, timeout time.Duration, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	stdout := &limitedBuffer{limit: 4 << 20}
	stderr := &limitedBuffer{limit: 1 << 20}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return stdout.String(), stderr.String(), fmt.Errorf("git command timed out after %s", timeout)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		if errors.Is(err, ErrOutputLimit) {
			return stdout.String(), stderr.String(), ErrOutputLimit
		}
		return stdout.String(), stderr.String(), fmt.Errorf("%s", msg)
	}
	return stdout.String(), stderr.String(), nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit int64
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if int64(b.Len()+len(p)) > b.limit {
		return 0, ErrOutputLimit
	}
	return b.Buffer.Write(p)
}

func (g *GitAdapter) runContextLimited(ctx context.Context, dir string, limit int64, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	stdout := &limitedBuffer{limit: limit}
	stderr := &limitedBuffer{limit: 1 << 20}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, ctx.Err()
		}
		if errors.Is(err, ErrOutputLimit) {
			return nil, ErrOutputLimit
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, errors.New(message)
	}
	return stdout.Bytes(), nil
}

type chunkStreamWriter struct {
	onChunk func(string)
}

func (w *chunkStreamWriter) Write(p []byte) (int, error) {
	if w.onChunk != nil && len(p) > 0 {
		chunk := strings.ReplaceAll(string(p), "\r", "\n")
		w.onChunk(chunk)
	}
	return len(p), nil
}
