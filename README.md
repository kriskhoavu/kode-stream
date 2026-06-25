# Plan Manager

Plan Manager is a local Git-native web app for browsing and editing item documents across workspaces.

It helps developers and technical leads turn folder-based item docs into item cards with a Kanban board, workspace management, file explorer, Markdown editor, metadata editor, Git diff, and guarded Git operations.

## Business Point Of View

Engineering plans and configured docs often live in Git as Markdown files. That is good for review, history, and ownership, but it is hard to see progress across many branches, sources, and workspaces.

Plan Manager gives those files a product-management view without moving them out of Git.

It solves these problems:

- Teams can browse items without manually walking folders and branches.
- A workspace can expose multiple sources, such as `plans`, `docs`, or `specs`.
- Structured items, configured docs, and freestyle docs can appear in the same workspace.
- Developers can inspect and update planning context, implementation phases, files, metadata, status, and local diffs in one place.
- The app stores its own registry and cache outside managed workspaces, so scanning does not dirty target workspaces.

## Vision

The long-term vision is a local authoring workspace for Git-based planning.

Current capabilities:

- Register local Git workspaces as workspaces.
- Configure one or more sources.
- Scan structured item roots, configured document roots, and freestyle docs roots.
- Configure `workspace-settings.yaml` for a source so arbitrary docs layouts can be split into item cards.
- Keep plain unstructured docs in a dedicated `Unsorted` Kanban lane until source settings are added.
- View one active workspace at a time.
- Browse a Kanban board by status.
- Filter by source, status, author, branch, and text.
- Browse files across all registered workspaces in Explorer.
- Switch each Explorer workspace to a local Git branch independently.
- Open a preview drawer from a board card.
- Open an item workspace with file tree, Markdown preview, Markdown editor, item info, and diff.
- Edit Markdown files with autosave.
- Edit structured item metadata.
- Move item status from the board or metadata form.
- Create new structured items.
- Commit selected item paths.
- Fetch, pull, push, create branches, and switch branches with guarded flows.
- Edit and delete workspace registrations.
- Reveal local paths in Finder, Windows Explorer, or the Linux file manager.
- Show a stale-content popup when app state changes in another tab.
- Inspect workspace health and recent local operation activity.
- Search indexed items across one or all workspaces with keyboard navigation.
- Save and restore Kanban filter views.
- Reopen recently viewed items from global search.
- Preview Markdown, KaTeX, HTML, JSON, YAML, source code, and plain text through one secure viewer.

See [plans/platform/PM-002/README.md](plans/platform/PM-002/README.md).

## Technical Stack

| Area         | Technology                              | Purpose                                      |
|--------------|-----------------------------------------|----------------------------------------------|
| Backend      | Go 1.22                                 | Local HTTP API, filesystem access, Git calls |
| Frontend     | React 19                                | App shell, Kanban, Explorer, item workspaces |
| Build        | Vite 6                                  | Frontend build and dev server                |
| Language     | TypeScript 5                            | Frontend type safety                         |
| Tests        | Vitest, React Testing Library, Go test  | Unit and UI behavior checks                  |
| Content      | Unified, KaTeX, highlight.js, YAML      | Safe rich file previews                      |
| Icons        | lucide-react                            | UI icons                                     |
| Storage      | YAML files in user config directory     | Registry and item index cache                |
| Distribution | Go binary with embedded frontend assets | Local app runtime                            |

## Project Layout

```text
cmd/plan-manager/          Go CLI entrypoint
internal/app/              HTTP server and embedded frontend setup
internal/api/              HTTP handlers
internal/config/           User config path resolution
internal/fileaccess/       Safe item file tree, file reads, and Markdown writes
internal/gitadapter/       Git status and guarded Git commands
internal/models/           Shared backend models
internal/itemindex/        YAML-backed item summary cache
internal/itemwriter/       Safe Markdown, metadata, status, and new-item writes
internal/audit/            Append-only local operation event storage
internal/navigation/       Saved filter and recent item storage
internal/application/health/  Workspace health checks
internal/application/search/  Ranked item index search
internal/registry/         YAML-backed workspace registry
internal/scanner/          Item and docs scanner
internal/systemdialog/     Native folder picker and path reveal
web/src/                   React app source
web/src/features/content-viewer/  Shared rich content rendering and controls
internal/app/frontend/     Embedded production frontend assets
plans/                     Product and implementation plans
specs/                     Product requirements and design references
docs/                      Supporting docs
```

## Requirements

- Go 1.22 or newer.
- Node.js 20 or newer.
- npm.
- Git.

Native path selection also uses platform tools:

- macOS: `osascript` and `open`.
- Windows: PowerShell and Explorer.
- Linux: `zenity` or `kdialog` for folder selection, and `xdg-open` for path reveal.

## Run The Application

Install dependencies:

```bash
npm install
```

Build the frontend:

```bash
npm run build
```

Build the local binary:

```bash
go build -o ./bin/plan-manager ./cmd/plan-manager
```

Run the app:

```bash
./bin/plan-manager serve -port 4317
```

Open:

```text
http://127.0.0.1:4317
```

The default port is `4317`. You can also set `PLAN_MANAGER_PORT`:

```bash
PLAN_MANAGER_PORT=4317 ./bin/plan-manager serve
```

## Development Commands

Run frontend typecheck:

```bash
npm run typecheck
```

Run frontend tests:

```bash
npm test -- --run
```

Run backend tests:

```bash
go test ./...
```

Build production frontend assets:

```bash
npm run build
```

Build the app binary:

```bash
go build -o ./bin/plan-manager ./cmd/plan-manager
```

## Data Location

Plan Manager stores its app data in the OS user config directory:

```text
<user-config-dir>/plan-manager/
  workspaces.yaml
  item-index.yaml
  audit-log.jsonl
  saved-filters.yaml
  recent-items.yaml
```

Examples:

- macOS: `~/Library/Application Support/plan-manager/`
- Linux: usually `~/.config/plan-manager/`
- Windows: usually `%AppData%\plan-manager\`

Registered workspaces are not used for app registry or cache storage. Plan Manager writes to them only when the user edits Markdown, changes metadata or status, creates an item, saves source item settings, commits, pulls, pushes, or runs a branch operation.

Each structured plan uses a minimal `plan.yaml` as its metadata source:

```yaml
plan:
  status: done
  tags: [backend, frontend]
```

The scanner infers identity from the directory path, title from `README.md`, and document metadata from conventional Markdown paths. `title` is needed only when it intentionally differs from the README heading; `owner` and `tags` are optional.

Sources may also contain an optional `workspace-settings.yaml`. This file is owned by the workspace and describes how a non-standard source root should be split into item cards. The conceptual fields are `source` and `item`; the legacy `repository-settings.yaml`, `service`, `ticket`, and `planDirectories` names are read for migration compatibility:

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

If the settings file is missing or invalid, the scanner falls back to the existing freestyle docs behavior.

## Current Safety Model

- The app binds to `127.0.0.1`.
- Markdown edits autosave after a short debounce.
- Registry and cache writes go to the app config directory.
- File reads and writes are restricted to configured sources.
- Markdown writes use expected content hashes to detect stale edits.
- Metadata writes are limited to structured items and configured source cards. A configured source card without `plan.yaml` gets one when metadata is saved.
- Commit operations stage only selected paths inside configured sources.
- Pull and branch switch block dirty working trees unless the request confirms the risk.
- No credentials are stored.

See [ARCHITECTURE.md](ARCHITECTURE.md) for system design details.
