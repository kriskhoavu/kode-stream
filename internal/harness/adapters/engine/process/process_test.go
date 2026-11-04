package process

import (
	"strings"
	"testing"
)

func TestLinesCutsOverlongLinesAndRingKeepsTheTail(t *testing.T) {
	var lines []string
	input := "short\n" + strings.Repeat("x", 100) + "\nlast"
	if err := Lines(strings.NewReader(input), 10, func(line []byte) { lines = append(lines, string(line)) }); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 || lines[1] != strings.Repeat("x", 10) || lines[2] != "last" {
		t.Fatalf("lines = %q", lines)
	}
	ring := NewRing(4)
	_, _ = ring.Write([]byte("abcdef"))
	if ring.String() != "cdef" {
		t.Fatalf("ring = %q", ring.String())
	}
	if got := Summary("a  b\nc", 10); got != "a b c" {
		t.Fatalf("summary = %q", got)
	}
	if got := Summary(strings.Repeat("é", 300), 200); len([]rune(got)) != 200 {
		t.Fatalf("summary length = %d", len([]rune(got)))
	}
	env := Environment(func(name string) string { return map[string]string{"PATH": "/bin", "SECRET": "x", "KEEP": "y"}[name] }, []string{"KEEP"}, map[string]string{"EXTRA": "z"})
	if strings.Join(env, ",") != "EXTRA=z,KEEP=y,PATH=/bin" {
		t.Fatalf("env = %v", env)
	}
}
