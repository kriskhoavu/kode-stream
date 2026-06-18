# PM-001: Plan Manager Read-Only MVP

## Overview

Plan Manager is a local Git-native web app for browsing planning documents.

The MVP lets a developer register local Git repositories as workspaces, scan one or more sources, view one active workspace on a Kanban board, and open an item workspace with a file tree, Markdown preview, raw Markdown view, metadata, and read-only Git diff.

The MVP is read-only for managed repositories. It does not edit plan files. It does not run Git write operations. It only writes Plan Manager registry and cache files in the app data directory.

## Source Material

| Source                                       | Role                 | How It Guides This Plan                                                                                  |
|----------------------------------------------|----------------------|----------------------------------------------------------------------------------------------------------|
| [Requirement](../../../specs/requirement.md) | Product requirements | Defines repository management, plan discovery, Kanban, workspace, Git operations, and distribution goals |
| [Design Image](../../../specs/design.png)    | UI reference         | Defines the desktop shell, board layout, plan workspace, mobile board, and light/dark visual direction   |

## Glossary

| Term                 | Meaning                                                                             | Maps To (code)              |
|----------------------|-------------------------------------------------------------------------------------|-----------------------------|
| Workspace            | A local Git repository registered in Plan Manager                                   | `RepositoryConfig`          |
| Source               | A configured scan root such as `plans`, `docs`, or `docs/plans`                     | `planDirectories`           |
| Structured Item Root | An item root that usually uses scope/identifier folders, such as `plans/api/DI-170` | `PlanScanner`               |
| Freestyle Docs Root  | A Markdown docs root that does not use scope/identifier folders                     | `metadataSource: docs`      |
| Item                 | A planning folder or docs item shown on the board                                   | `PlanSummary`, `PlanDetail` |
| Scope                | Optional item grouping, currently stored as `service`                               | `service`                   |
| Identifier           | Optional stable item key, currently stored as `ticket`                              | `ticket`                    |
| Item Metadata        | Optional machine-readable metadata for an item                                      | `plan.yaml`                 |
| Document             | A Markdown file that belongs to an item or docs root                                | `PlanDocument`              |
| Scan                 | Read-only indexing of configured sources                                            | `RepositoryScanner`         |
| Board Status         | The Kanban column for an item                                                       | `PlanStatus`                |
| Item Workspace       | The details view for one item or docs root                                          | `PlanWorkspace`             |
| Visual Baseline      | The required UI reference for v1                                                    | `specs/design.png`          |

## Components

| Layer    | Component          | Purpose                                                                          |
|----------|--------------------|----------------------------------------------------------------------------------|
| Backend  | Workspace registry | Stores, updates, and deletes registered workspaces in the user data directory    |
| Backend  | Plan scanner       | Reads Git state, structured item roots, freestyle docs roots, and Markdown files |
| Backend  | Plan index         | Caches searchable item summaries and document metadata                           |
| Backend  | App state API      | Exposes a cheap version for stale-content detection                              |
| Backend  | HTTP API           | Serves repository, plan, file, and diff data to the frontend                     |
| Frontend | App shell          | Shows active workspace in the top bar and workspace selection in the left nav    |
| Frontend | Kanban board       | Shows active-workspace items by status with source and multi-select filters      |
| Frontend | Workspace page     | Registers, edits, deletes, scans, and reveals local workspaces                   |
| Frontend | Item workspace     | Shows file tree, raw Markdown, preview, metadata, and read-only diff             |
| DevOps   | Build packaging    | Builds one local app binary with embedded frontend assets                        |
| DevOps   | AI verification    | Runs Playwright MCP checks during implementation                                 |

## Data Flow

```text
Developer starts Plan Manager
  -> backend loads app config from user data directory
  -> frontend asks for repositories
  -> developer selects one active workspace from the left nav
  -> developer registers or edits this repo and plan directories
  -> backend validates Git repo, branch, and folders
  -> developer triggers Scan
  -> scanner reads local branches and working tree
  -> scanner indexes structured plan folders and freestyle docs roots
  -> scanner reads plan.yaml first when present
  -> scanner falls back to folder and README parsing when plan.yaml is missing
  -> app state version changes after registry or index updates
  -> other tabs show a refresh popup instead of auto-reloading
  -> frontend renders board columns, filter facets, and cards
  -> developer opens a card
  -> frontend loads file tree, file content, metadata, and diff
```

