# PM-003: Technical Architecture Refactoring

## Overview

PM-003 improves the internal architecture of Plan Manager without changing behavior, APIs, workflows, or UI. The app is stable, but several modules now combine routing, orchestration, parsing, state, and rendering in the same files. This plan documents the current debt, the target architecture, and a low-risk migration path.

## Related Plans

| Item                          | Relationship   | Key Context                                                                                      |
|-------------------------------|----------------|--------------------------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Parent feature | Created the read-only registry, scanner, item index, Kanban board, and item workspace            |
| [PM-002](../PM-002/README.md) | Parent feature | Added editing, source settings, guarded writes, Git operations, stale state, and authoring flows |

### What PM-002 Established

- **Workspace**: a local Git repository registered in Plan Manager.
- **Source**: a configured scan root such as `plans`, `docs`, or `specs`.
- **Item**: a planning folder or docs item shown in the app.
- **Source Structure**: source-owned settings that map arbitrary folders to cards.
- **Write Guard**: backend checks that keep file and Git operations inside allowed paths.
- **App State Version**: `/api/state` changes when registry or indexed item data changes.
- **No credential storage**: Git credentials stay in the user's local Git setup.

## Current Architecture Assessment

| Area                | Current State                                                                                                    | Risk                                                                                          |
|---------------------|------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------|
| Backend routing     | `internal/api/api.go` owns route registration, request decoding, orchestration, response shaping, and helpers    | High coupling makes small endpoint changes hard to test in isolation                          |
| Backend scanning    | `internal/scanner/scanner.go` mixes traversal, source settings matching, YAML parsing, fallback parsing, and IDs | Scanner changes can regress multiple source modes at once                                     |
| Backend path safety | Safe path helpers exist in `fileaccess`, `itemwriter`, `writeguard`, and `api`                                   | Duplication can create inconsistent safety behavior                                           |
| Backend models      | `internal/models` is the shared contract for API, storage, scanner, writer, and Git                              | Convenient, but it couples storage and transport shape                                        |
| Frontend shell      | `App.tsx` owns routing, workspace state, stale state polling, theme state, and navigation layout                 | App-wide state is difficult to reuse or test outside the shell                                |
| Frontend pages      | `KanbanPage.tsx`, `ItemWorkspacePage.tsx`, and `WorkspacesPage.tsx` include state, data loading, helpers, and UI | Large components hide reusable behavior and make focused tests expensive                      |
| Frontend styles     | `web/src/styles/app.css` is a single global stylesheet of more than 3,000 lines                                  | Style ownership is unclear and regressions are hard to localize                               |
| Tests               | Backend has focused package tests; frontend has limited page and helper tests                                    | Refactors need characterization tests around orchestration, state, diff parsing, and settings |
| Performance         | Scans and Git metadata collection are synchronous and repeated after many writes                                 | Large workspaces may pay avoidable filesystem and Git process cost                            |

## Improvement Opportunities

- Split HTTP routing from application use cases.
- Introduce backend service interfaces at package boundaries.
- Consolidate path guard logic into one package.
- Split scanner traversal, source settings matching, metadata parsing, and item assembly.
- Keep API JSON types stable while allowing internal domain types to evolve.
- Move frontend route, workspace, stale state, editor, Git, and source settings behavior into hooks.
- Extract repeated frontend panels and utility logic from large pages.
- Replace the single CSS file with feature-owned CSS sections or modules while preserving class names during migration.
- Add characterization tests before moving code.
- Cache repeated scan inputs and avoid full rescans after writes that only need targeted refresh.

## Target Architecture

```text
cmd/plan-manager
  -> internal/app
  -> internal/httpapi
  -> internal/application
  -> internal/domain
  -> internal/storage, internal/files, internal/gitadapter, internal/scanner

web/src
  -> app shell and route adapters
  -> features/{kanban,item-workspace,workspaces,branches,items}
  -> shared api, hooks, ui, domain helpers
```

The target keeps the same API routes and response payloads. The main change is responsibility ownership. HTTP handlers should decode requests and call application services. Application services should coordinate registry, index, scanner, file, and Git operations. Domain and guard packages should own reusable rules. The frontend should keep page output unchanged while moving state and helpers into testable hooks and feature modules.

## Recommended Package Structure

### Backend

