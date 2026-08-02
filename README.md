# Kode Stream

Kode Stream is a local, Git-native workspace for planning documents, Jira context, terminal sessions, verification
harnesses, and LLM Wiki & Graph workflows. It turns Markdown-based plans, specs, and docs into a focused workflow UI
while keeping the files in Git.

The app is built for engineering teams that keep work plans close to code and need faster navigation, safer edits, and
clearer local Git operations.

## What It Does

- Registers local workspaces or clones remote Git repositories.
- Indexes structured plans, configured document sources, and freestyle Markdown docs.
- Shows indexed work in a Workstream board with filtering, saved views, and quick search.
- Opens each item in a workspace view with file tree, preview, Markdown editor, metadata, diff, Jira context, and Git tools.
- Provides a global Workstream Explorer for browsing, searching, creating, renaming, editing, and reviewing workspace files.
- Runs guarded Git actions for status, commit, fetch, pull, push, and branch operations.
- Connects workspace items with Jira issue context and guarded attachment access.
- Launches external or embedded terminal sessions with supported local AI CLIs.
- Runs verification harness jobs and tracks their status and artifacts.
- Indexes LLM Wiki content and graph relationships for structured knowledge workflows.
- Stores app registry, cache, audit log, filters, recents, and AI settings outside managed repositories.

## Deployment Modes

Kode Stream has three supported deployment models. Storage is a separate choice only for Local.

| Model                           | Install/run location                          | Workspace capability                                                                            | Storage                                  |
|---------------------------------|-----------------------------------------------|-------------------------------------------------------------------------------------------------|------------------------------------------|
| Local application               | User machine; Homebrew or local binary        | Full local reads, writes, Git, terminal, AI, runtime, and verification                          | `datadir` (default) or SQLite `database` |
| Cloud Agent-Backed              | Cloud VM/container plus Agent on user machine | Cloud coordinates; Agent executes privileged actions locally                                    | Postgres `database`                      |
| Cloud Agentless Remote Snapshot | Cloud VM/container                            | Read-only provider snapshot pinned to a commit; no Agent, checkout, or hosted command execution | Postgres `database`                      |

```text
Local: Browser -> loopback Kode Stream -> local repository
Cloud + Agent: Browser -> Cloud API -> outbound Agent -> local repository
Cloud snapshot: Browser -> Cloud API -> provider API -> immutable repository commit
```

Remote Snapshot currently provides the commit-pinned backend foundation for metadata, tree, and file reads. Its
self-service registration and snapshot-backed board/search UI remain planned work.

See [Architecture](docs/architecture/ARCHITECTURE.md) for the capability boundary, [Cloud modes](docs/cloud/cloud-modes.md) for operating
guidance, [Storage](docs/storage/storage-architecture.md) for the storage decision matrix, and the
[Documentation map](docs/README.md) for the full documentation taxonomy.

## Tech Stack

| Area        | Technology                                          |
|-------------|-----------------------------------------------------|
| Backend     | Go 1.25 module, Go 1.22+ source patterns            |
| API         | Gin 1.9 + Go `net/http`                             |
| Frontend    | React 19, TypeScript 5, Vite 6                      |
| Testing     | Vitest, React Testing Library, `go test`            |
| Rendering   | Unified, KaTeX, highlight.js, YAML                  |
| Packaging   | Go binary with embedded frontend assets             |
| Persistence | Local SQLite or YAML/JSONL data-dir, Cloud Postgres |

## Requirements

- Go `1.25+`
- Node.js `20+`
- npm
- Git

Platform integrations:

- macOS: `osascript`, `open`
- Windows: PowerShell, Explorer
- Linux: `zenity` or `kdialog`, `xdg-open`

AI session launch requires an installed and authenticated supported CLI. Kode Stream does not bypass provider
authentication, approval prompts, or sandbox behavior.

## Quick Start

```bash
npm install
npm run build
go build -o ./bin/kode-stream ./cmd/kode-stream
./bin/kode-stream serve -port 4317
```

Open `http://localhost:4317`.

The default port is `4317`. You can also set it with `KODE_STREAM_PORT`:

```bash
KODE_STREAM_PORT=4317 ./bin/kode-stream serve
```

## Install With Homebrew

macOS users can install Kode Stream from the public tap:

```bash
brew update
brew tap kriskhoavu/homebrew-tap
brew install kode-stream
kode-stream serve -port 4317
```

Open `http://localhost:4317`.

Useful commands:

```bash
kode-stream doctor
brew test kode-stream
brew upgrade kode-stream
```

## Development

```bash
npm run typecheck
npm test -- --run
go test ./...
```

Build the production assets and local binary:

```bash
npm run build
go build -o ./bin/kode-stream ./cmd/kode-stream
```

Run frontend development server:

```bash
npm run dev
```

## CLI

