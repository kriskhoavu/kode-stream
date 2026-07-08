# Kode Stream

Kode Stream is a local, Git-native web application for browsing and editing planning documents across repositories and
workspaces. It turns file-based plans into a workflow-oriented UI without moving content out of Git.

It is designed for engineers who want faster visibility, safer edits, and cleaner Git operations when plans and specs
are stored as Markdown.

## Why Kode Stream

Teams often keep plans in Git because it gives strong review, ownership, and history. The tradeoff is discoverability
and flow: progress is hard to track across folders, branches, and multiple repositories.

Kode Stream addresses that gap by providing:

- A Workstream with a board view over document-backed items
- Multi-workspace and multi-source support (`plans`, `docs`, `specs`, etc.)
- One place to edit Markdown, metadata, status, and related files
- Built-in Git actions with guardrails for risky operations
- App-owned index and state outside managed repositories

## Feature Highlights

- Register workspaces from local paths or remote Git URLs (HTTPS/SSH)
- Import selected entries from a current-schema `workspaces.yaml` after validation and review
- Configure one or more sources per workspace
- Index structured items, configured docs, and freestyle docs in one board
- Keep unmatched docs in an `Unsorted` lane until mapping rules are added
- Load branch-scoped Workstream snapshots without changing the user's Git checkout
- Filter Workstream items by source, status, author, branch, and free text
- Open item workspaces with file tree, rich preview, markdown editor, metadata, and diff
- Autosave Markdown edits with stale-write protection
- Edit item metadata and move status from either board or metadata form
- Create new structured items
- Search indexed items across one or all workspaces with keyboard navigation
- Save and restore Workstream filter views
- Reopen recently viewed items quickly
- Use guarded Git flows for commit, fetch, pull, push, branch create/switch
- Inspect workspace health and recent operation history
- Detect local Claude, Codex, Copilot, and OpenCode CLIs
- Launch Terminal, iTerm2, or WezTerm with workspace-only or selected-card context
- Connect a workspace to Jira Cloud or Jira Server/Data Center through REST APIs
- View matching Jira tickets and safely open attachments from an item

See implementation details in [plans/platform/PM-002/README.md](plans/platform/PM-002/README.md).

## Tech Stack

| Area              | Technology                               | Purpose                                            |
|-------------------|------------------------------------------|----------------------------------------------------|
| Backend           | Go 1.22                                  | Local HTTP API, filesystem access, Git integration |
| Frontend          | React 19 + TypeScript 5                  | UI shell, Workstream, Explorer, item workspace     |
| Build             | Vite 6                                   | Frontend build and dev tooling                     |
| Testing           | Vitest, React Testing Library, `go test` | UI and backend validation                          |
| Content Rendering | Unified, KaTeX, highlight.js, YAML       | Safe rich preview for multiple file types          |
| Persistence       | YAML files in user config directory      | Workspace registry and index cache                 |
| Distribution      | Go binary with embedded frontend assets  | Single local runtime                               |

## Requirements

- Go `1.22+`
- Node.js `20+`
- npm
- Git

Platform-specific tools used for native folder/file selection and path reveal:

- macOS: `osascript`, `open`
- Windows: PowerShell, Explorer
- Linux: `zenity` or `kdialog` (picker), `xdg-open` (reveal)

External AI session launch currently supports macOS Terminal, iTerm2, and WezTerm. Install and authenticate at least one supported AI CLI separately. Kode Stream does not bypass the CLI's permission prompts or sandbox.

Embedded AI sessions are also available on macOS and supported Unix hosts. They run the selected provider in an app-owned terminal with explicit cancel, reconnect, and exit state. External launch remains the fallback on unsupported platforms or when users prefer a native terminal.

The terminal dock supports multiple concurrent sessions across workspaces. Minimize it into a compact bottom-right restore chip while reading and editing plans, switch sessions from workspace-labeled tabs after restoration, or maximize the active terminal for full-screen work.

## Quick Start

```bash
npm install
npm run build
go build -o ./bin/kode-stream ./cmd/kode-stream
./bin/kode-stream serve -port 4317
```

Open `http://127.0.0.1:4317`.

Default port is `4317`. You can also set `KODE_STREAM_PORT`:

```bash
KODE_STREAM_PORT=4317 ./bin/kode-stream serve
```

## Install With Homebrew (macOS)

For macOS users, Kode Stream can be installed from the public tap:

```bash
brew update
brew tap kriskhoavu/homebrew-tap
brew install kode-stream
```

Run the app:

```bash
kode-stream serve -port 4317
```

Open `http://127.0.0.1:4317` in your browser.

Run in the background (optional):

```bash
nohup kode-stream serve -port 4317 >/dev/null 2>&1 &
```

Stop the app:

```bash
pkill -f "kode-stream serve"
```

If running in the foreground, press `Ctrl+C` in the same terminal.

Validate the installed formula:

```bash
kode-stream doctor
brew test kode-stream
```

