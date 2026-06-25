# Backend Design: Performance And Architecture Review

## Overview

The backend should keep existing API routes and JSON response shapes while reducing repeated filesystem, Git, and index work. PM-015 introduces clearer patterns around scanning, state versioning, refresh decisions, and handler ownership.

## Current Implementation Review

| Component                      | Current Shape                                                                    | Risk                                                                              |
|--------------------------------|----------------------------------------------------------------------------------|-----------------------------------------------------------------------------------|
| `internal/api/api.go`          | Single route file with all handlers, response helpers, and request decoding      | Hard to review endpoint changes and easy to grow beyond resource boundaries       |
| `workspace.Service.State`      | Lists workspaces, queries all indexed items, marshals payload, and hashes it     | Polling `/api/state` scales with item count rather than metadata changes          |
| `scanner.Scanner`              | Uses `SourceReader`, but parsing, Git metadata, documents, and counts share flow | Hard to add caching or batch metadata without touching broad scan behavior        |
| `workspacefiles.Service`       | Mutations decide audit and refresh inline                                        | New mutation types must repeat refresh logic and source boundary decisions        |
| `workspacefiles.Access.Search` | Walks directories per request with bounded entry count                           | Correct but repeated searches may redo ignore checks and stat/classification work |
| `itemindex.Index`              | In-memory YAML-backed list with linear query/get                                 | Acceptable for small data, but hot paths read all items repeatedly                |

## Target Patterns

| Pattern                  | Backend Use                                                                              |
|--------------------------|------------------------------------------------------------------------------------------|
| Pipeline                 | Scanner stages: source roots, item candidates, metadata, documents, Git metadata, output |
| Strategy                 | Refresh policy chooses full scan, branch scan, targeted item refresh, or no refresh      |
| Adapter                  | Git, filesystem, source reader, and index interfaces stay behind services                |
| Command/Query Separation | Workspace file reads/searches stay separate from writes/reverts/renames                  |
| Facade                   | Existing `api.API` and `scanner.Scanner` remain stable while internals split             |

## Data Model

No database schema is introduced. Existing YAML-backed storage remains the source of persistence.

| Store                | Existing File     | PM-015 Change                                                           |
|----------------------|-------------------|-------------------------------------------------------------------------|
| Workspace registry   | `workspaces.yaml` | No format change                                                        |
| Item index           | `item-index.yaml` | Add or derive lightweight state metadata without changing item contract |
| Branch scan metadata | `item-index.yaml` | Reuse `BranchScanMetadata` for branch cache validation                  |
| Audit log            | `audit-log.jsonl` | No format change; ensure mutation timing remains recorded               |

## API Contract

PM-015 should preserve all public routes. Internal handler files may be split by resource.

| Method | Endpoint Pattern                       | Contract Change | Notes                                                            |
|--------|----------------------------------------|-----------------|------------------------------------------------------------------|
| GET    | `/api/state`                           | None            | Should become O(metadata) rather than O(all items) when possible |
| POST   | `/api/workspaces/{id}/scan`            | None            | May use staged scanner internals and batch metadata              |
| POST   | `/api/workspaces/{id}/kanban/branch`   | None            | Preserve branch snapshot cache behavior from PM-013              |
| GET    | `/api/workspaces/{id}/tree`            | None            | Keep lazy directory loading and ignore behavior                  |
| GET    | `/api/workspaces/files/content-search` | None            | Keep budgeted search; expose no new fields in first phase        |
| POST   | `/api/items/{id}/files/{fileID}`       | None            | Preserve materialization and stale-content guards                |

## Backend Design

### Scanner Pipeline

```text
Source root
  -> source settings resolver
  -> item candidate matcher
  -> metadata parser
  -> document resolver
  -> file count provider
  -> Git metadata provider
  -> item assembler
```

The scanner facade should still expose `Scan` and `ScanWithRequest`. Internals can move into small stage functions or types that are independently tested.

### State Snapshot

`workspace.Service.State` should avoid hashing every item on each poll. The index can expose a lightweight snapshot based on:

- workspace count and last scanned timestamps
- item count
- latest indexed update time
- branch scan metadata versions
- persisted index file modification time or explicit revision

The initial implementation can compute and cache this snapshot inside `itemindex.Index` after load and writes.

### Refresh Policy

Workspace file mutations should delegate refresh decisions to a small policy:

```text
MutationResult + affected paths + workspace sources
  -> no refresh
  -> targeted source refresh
  -> current branch refresh
  -> full workspace refresh
```

This keeps future operations such as delete, move folder, or bulk edit from duplicating path checks.

### Batch Git Metadata

The scanner should use a `MetadataProvider` interface. The current per-path Git calls stay as fallback; a batch provider can load author/update data for many item paths in one command or bounded set of commands.

## Design Decisions

| Decision                                           | Rationale                                                                           |
|----------------------------------------------------|-------------------------------------------------------------------------------------|
| Keep YAML storage                                  | PM-015 is performance and architecture work, not a persistence migration            |
| Preserve API routes                                | Frontend and tests already depend on stable route contracts                         |
| Introduce interfaces only at hot or volatile seams | Avoid abstraction churn while making scan, refresh, and handlers easier to extend   |
| Measure before and after                           | Performance work should include scan/state/search timing baselines                  |
| Keep full scan fallback                            | Correctness is more important than partial refresh when source ownership is unclear |

## Verification Strategy

| Area             | Verification Command                                                                            |
|------------------|-------------------------------------------------------------------------------------------------|
| Backend all      | `rtk go test ./...`                                                                             |
| Scanner focused  | `rtk go test ./internal/scanner ./internal/itemindex`                                           |
| API focused      | `rtk go test ./internal/api ./internal/application/...`                                         |
| Performance note | Capture before/after timing for `/api/state`, scan, and workspace search on a fixture workspace |
