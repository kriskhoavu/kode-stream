# Backend Design: Service-Oriented Refactor

## Overview

The backend should keep all existing routes while moving orchestration out of `internal/api/api.go`. The current API package should become a delivery layer. Use case services should own workspace, item, and Git workflows.

## Current Technical Debt

| Module                | Debt                                                                                 | Refactor Direction                                                                |
|-----------------------|--------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------|
| `internal/api/api.go` | Routes, decoding, orchestration, normalization, Git guards, and helpers in one file  | Split into `httpapi` handlers plus application services                           |
| `internal/scanner`    | Traversal, source settings, metadata parsing, fallback docs, and natural sorting mix | Keep scanner facade and split internals by concern                                |
| `internal/fileaccess` | Safe path logic overlaps with writer and API Git path validation                     | Move common path logic to `internal/security/pathguard`                           |
| `internal/itemwriter` | Writes metadata, creates items, refreshes scans, and knows registry/index details    | Move refresh sequencing to item application service                               |
| `internal/models`     | API, persistence, scanner, and UI-facing shapes share one package                    | Keep as DTO compatibility layer, then introduce focused domain types where useful |
| `internal/gitadapter` | Adapter is mostly thin, but app decisions are split between API and writer refreshes | Keep as adapter; move guard and refresh decisions to application Git service      |

## Target Services

| Service             | Public Methods                                                                                               |
|---------------------|--------------------------------------------------------------------------------------------------------------|
| `workspace.Service` | `List`, `Get`, `Create`, `Update`, `Delete`, `Scan`, `State`, `GetSourceStructure`, `SaveSourceStructure`    |
| `item.Service`      | `List`, `Detail`, `Files`, `FileContent`, `SaveFile`, `RevertFile`, `SaveMetadata`, `UpdateStatus`, `Create` |
| `git.Service`       | `Status`, `Fetch`, `Pull`, `Push`, `Commit`, `CreateBranch`, `SwitchBranch`                                  |

## Route Ownership

| Current Route Group     | Future Handler             | Future Service        |
|-------------------------|----------------------------|-----------------------|
| `/api/workspaces*`      | `httpapi.WorkspaceHandler` | `workspace.Service`   |
| `/api/items*`           | `httpapi.ItemHandler`      | `item.Service`        |
| `/api/workspaces/*/git` | `httpapi.GitHandler`       | `git.Service`         |
| `/api/system/*`         | `httpapi.SystemHandler`    | `systemdialog.Dialog` |
| `/api/state`            | `httpapi.StateHandler`     | `workspace.Service`   |

## Package Migration

| Phase | Move                                                                   | Compatibility Rule                                     |
|-------|------------------------------------------------------------------------|--------------------------------------------------------|
| B1    | Add tests and extract pure API helpers into package-private files      | No package rename yet                                  |
| B2    | Introduce application services and call them from existing API methods | Routes and DTOs stay unchanged                         |
| B3    | Rename `internal/api` to `internal/httpapi` or add `httpapi` facade    | `internal/app` is the only app wiring change           |
| B4    | Split scanner internals under stable `scanner.Scanner` facade          | `Scan(workspace)` result remains byte-for-byte stable  |
| B5    | Consolidate path guard helpers                                         | Existing path guard tests must pass before replacement |
| B6    | Add targeted refresh and Git batching when covered by tests            | Full scan remains fallback on unexpected conditions    |

## Backend Test Additions

| Area        | Tests                                                                  |
|-------------|------------------------------------------------------------------------|
| HTTP        | Route status codes and JSON shape for workspace, item, source, and Git |
| Services    | Fake dependencies for scan, save, status move, commit, pull, and push  |
| Scanner     | Configured source matching, fallback docs roots, item YAML precedence  |
| Path guard  | Traversal, symlink escape, source scope, selected Git paths            |
| Performance | Scan fixture tests that count Git calls after batching is introduced   |

## Backend Performance Plan

- Keep full workspace scans as the correctness baseline.
- Add an application-level targeted refresh method for single-item metadata and status writes.
- Add Git metadata batching only after scanner behavior has package-level tests.
- Use source settings file hash and directory mod time to skip unchanged source traversal where safe.
- Keep cache invalidation explicit and local to scan services.