Notes:

- Homebrew formula support is currently macOS only.
- Use `brew upgrade kode-stream` for updates.

## Development

```bash
npm run typecheck
npm test -- --run
go test ./...
```

Useful build commands:

```bash
npm run build
go build -o ./bin/kode-stream ./cmd/kode-stream
```

## CLI Commands

```text
kode-stream serve [-port 4317]
kode-stream doctor [--provider github|bitbucket] [--repo <path-or-url>] [--format text|json] [--strict] [--port <n>]
```

- `serve`: starts the local app server (binds to `127.0.0.1`)
- `doctor`: runs environment and repository checks for troubleshooting and setup validation

## Data Directory and Settings

Kode Stream stores app-owned state in a user-level data directory (resolved via `os.UserConfigDir()`).

Typical defaults:

- macOS: `~/Library/Application Support/kode-stream/`
- Linux: `~/.config/kode-stream/`
- Windows: `%AppData%\kode-stream\`

Resolution order at startup:

1. `KODE_STREAM_DATA_DIR` environment variable
2. `bootstrap.yaml` override in the default OS data directory
3. Default OS data directory

When changed from the UI, override is written to:

```text
<default-os-data-dir>/bootstrap.yaml
```

Example:

```yaml
dataDir: /Users/me/.kode-stream-data
```

Changing `dataDir` requires a restart.

AI settings contain executable paths and argument templates only. Workspace-only sessions open at the workspace root without generated context, allowing manual file and directory references. Selected-card sessions pass the card's workspace-relative path directly to the AI, which can read relevant documents from that directory before waiting for the user's request. No context file or directory is created.

Embedded terminal grants remain in browser memory and expire quickly. Closing or navigating away from a running session requires confirmation; cancellation, disconnect timeout, and Kode Stream shutdown clean up the provider process.

Jira token lookup checks the running process environment first and then falls back to `~/.creds.zsh` and `~/.creds.sh` if the environment variable is missing. The file must contain an exported variable such as `export CC_JIRA_API_TOKEN="..."`.

### Data Directory Structure

```text
<effective-data-dir>/
  bootstrap.yaml
  workspaces.yaml
  item-index.yaml
  audit-log.jsonl
  saved-filters.yaml
  recent-items.yaml
  ai-settings.yaml
  clone-root/
```

### Workspace-Level Files

- `workspace-settings.yaml`: source mapping rules for non-standard docs layouts
- `plan.yaml`: item metadata (`status`, owner, tags, title, document overrides)

`workspace-settings.yaml` example:

```yaml
version: 1
cards:
  - pathPattern: "{folder}/feature/{item}"
    fields:
      source: docs
      item: "{item}"
      title: readme_heading
      status: draft
      tags: [docs]
```

If source settings are missing or invalid, Kode Stream falls back to freestyle docs handling.

## Safety Model

- Local-only server bind (`127.0.0.1`)
- Writes restricted to configured sources
- Markdown stale-write detection via expected content hashes
- Metadata writes limited to structured/configured items
- Commit stages only user-selected paths within configured sources
- Pull and branch switch protect dirty trees unless risk is explicitly confirmed
- No credential storage

Kode Stream writes into managed repositories only for explicit user actions (edit, metadata/status update, item
creation, source settings save, commit/pull/push, branch operations). Registry and cache remain app-owned.
Import preview is read-only. Confirmed imports merge into the effective app registry with one atomic replacement, then
scan each new workspace independently. Imported directories are never treated as app-managed clones and are not removed
when their registrations are deleted.

## Project Layout

```text
kode-stream/
├── cmd/
│   └── kode-stream/                # CLI entrypoint
├── internal/
│   ├── server/                      # Composition root, API transport, embedded frontend
│   ├── common/                      # Shared errors, HTTP helpers, compatibility contracts
│   ├── filesystem/                  # Bounded content, path, and write capabilities
│   ├── workspace/                   # Registry, scanning, files, safety, and health
│   ├── item/                        # Item workflows, index, and persistence
│   ├── search/                      # Item, content, and path search
│   ├── knowledge/                   # Structured Wiki indexing and actions
│   ├── git/                         # Guarded Git workflows and repository
│   ├── jira/                        # Jira integration
│   ├── ai/                          # AI settings, launch, and embedded terminal
│   ├── system/                      # Configuration, dialogs, health, and doctor
│   ├── audit/                       # Operation audit events
│   └── navigation/                  # Saved filters and recent items
├── web/                             # React frontend
│   └── src/
│       └── features/
│           └── content-viewer/
├── plans/                           # Product & implementation plans
├── specs/                           # Product requirements & design specs
├── docs/                            # Supporting documentation
├── go.mod
├── go.sum
└── README.md
```

## Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md): system architecture and design decisions
- [Workspace import API](docs/workspace-import-api.md): preview, confirmation, and native file-selection contracts
- [plans/platform/PM-002/README.md](plans/platform/PM-002/README.md): product capability baseline
