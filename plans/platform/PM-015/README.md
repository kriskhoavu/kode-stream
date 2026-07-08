# PM-015: Implementation Performance And Render Architecture Review

PM-015 reviews the current Kode Stream backend and frontend implementation and turns the findings into a phased improvement plan. The work focuses on performance, render mechanics, code conventions, design-pattern based decomposition, and future extension points without changing current user workflows.

## Related Plans

| Item                          | Relationship          | Key Context                                                                                  |
|-------------------------------|-----------------------|----------------------------------------------------------------------------------------------|
| [PM-003](../PM-003/README.md) | Architecture baseline | Split early app services and frontend helpers, but large page and orchestration files remain |
| [PM-013](../PM-013/README.md) | Source reader model   | Added `SourceReader`, branch snapshots, materialization, and branch-aware scan metadata      |
| [PM-014](../PM-014/README.md) | Source Items UX       | Added source proposal generation and richer source-settings preview behavior                 |

## Review Findings

| Area                    | Current Observation                                                                  | Improvement Direction                                                             |
|-------------------------|--------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------|
| Backend scan/state      | `/api/state` hashes full workspace and item payloads; scans still do repeated reads  | Add a state snapshot/version store and source-scan pipeline with reusable stages  |
| Backend Git metadata    | Scanner asks Git for author/update data per item path                                | Add a batch metadata provider behind the scanner facade                           |
| Backend handlers        | `internal/api/api.go` is 908 lines and owns every route in one file                  | Split handlers by resource while keeping the same `api.API` facade and routes     |
| Backend workspace files | Tree, search, mutation, audit, refresh, and Git state decisions are tightly coupled  | Use command/query separation and a refresh policy strategy                        |
| Frontend pages          | `KanbanPage`, `WorkspacesPage`, and `ItemWorkspacePage` are 1338, 765, and 745 lines | Move data loading and user actions into feature controllers/hooks                 |
| Frontend renderers      | Markdown rendering and source highlighting run on the main thread                    | Add async renderer adapters, bounded caches, and deferred heavy preview rendering |
| Frontend Explorer       | `useWorkspaceExplorer` has cache, selection, tree, Git, and decoration concerns      | Split into tree store, selection controller, and provider adapters                |
| Code conventions        | Some indentation and feature ownership are inconsistent after iterative changes      | Add package/module ownership rules and focused lint-like review checks            |

## Glossary

| Term               | Meaning                                                                                   | Code                                                      |
|--------------------|-------------------------------------------------------------------------------------------|-----------------------------------------------------------|
| Render Adapter     | Frontend strategy that renders one content kind safely and asynchronously                 | `ContentViewer`, `MarkdownPreview`, `SourceCodeView`      |
| Scan Pipeline      | Ordered backend stages that read sources, match items, parse metadata, and decorate items | `scanner.Scanner`, `SourceReader`                         |
| Refresh Policy     | Decision object that chooses full scan, branch scan, targeted refresh, or no refresh      | `workspacefiles.Service.refreshIfSource`                  |
| State Snapshot     | Lightweight app version computed from persisted index metadata                            | `workspace.Service.State`                                 |
| Feature Controller | Hook/module that owns page data loading and commands, leaving JSX focused on layout       | `useWorkspaceExplorer`, future Kanban hooks               |
| Provider Adapter   | Interface around filesystem, Git, search, or renderer implementation                      | `SourceReader`, `workspacefiles.Access`, renderer modules |

## Components

| Layer    | Component                             | Purpose                                                                       |
|----------|---------------------------------------|-------------------------------------------------------------------------------|
| Backend  | `internal/scanner`                    | Convert source roots into item details through a staged pipeline              |
| Backend  | `internal/itemindex`                  | Persist item summaries, branch metadata, warnings, and future state snapshots |
| Backend  | `internal/application/workspace`      | Coordinate scans, branch loads, source settings, and app state                |
| Backend  | `internal/application/workspacefiles` | Coordinate file mutations, audit records, refresh policy, and Git state       |
| Backend  | `internal/api`                        | Keep HTTP contracts stable while handlers move into resource files            |
| Frontend | `web/src/features/content-viewer`     | Render Markdown, HTML, JSON/YAML, and source text safely and efficiently      |
| Frontend | `web/src/features/workspace-explorer` | Own Explorer tree cache, selection, path search, Git markers, and mutations   |
| Frontend | `web/src/features/kanban`             | Own Kanban filters, branch loading, drag/drop, cards, and preview drawer      |
| Frontend | `web/src/features/file-editor`        | Own autosave, stale-content handling, and reusable edit session state         |

## Data Flow

```text
User opens a workspace view
  -> frontend feature controller loads route-specific data
  -> API resource handler decodes request and calls application service
  -> service chooses query, mutation, scan, or refresh policy
  -> scanner/workspace file provider uses bounded adapters
  -> item index writes metadata and state snapshot
  -> frontend renders through memoized selectors and renderer adapters
```

## Design Decisions

| Decision                                              | Alternatives Considered                      | Rationale                                                                               |
|-------------------------------------------------------|----------------------------------------------|-----------------------------------------------------------------------------------------|
| Keep PM-015 behavior-preserving at first              | Combine refactor with new UI features        | Performance and architecture changes need stable contracts and regression tests         |
| Use staged backend pipelines and strategy interfaces  | Add one-off caches inside existing functions | Patterns make scan, refresh, and renderer behavior easier to extend safely              |
| Split by resource before renaming public types        | Rename packages and DTOs immediately         | Stable API and TypeScript contracts reduce review risk                                  |
| Move frontend orchestration into feature controllers  | Split JSX only                               | Render performance improves when data loading and derived state have explicit ownership |
| Defer heavy renderer work                             | Keep all preview rendering synchronous       | Large Markdown/source files should not block typing, navigation, or tree interaction    |
| Define conventions in architecture docs after changes | Rely on implicit local style                 | Future PM tickets need clear ownership and extension rules                              |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
