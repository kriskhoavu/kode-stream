// Package process starts a coding CLI in its own process group and stops the
// whole group: SIGTERM first, SIGKILL five seconds later.
package process

import (
	"bufio"
	"io"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

// GracePeriod is how long a CLI may take to exit after SIGTERM.
const GracePeriod = 5 * time.Second

// Environment builds a child environment from PATH, HOME and an allow-list,
// never the server's whole environment.
func Environment(getenv func(string) string, allowed []string, extra map[string]string) []string {
	keys := map[string]string{}
	for _, name := range append([]string{"PATH", "HOME", "USER", "LANG", "TMPDIR"}, allowed...) {
		if value := getenv(name); value != "" {
			keys[name] = value
		}
	}
	for name, value := range extra {
		keys[name] = value
	}
	names := make([]string, 0, len(keys))
	for name := range keys {
		names = append(names, name)
	}
	sort.Strings(names)
	env := make([]string, 0, len(names))
	for _, name := range names {
		env = append(env, name+"="+keys[name])
	}
	return env
}

// Stop terminates cmd's process group once, whatever calls it.
type Stop struct {
	once sync.Once
	cmd  *exec.Cmd
	done <-chan struct{}
}

func NewStop(cmd *exec.Cmd, done <-chan struct{}) *Stop { return &Stop{cmd: cmd, done: done} }

func (s *Stop) Terminate() {
	s.once.Do(func() {
		if s.cmd.Process == nil {
			return
		}
		_ = signalGroup(s.cmd.Process.Pid, false)
		select {
		case <-s.done:
		case <-time.After(GracePeriod):
			_ = signalGroup(s.cmd.Process.Pid, true)
		}
	})
}

// Ring keeps the last limit bytes written to it.
type Ring struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func NewRing(limit int) *Ring { return &Ring{limit: limit} }

func (r *Ring) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = append(r.data, p...)
	if len(r.data) > r.limit {
		r.data = append([]byte{}, r.data[len(r.data)-r.limit:]...)
	}
	return len(p), nil
}

func (r *Ring) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.data)
}

// Lines calls visit with each line of r, each cut to max bytes; the rest of
// an over-long line is skipped rather than failing the stream.
func Lines(r io.Reader, max int, visit func([]byte)) error {
	reader := bufio.NewReaderSize(r, 64<<10)
	var line []byte
	for {
		chunk, isPrefix, err := reader.ReadLine()
		if len(chunk) > 0 && len(line) < max {
			room := max - len(line)
			if len(chunk) > room {
				chunk = chunk[:room]
			}
			line = append(line, chunk...)
		}
		if err != nil {
			if len(line) > 0 {
				visit(line)
			}
			if err == io.EOF {
				return nil
			}
			return err
		}
		if !isPrefix {
			visit(line)
			line = line[:0]
		}
	}
}

// Summary returns at most limit characters of text on one line.
func Summary(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) > limit {
		return string(runes[:limit-1]) + "…"
	}
	return text
}
