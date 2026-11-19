#!/usr/bin/env bash
# Builds a small sandbox for trying the harness's pipeline end to end without
# an issue tracker or a pull request:
#
#   <dir>/repo        the workspace: a tiny Go module, origin = remote.git
#   <dir>/remote.git  a local bare remote; hand-off pushes feature/* here
#   <dir>/tasks       the local task folder; one Markdown file per task
#
# Usage:
#   scripts/harness-sandbox.sh init [dir]   create the sandbox and SANDBOX-1
#   scripts/harness-sandbox.sh next [dir]   reset the sandbox, add the next task
#   scripts/harness-sandbox.sh status [dir] print the paths and the settings
#
# dir defaults to $HARNESS_SANDBOX, else ~/kode-stream-sandbox.
set -euo pipefail

command="${1:-}"
dir="${2:-${HARNESS_SANDBOX:-$HOME/kode-stream-sandbox}}"
repo="$dir/repo"
remote="$dir/remote.git"
tasks="$dir/tasks"

usage() {
	sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//'
	exit "${1:-0}"
}

# The task every round asks for. Small enough for one plan and one build,
# with a test the Test step runs and room to ask for a change at Review.
write_task() {
	local key="$1"
	cat >"$tasks/$key.md" <<EOF
# Add a Goodbye function

The \`greet\` package says hello. Add the matching goodbye.

## Acceptance criteria

- \`greet.Goodbye(name string) string\` returns \`Goodbye, <name>!\`.
- An empty or blank name returns \`Goodbye, friend!\`, as \`Hello\` does.
- A table test in \`greet/greet_test.go\` covers both cases.
- \`go test ./...\` passes. No file outside \`greet/\` and \`plans/\` changes.
EOF
	echo "Added task $key: $tasks/$key.md"
}

next_key() {
	local highest=0 file number
	for file in "$tasks"/SANDBOX-*.md; do
		[ -e "$file" ] || continue
		number="${file##*/SANDBOX-}"
		number="${number%.md}"
		[[ "$number" =~ ^[0-9]+$ ]] && [ "$number" -gt "$highest" ] && highest="$number"
	done
	echo "SANDBOX-$((highest + 1))"
}

init() {
	if [ -e "$repo" ] || [ -e "$remote" ]; then
		echo "The sandbox already exists at $dir. Use 'next' to reset it." >&2
		exit 1
	fi
	mkdir -p "$repo/greet" "$repo/plans" "$tasks"
	git init --quiet --bare --initial-branch=main "$remote"
	git -C "$repo" init --quiet --initial-branch=main
	cat >"$repo/go.mod" <<'EOF'
module sandbox

go 1.22
EOF
	cat >"$repo/greet/greet.go" <<'EOF'
// Package greet says hello.
package greet

import "strings"

// Hello greets name, or a friend when name is blank.
func Hello(name string) string {
	if strings.TrimSpace(name) == "" {
		name = "friend"
	}
	return "Hello, " + name + "!"
}
EOF
	cat >"$repo/greet/greet_test.go" <<'EOF'
package greet

import "testing"

func TestHello(t *testing.T) {
	for _, test := range []struct{ name, want string }{
		{"Ada", "Hello, Ada!"},
		{" ", "Hello, friend!"},
	} {
		if got := Hello(test.name); got != test.want {
			t.Errorf("Hello(%q) = %q, want %q", test.name, got, test.want)
		}
	}
}
EOF
	cat >"$repo/AGENTS.md" <<'EOF'
# Sandbox

A tiny Go module for trying the Kode Stream harness. Code lives in `greet/`,
plans in `plans/`. Run `go test ./...` before you finish.
EOF
	cat >"$repo/plans/README.md" <<'EOF'
# Plans

The harness writes each task's plan here, under its key.
EOF
	git -C "$repo" add -A
	git -C "$repo" -c user.name=Sandbox -c user.email=sandbox@localhost commit --quiet -m "Start the sandbox"
	git -C "$repo" remote add origin "$remote"
	git -C "$repo" push --quiet -u origin main
	write_task SANDBOX-1
	status
}

# next puts the checkout back on the first commit, drops every branch a run
# left, and adds a fresh task: a finished task never reopens, so each try
# needs a new key.
next() {
	[ -d "$repo/.git" ] || { echo "No sandbox at $dir. Run 'init' first." >&2; exit 1; }
	git -C "$repo" worktree prune
	git -C "$repo" checkout --quiet main
	git -C "$repo" fetch --quiet --prune origin
	git -C "$repo" reset --quiet --hard origin/main
	git -C "$repo" clean --quiet -fd
	local branch
	for branch in $(git -C "$repo" for-each-ref --format='%(refname:short)' refs/heads/harness/); do
		git -C "$repo" branch --quiet -D "$branch" 2>/dev/null || echo "Kept $branch: a run's worktree still uses it." >&2
	done
	for branch in $(git -C "$remote" for-each-ref --format='%(refname:short)' refs/heads/feature/); do
		git -C "$remote" branch --quiet -D "$branch"
	done
	write_task "$(next_key)"
}

status() {
	cat <<EOF

Sandbox: $dir
Tasks:   $(ls "$tasks" 2>/dev/null | tr '\n' ' ')
Pushed:  $(git -C "$remote" for-each-ref --format='%(refname:short)' refs/heads/feature/ 2>/dev/null | tr '\n' ' ')

Once in Kode Stream:
  1. Add a workspace at $repo.
  2. In Harness settings, add a source: Local folder, for that workspace.
     Task folder: $tasks
     Test command: go test ./...
  3. Save. The task shows in the Task Queue after the next look.
EOF
}

case "$command" in
	init) init ;;
	next) next ;;
	status) status ;;
	-h | --help | help) usage ;;
	*) usage 1 ;;
esac