| Package                          | Responsibility                                                                 |
|----------------------------------|--------------------------------------------------------------------------------|
| `internal/app`                   | Process wiring, embedded frontend, server startup                              |
| `internal/httpapi`               | Routes, request decoding, response encoding, HTTP status mapping               |
| `internal/application/workspace` | Workspace registration, scanning, source settings, state version orchestration |
| `internal/application/item`      | Item details, files, metadata writes, status changes, item creation            |
| `internal/application/git`       | Guarded Git operations and post-operation refresh decisions                    |
| `internal/domain/item`           | Item IDs, statuses, metadata rules, document ordering, diff-facing helpers     |
| `internal/domain/workspace`      | Workspace validation, source validation, app terminology                       |
| `internal/security/pathguard`    | Safe joins, symlink checks, selected path validation, source scope checks      |
| `internal/scanner`               | Public scanner facade                                                          |
| `internal/scanner/source`        | Source traversal and source settings matching                                  |
| `internal/scanner/metadata`      | `item.yaml` parsing, fallback Markdown parsing, legacy compatibility           |
| `internal/storage/registry`      | Workspace YAML persistence                                                     |
| `internal/storage/itemindex`     | Derived item index persistence and query                                       |
| `internal/files`                 | File tree, file reads, Markdown writes, content hashes                         |
| `internal/gitadapter`            | Thin Git command adapter                                                       |
| `internal/models`                | Stable API DTOs during migration; shrink after services own internal types     |

### Frontend

| Path                              | Responsibility                                                              |
|-----------------------------------|-----------------------------------------------------------------------------|
| `web/src/app/App.tsx`             | App shell composition only                                                  |
| `web/src/app/router.ts`           | Current path parsing and navigation helpers                                 |
| `web/src/app/useAppState.ts`      | Workspace list, active workspace, content refresh, stale state polling      |
| `web/src/shared/api`              | Request wrapper, endpoint clients, response normalization                   |
| `web/src/shared/domain`           | Status labels, source helpers, diff parsing, file tree helpers              |
| `web/src/shared/ui`               | Reusable menus, dialogs, status badges, file tree, panels                   |
| `web/src/features/kanban`         | Board state, filters, lanes, cards, preview drawer                          |
| `web/src/features/item-workspace` | Editor state, file tree, preview/raw/diff panels, metadata panel, Git panel |
| `web/src/features/workspaces`     | Workspace forms, source settings editor, directory picker integration       |
| `web/src/features/items`          | List view                                                                   |
| `web/src/features/branches`       | Branch grouping and navigation                                              |
| `web/src/styles`                  | Global tokens plus feature-owned styles, migrated without visual changes    |

## Performance Opportunities

| Opportunity                               | Why It Helps                                                            | Risk Control                                             |
|-------------------------------------------|-------------------------------------------------------------------------|----------------------------------------------------------|
| Targeted item refresh after metadata save | Avoids full workspace scans for writes that touch one item              | Keep full scan fallback and compare test output          |
| Scanner result cache per source root      | Avoids re-reading unchanged trees during repeated scans                 | Key by path, mod time, settings hash, and branch         |
| Batch Git metadata lookups                | Reduces one Git process per item for author and update time             | Add tests that current author/update fallback is stable  |
| Memoized frontend selectors               | Reduces repeated filter and file tree work on render                    | Keep pure helper tests around filters and tree selection |
| Lazy render heavy panels                  | Avoids diff/editor work when panels are hidden                          | Preserve tab state and existing loading behavior         |
| API client request grouping               | Reduces duplicate item/file/diff/Git requests on workspace page opening | Keep endpoint contracts unchanged                        |

## Migration Strategy

1. Add characterization tests around current behavior.
2. Extract pure helpers without changing call sites.
3. Move backend orchestration into application services behind existing API handlers.
4. Split scanner internals while keeping `scanner.Scanner.Scan` unchanged.
5. Consolidate path guards and replace duplicate helpers one package at a time.
6. Move frontend state into hooks with the same rendered markup and class names.
7. Split large frontend components into feature components.
8. Move CSS into feature-owned files after component ownership is clear.
9. Add targeted performance improvements only after measurements or regression tests exist.

## Design Decisions

| Decision                                      | Alternatives Considered                         | Rationale                                                          |
|-----------------------------------------------|-------------------------------------------------|--------------------------------------------------------------------|
| Keep API contracts unchanged                  | Version API routes during refactor              | PM-003 is architectural only and must not change workflows         |
| Use application services before new storage   | Split storage first                             | Use cases reveal real boundaries and reduce handler coupling first |
| Keep `models` as compatibility DTOs initially | Rename all models to domain and API types early | A staged migration avoids high-risk cross-package churn            |
| Preserve frontend class names during splits   | Convert to CSS modules immediately              | Keeping selectors stable prevents accidental visual changes        |
| Add characterization tests first              | Refactor and rely on manual testing             | The acceptance criteria require unchanged behavior                 |
| Optimize after structural seams exist         | Add caching before refactoring                  | Caching mixed into large modules would increase coupling           |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Architecture Design](design/design-01-architecture.md)
- [Backend Design](design/design-02-backend.md)
- [Frontend Design](design/design-03-frontend.md)
- [Implementation Plan](implementation-plan.md)
