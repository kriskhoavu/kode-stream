# Kode Stream

Kode Stream is a local, Git-native workspace for plans, Jira context, terminal and agent sessions, verification and
wiki knowledge. It turns Markdown plans and docs in Git into a workflow UI, and the files stay in Git.

## What It Does

- Registers local workspaces, or clones remote Git repositories.
- Indexes structured plans, configured sources and freestyle Markdown.
- Shows work on the Workstream board, with filters, saved views and quick search.
- Opens an item with its file tree, preview, Markdown editor, metadata, diff, Jira context and Git tools.
- Runs guarded Git actions: status, commit, fetch, pull, push and branch operations.
- Launches AI CLIs in an external terminal or an embedded one.
- Lists a workspace's agent sessions in **Agent Space**: interactive sessions a person drives in a terminal, and
  harness sessions the [engineering harness](wiki/platform/harness.md) runs headless.
- Arranges workspace, plan and session nodes on a branch-scoped **Canvas**.
- Runs verification jobs and tracks their status and artifacts.
- Indexes a wiki and its link graph in **Knowledge**.

## Deployment Modes

| Model                           | Runs on                                       | Workspace capability                                                  | Storage                                  |
|---------------------------------|-----------------------------------------------|-----------------------------------------------------------------------|------------------------------------------|
| Local application               | User machine, via Homebrew or a local binary  | Full local reads, writes, Git, terminal, AI, runtime and verification | `datadir` (default) or SQLite `database` |
| Cloud Agent-Backed              | Cloud VM or container plus an Agent on the user machine | Agent connection and metadata. Command delivery is not built  | Postgres `database`                      |
| Cloud Agentless Remote Snapshot | Cloud VM or container                         | Read-only provider reads pinned to a commit                           | Postgres `database`                      |

Read next:

- [Architecture](docs/architecture/overview.md) and [Development](docs/development.md).
- The [wiki index](wiki/index.md), which holds product behavior.
- [`deploy/`](deploy/README.md) for operator procedures.
- [AGENTS.md](AGENTS.md) for AI coding agents.

## Requirements

- Go 1.25+, Node.js 22, npm, Git.
- macOS: `osascript`, `open`. Windows: PowerShell, Explorer. Linux: `zenity` or `kdialog`, `xdg-open`.
- For AI sessions, an installed and signed-in supported CLI. Kode Stream never bypasses provider authentication,
  approval prompts or sandboxing.

## Quick Start

```bash
make up        # Local mode in Docker
make dev-up    # from source, in the background, no image rebuild
```

Open `http://localhost:4317`. `make down`, `make logs` and `make help` do what they say. `make dev-down`, `make dev-ps`
and `make dev-logs` manage the source build.

By hand:

```bash
npm install && npm run build
go build -o ./bin/kode-stream ./cmd/kode-stream
./bin/kode-stream serve -port 4317   # or set KODE_STREAM_PORT
```

Other stacks: `KODE_STREAM_STORAGE_OPTION=database make up` (SQLite), `make up STACK=cloud` (Cloud with an Agent) and
`KODE_STREAM_CLOUD_WORKSPACE_MODE=agentless make up STACK=cloud`. See the
[Local](deploy/docker/local/local-mode/README.md) and [Cloud](deploy/docker/local/cloud-mode/README.md) stacks.

## Install With Homebrew

```bash
brew tap kriskhoavu/homebrew-tap
brew install kode-stream
kode-stream serve -port 4317
```

`kode-stream doctor`, `brew test kode-stream` and `brew upgrade kode-stream` check and update it.

## CLI

```text
kode-stream serve [-port 4317]
kode-stream doctor [--provider github|bitbucket] [--repo <path-or-url>] [--format text|json] [--strict] [--port <n>]
kode-stream agent start|status|doctor
```

`serve` starts the server. `doctor` checks the environment and repository setup. `agent` runs the Cloud Agent.

## Verify

```bash
make verify   # npm run typecheck, npm test, go test ./..., go test -race ./..., npm run build
```

## Storage

Kode Stream keeps its own state outside managed repositories, in the OS data directory:

- macOS: `~/Library/Application Support/kode-stream/`
- Linux: `~/.config/kode-stream/`
- Windows: `%AppData%\kode-stream\`

`KODE_STREAM_DATA_DIR`, or `dataDir` in `bootstrap.yaml` in that directory, moves it. Options, files, environment
variables, sync and backup are in the [storage reference](wiki/platform/storage-reference.md).

## Workspace Files

Kode Stream reads configured source directories such as `plans`, `wiki` or `specs`.

- `plan.yaml`: item metadata such as `status`, `owner`, `tags` and title overrides.
- `README.md`: the item's main document and default title.
- `.kode-stream/workspace-settings.yaml`: optional mapping rules for sources with a non-standard layout. A rule belongs
  to the source its `fields.source` names, by path or last path segment. A source no rule names reads
  `workspace-settings.yaml` inside itself. `pathPattern` is relative to the source.
- `.kode-stream/settings.yaml`: optional workspace settings a team shares. Kode Stream writes it only on export and
  reads it only on import. See [workspace settings](wiki/domains/workspace/concepts/workspace-settings.md).

```yaml
# .kode-stream/workspace-settings.yaml
version: 1
cards:
  - pathPattern: "{folder}/feature/{item}"
    fields:
      source: wiki
      item: "{item}"
      title: readme_heading
      status: draft
      tags: [wiki]
```

```yaml
# .kode-stream/settings.yaml
version: 1
ai:
  provider: claude
harness:
  review: true
  tests:
    - go test ./...
  models:
    coder: { engine: claudecode, model: sonnet, hardModel: opus, maxTurns: 120 }
  procedure:
    planner: { skill: feature-planning, track: Plan }
  sources:
    - { kind: jira, enabled: true, query: "project = PM AND sprint in openSprints()" }
```

## Safety Model

- The server listens on loopback by default.
- Writes stay inside configured sources, `.kode-stream/workspace-settings.yaml` and, on export,
  `.kode-stream/settings.yaml`.
- Markdown saves need the expected content hash.
- File access rejects path traversal and symlink escapes.
- Commits stage only selected paths inside configured sources.
- Pull and branch switch guard against a dirty working tree.
- Kode Stream stores no credentials.