## Design Decisions

| Decision                                    | Alternatives Considered                     | Rationale                                                                                                                           |
|---------------------------------------------|---------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------|
| Use Go plus React/Vite                      | Node-only, Rust plus React                  | Go gives a simple local binary and strong filesystem/Git access. React/Vite fits the proposed UI.                                   |
| Store app data outside managed repos        | Store config in each repo, config file only | The app should not dirty target repositories. A cache is needed for large plan sets.                                                |
| Make v1 read-only                           | Editable workspace, full Git manager        | Read-only browsing gives value first and avoids save, lock, credential, and branch mutation risks.                                  |
| Use `plan.yaml` first                       | README-only parsing                         | Existing plans already use `plan.yaml`. It gives stable metadata. File explorer order is filesystem-based.                          |
| Add fallback parsing                        | Require `plan.yaml`                         | Older plans and custom folders should still appear as normal plan cards with inferred metadata.                                     |
| Support freestyle docs roots                | Force docs into scope/identifier structure  | General docs folders such as `docs/` should be browsable without fake identifiers.                                                  |
| Scope Kanban to one active workspace        | Mix all repositories on one board           | A board should represent one project workspace. Repository switching belongs in the left nav.                                       |
| Use client-side multi-select board filters  | Add many query params to `/api/plans`       | The board loads cached summaries for the active workspace. Source, status, author, and branch facets give OR filters without churn. |
| Show stale-content prompt                   | Auto-reload pages                           | Reading and detail views should not be interrupted. Users decide when to refresh in-place.                                          |
| Keep repository edit/delete app-local       | Treat registry changes as managed repo ops  | Registry writes only touch Plan Manager data. They do not modify registered repositories.                                           |
| Do not auto fetch in v1                     | Fetch every 15 seconds                      | Fetch changes `.git` refs and can trigger credentials. Manual scan is safer for v1.                                                 |
| Treat `specs/design.png` as visual baseline | Treat image as inspiration only             | The UI must not drift away from the documented proposal.                                                                            |
| Use Playwright MCP as a phase gate          | Manual browser checks only                  | AI-agent-run browser checks make layout and workflow regressions visible during development.                                        |

## Implementation Clarifications

- PM-001 should support at least 100 workspaces, 10,000 items, and 100,000 files through cached item summaries.
- Board and list views must read from cached metadata. They must not load every Markdown file on each render.
- File content should load only when the user opens a plan file.
- Backend code should keep clear boundaries between repository registry, Git access, scanning, indexing, and HTTP handlers.
- HTTP handlers must not read arbitrary filesystem paths directly. They must go through the plan index and file access layer.
- Manual Scan rebuilds derived metadata for one repository.
- Workspace edit updates app registry metadata after validation.
- Workspace delete removes the app registry entry and cached items for that workspace.
- Kanban shows one active repository/workspace at a time.
- Kanban can filter by configured source, such as `plans` or `docs`.
- Registry and plan-index changes update the app state version.
- When another tab changes content, existing tabs show a top-right refresh popup.
- The refresh popup reloads app data in place and does not refresh the whole browser page.
- A bad plan creates a scan warning. It must not fail the whole repository scan.
- The app must not write to registered repositories in PM-001.
- File reads must stay inside configured plan directories.
- Structured item roots usually use scope/identifier folders.
- Freestyle docs roots with Markdown files are indexed as docs items.
- The UI exposes generic content labels: Item, Source, Scope, and Identifier.
- Kanban filters support OR within a facet and AND across facets.
- The UI should match the layout, density, navigation, and mobile behavior of `specs/design.png`. It does not need pixel-perfect parity.

## Next Plan

After PM-001 is complete, create `PM-002: Plan Editing And Git Operations`.

PM-002 should turn the read-only workspace into a safe authoring workflow. It should cover Markdown editing, status moves, new plan creation, commit, pull, push, branch create, branch switch, dirty-state handling, and write-operation safeguards.

PM-002 should reuse the PM-001 terminology and APIs where possible. It should add write APIs only after the read-only scan, board, workspace, and Playwright MCP acceptance flow are stable.

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Infrastructure Design](design/design-03-infrastructure.md)
- [Pipeline Design](design/design-04-pipeline.md)
- [Implementation Plan](implementation-plan.md)
