# Architecture Design: PM-003 Target Architecture

## Overview

The target architecture separates delivery, application orchestration, domain rules, and infrastructure adapters. It keeps public behavior stable and changes the internal ownership of code in small steps.

## Architectural Layers

| Layer            | Owns                                                              | Must Not Own                                             |
|------------------|-------------------------------------------------------------------|----------------------------------------------------------|
| Delivery         | HTTP routing, request decoding, response encoding, browser shell  | Business rules, scanner traversal, Git command decisions |
| Application      | Use case orchestration and transaction-like sequencing            | HTTP details, React state, low-level filesystem parsing  |
| Domain           | Stable rules, validation, IDs, statuses, item metadata semantics  | File IO, Git processes, local storage formats            |
| Infrastructure   | YAML persistence, filesystem access, Git commands, native dialogs | Workflow decisions, API response shaping                 |
| Frontend feature | View state, effects, feature composition, rendering               | Backend rules that must be enforced server-side          |
| Shared frontend  | API client, pure helpers, reusable UI components                  | Feature-specific orchestration                           |

## Current Dependency Direction

```text
internal/app
  -> internal/api
     -> registry, itemindex, scanner, fileaccess, itemwriter, gitadapter, systemdialog, writeguard

web/src/App.tsx
  -> pages
  -> api client
  -> browser storage and route state

large pages
  -> api client
  -> local state
  -> rendering
  -> pure helpers
```

## Target Dependency Direction

```text
internal/app
  -> internal/httpapi
     -> internal/application/*
        -> internal/domain/*
        -> internal/storage/*
        -> internal/files
        -> internal/scanner
        -> internal/gitadapter

web/src/app
  -> web/src/features/*
     -> web/src/shared/api
     -> web/src/shared/domain
     -> web/src/shared/ui
```

## Boundary Contracts

| Boundary                   | Contract                                                                                  |
|----------------------------|-------------------------------------------------------------------------------------------|
| HTTP to application        | Input structs, workspace or item IDs, and typed result values                             |
| Application to scanner     | `Scan(workspace) -> items, warnings`; later targeted scan APIs can be added behind facade |
| Application to file access | Safe file tree, read, write, relative path, and content hash operations                   |
| Application to Git adapter | Status and operation methods; adapter does not decide when to rescan                      |
| Frontend page to hook      | View model, command functions, loading/error state, and dirty state                       |
| Hook to API client         | Stable endpoint methods and normalized DTOs                                               |

## Dependency Rules

- HTTP packages may depend on application packages.
- Application packages may depend on domain and infrastructure interfaces.
- Domain packages must not depend on HTTP, storage, scanner, Git, or React.
- Infrastructure packages must not depend on HTTP handlers.
- Frontend features may depend on shared modules.
- Shared frontend modules must not depend on feature modules.

## Test Strategy

| Test Type                | Coverage                                                                           |
|--------------------------|------------------------------------------------------------------------------------|
| Backend characterization | Current route responses, error codes, scan results, write guards, Git guard inputs |
| Backend unit             | Domain validation, path guard, scanner metadata parsing, source matching           |
| Backend service          | Application services with fake registry, index, scanner, files, and Git            |
| Frontend helper          | Filters, source labels, diff parsing, file tree state, source settings             |
| Frontend hook            | Loading, dirty state, autosave, stale state, Git command state                     |
| Frontend smoke           | Existing page behavior with React Testing Library                                  |

## Acceptance Protection

| Acceptance Criterion              | Protection                                                                   |
|-----------------------------------|------------------------------------------------------------------------------|
| Behavior remains unchanged        | Characterization tests before extraction                                     |
| More modular and testable         | Application services, hooks, and pure helper modules                         |
| Coupling is reduced               | Directional package rules and service interfaces                             |
| Performance improves where useful | Add measured scanner, Git, and render optimizations after structure improves |
| Architecture documentation exists | PM-003 plan docs and updated top-level architecture docs after migration     |