```text
kode-stream serve [-port 4317]
kode-stream doctor [--provider github|bitbucket] [--repo <path-or-url>] [--format text|json] [--strict] [--port <n>]
kode-stream agent start|status|doctor
```

- `serve`: starts the local app server.
- `doctor`: checks the environment and repository setup.
- `agent`: starts, checks, or diagnoses the Cloud Agent command surface.

For a local Agent-Backed Cloud smoke stack with Docker, Postgres, Keycloak, OAuth2Proxy, and a foreground Cloud Agent:

```bash
./runbooks/docker/cloud-mode/run.sh
```

For the Agentless Remote Snapshot control-plane stack, use:

```bash
KODE_STREAM_CLOUD_WORKSPACE_MODE=agentless ./runbooks/docker/cloud-mode/run.sh
```

See [Local Cloud Stack](runbooks/docker/cloud-mode/README.md) for both flows.

## Local Docker Mode

Local mode can also run in Docker with either supported Local storage option:

```bash
./runbooks/docker/local-mode/run.sh
KODE_STREAM_STORAGE_OPTION=database ./runbooks/docker/local-mode/run.sh
```

The selected host workspace is mounted at `/workspace`. See [Local Docker Stack](runbooks/docker/local-mode/README.md) for the
storage boundary and container limitations for Git credentials, terminal, AI, dialogs, and path reveal.

## Storage And Data Directory

Kode Stream stores app-owned state outside managed repositories. Local mode supports `datadir` and `database` storage
options. Local `datadir` is the default and uses YAML/JSONL files under the OS user config directory. Local `database`
uses SQLite in the same directory. Cloud mode requires `database` with Postgres through `KODE_STREAM_DATABASE_URL`.

Typical defaults:

- macOS: `~/Library/Application Support/kode-stream/`
- Linux: `~/.config/kode-stream/`
- Windows: `%AppData%\kode-stream\`

Startup resolution order:

1. `KODE_STREAM_DATA_DIR`
2. `bootstrap.yaml` in the default OS data directory
3. Default OS data directory

Example `bootstrap.yaml`:

```yaml
dataDir: /Users/me/.kode-stream-data
storageOption: database
```

Changing `dataDir` or `storageOption` requires a restart.

Main files:

```text
<effective-data-dir>/
  bootstrap.yaml
  kode-stream.db        # local database option
  workspaces.yaml       # local datadir option
  item-index.yaml       # local datadir option
  audit-log.jsonl       # local datadir option
  saved-filters.yaml    # local datadir option
  recent-items.yaml     # local datadir option
  ai-settings.yaml      # local datadir option
  backups/storage-sync/
  clone-root/
```

Storage configuration:

| Variable                     | Local default                         | Cloud requirement                   |
|------------------------------|---------------------------------------|-------------------------------------|
| `KODE_STREAM_STORAGE_OPTION` | `datadir`                             | `database`                          |
| `KODE_STREAM_STORAGE_DRIVER` | derived from option                   | `postgres`                          |
| `KODE_STREAM_SQLITE_PATH`    | `<effective-data-dir>/kode-stream.db` | unused                              |
| `KODE_STREAM_DATABASE_URL`   | unused                                | secret-managed Postgres URL         |
| `KODE_STREAM_MIGRATIONS`     | `auto`                                | `auto` or operator-managed `manual` |

Local examples:

```bash
KODE_STREAM_STORAGE_OPTION=database ./run.sh restart
KODE_STREAM_STORAGE_OPTION=datadir ./run.sh restart
./run.sh smoke-storage
```

Settings can manually sync `datadir -> database` or `database -> datadir`. Each sync creates a target backup under
`backups/storage-sync/`. Runtime writes go only to the selected storage option.

See [Storage](docs/storage/storage-architecture.md) for supported storage options, performance comparison, backup,
restore, manual sync, and Cloud Postgres operations.

## Workspace Files

Kode Stream reads configured source directories such as `plans`, `wiki`, or `specs`.

Common workspace files:

- `workspace-settings.yaml`: optional mapping rules for non-standard source layouts.
- `plan.yaml`: item metadata such as `status`, `owner`, `tags`, and optional title overrides.
- `README.md`: primary item document and default title source.

Example `workspace-settings.yaml`:

```yaml
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

## Safety Model

- Server access is local by default.
- Writes are limited to configured workspace sources.
- Markdown saves use expected content hashes to prevent stale overwrites.
- File access rejects path traversal and symlink escapes.
- Git commits stage only user-selected paths inside configured sources.
- Pull and branch switch guard against dirty working trees.
- Credentials are not stored by Kode Stream.

## Architecture

See [Architecture](docs/architecture/ARCHITECTURE.md) for system boundaries, storage design, data flow, and API structure.

For hosted deployment, see [Cloud Deployment](docs/cloud/cloud-deployment.md).
