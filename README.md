# Plan Manager

Plan Manager is a local Git-native web app for browsing and editing planning documents across repositories.

It helps developers and technical leads turn folder-based planning docs into a workspace with a Kanban board, repository management, file explorer, Markdown editor, metadata editor, Git diff, and guarded Git operations.

## Business Point Of View

Engineering plans often live in Git as Markdown files. That is good for review, history, and ownership, but it is hard to see progress across many branches, services, and repositories.

Plan Manager gives those files a product-management view without moving them out of Git.

It solves these problems:

- Teams can browse plans without manually walking folders and branches.
- A repository can expose multiple plan sources, such as `plans` and `docs`.
- Structured plans and freestyle docs can appear in the same workspace.
- Developers can inspect and update planning context, implementation phases, files, metadata, status, and local diffs in one place.
- The app stores its own registry and cache outside managed repositories, so scanning does not dirty target repos.

## Vision

The long-term vision is a local authoring workspace for Git-based planning.

Current PM-002 capabilities:

- Register local Git repositories.
- Configure one or more plan directories.
- Scan structured plan roots, configured document roots, and freestyle docs roots.
- Configure `repository-settings.yaml` for a source directory so arbitrary docs layouts can be split into work item cards.
- View one active repository workspace at a time.
- Browse a Kanban board by status.
- Filter by source, status, author, branch, and text.
- Open a preview drawer from a board card.
- Open a plan workspace with file tree, Markdown preview, Markdown editor, plan info, and diff.
- Edit Markdown files with autosave.
- Edit structured plan metadata.
- Move plan status from the board or metadata form.
- Create new structured plans.
- Commit selected plan paths.
- Fetch, pull, push, create branches, and switch branches with guarded flows.
- Edit and delete repository registrations.
- Reveal local paths in Finder, Windows Explorer, or the Linux file manager.
- Show a stale-content popup when app state changes in another tab.

See [plans/platform/PM-002/README.md](plans/platform/PM-002/README.md).

## Technical Stack

| Area         | Technology                              | Purpose                                      |
|--------------|-----------------------------------------|----------------------------------------------|
| Backend      | Go 1.22                                 | Local HTTP API, filesystem access, Git calls |
| Frontend     | React 19                                | App shell, Kanban, repository, plan views    |
| Build        | Vite 6                                  | Frontend build and dev server                |
| Language     | TypeScript 5                            | Frontend type safety                         |
| Tests        | Vitest, React Testing Library, Go test  | Unit and UI behavior checks                  |
| Markdown     | marked                                  | Markdown preview rendering                   |
| Icons        | lucide-react                            | UI icons                                     |
| Storage      | YAML files in user config directory     | Registry and plan index cache                |
| Distribution | Go binary with embedded frontend assets | Local app runtime                            |

## Repository Layout

```text
cmd/plan-manager/          Go CLI entrypoint
internal/app/              HTTP server and embedded frontend setup
internal/api/              HTTP handlers
internal/config/           User config path resolution
internal/fileaccess/       Safe plan file tree, file reads, and Markdown writes
internal/gitadapter/       Git status and guarded Git commands
internal/models/           Shared backend models
internal/planindex/        YAML-backed plan summary cache
internal/planwriter/       Safe Markdown, metadata, status, and new-plan writes
internal/registry/         YAML-backed repository registry
internal/scanner/          Plan and docs scanner
internal/systemdialog/     Native folder picker and path reveal
web/src/                   React app source
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
  repositories.yaml
  plan-index.yaml
```

Examples:

- macOS: `~/Library/Application Support/plan-manager/`
- Linux: usually `~/.config/plan-manager/`
- Windows: usually `%AppData%\plan-manager\`

Registered repositories are not used for app registry or cache storage. PM-002 writes to them only when the user edits Markdown, changes metadata or status, creates a plan, saves source structure settings, commits, pulls, pushes, or runs a branch operation.

Source directories may also contain an optional `repository-settings.yaml`. This file is owned by the repository and describes how a non-standard source root should be split into cards:

```yaml
version: 1
cards:
  - pathPattern: "{service}/feature/{ticket}"
    fields:
      service: "{service}"
      ticket: "{ticket}"
      title: readme_heading
      status: draft
      tags: [docs]
```

If the settings file is missing or invalid, the scanner falls back to the existing freestyle docs behavior.

## Current Safety Model

- The app binds to `127.0.0.1`.
- Markdown edits autosave after a short debounce.
- Registry and cache writes go to the app config directory.
- File reads and writes are restricted to configured plan directories.
- Markdown writes use expected content hashes to detect stale edits.
- Metadata writes are limited to structured plans and configured source cards. A configured source card without `plan.yaml` gets one when metadata is saved.
- Commit operations stage only selected paths inside configured plan directories.
- Pull and branch switch block dirty working trees unless the request confirms the risk.
- No credentials are stored.

See [ARCHITECTURE.md](ARCHITECTURE.md) for system design details.
