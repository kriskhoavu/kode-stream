# Implementation Plan: PM-023 - Domain-Oriented Backend Packaging

## Overview

Migrate the backend one dependency-safe phase at a time. Every phase must compile, pass the full Go suite, and be committed before the next phase begins.

## Phases Summary

| Phase | Name                                  | Status      |
|-------|---------------------------------------|-------------|
| B1    | Architecture contract and foundations | Complete    |
| B2    | Supporting domains                    | Complete    |
| B3    | AI, Git, and Jira domains             | Complete    |
| B4    | Workspace, Item, Search, and System   | Complete    |
| B5    | Knowledge domain                      | In Progress |
| B6    | Legacy removal and full verification  | Pending     |

## Backend Phases

### Phase B1: Architecture Contract And Foundations

**Deliverables:**

- [x] Record package, naming, dependency, and compatibility rules.
- [x] Add shared HTTP helpers and route-composition foundations.
- [x] Document role filenames, domain ownership, and dependency rules without adding a technical package.

**Verification:** `go test ./... && go vet ./...`

**Commit:** `PM-023: Define domain packaging architecture`

### Phase B2: Supporting Domains

**Deliverables:**

- [x] Migrate health, navigation, audit, configuration, and system behavior.
- [x] Move their owned HTTP routes into domain controllers.
- [x] Preserve storage formats and focused tests.

Navigation and Audit reached their final ownership. Configuration is now owned by System. Health was moved out of the legacy application tree as an intermediate step; B4 folds workspace checks into Workspace and application liveness into System.

**Verification:** `go test ./... && go vet ./...`

**Commit:** `PM-023: Migrate supporting backend domains`

### Phase B3: AI, Git, And Jira Domains

**Deliverables:**

- [x] Consolidate Git, Jira, and AI settings/session behavior into their three owning domains.
- [x] Co-locate Git and Jira repositories with their domains; fold AI settings, process, and terminal behavior into AI.
- [x] Preserve OS-specific behavior and route contracts.

**Verification:** `go test ./... && go vet ./...`

**Commit:** `PM-023: Migrate integration backend domains`

### Phase B4: Workspace, Item, Search, And System Domains

**Deliverables:**

- [x] Consolidate registry, scanner, and workspace-file behavior under Workspace ports and services.
- [x] Consolidate item index, writer, and item-file behavior under Item ports and services.
- [x] Migrate item and content search with explicit cross-domain ports.
- [x] Fold workspace health into Workspace and application health/configuration/dialog behavior into System.

**Verification:** `go test ./... && go vet ./...`

**Commit:** `PM-023: Migrate core backend domains`

### Phase B5: Knowledge Domain

**Deliverables:**

- [ ] Combine Knowledge controller, service, models, parsing, relationships, and repository ports in the domain package.
- [ ] Move persistence and process implementations into infrastructure.
- [ ] Preserve PM-022 detection, safety, and action behavior.

**Verification:** `go test ./... && go vet ./...`

**Commit:** `PM-023: Migrate Knowledge backend domain`

### Phase B6: Legacy Removal And Full Verification

**Deliverables:**

- [x] Remove the obsolete `application` layer and route services directly to domain packages.
- [ ] Remove compatibility aliases and the obsolete `api` and `models` packages.
- [x] Remove obsolete top-level technical packages after their implementations move to the ownership map.
- [ ] Review the final package graph against the documented ownership map.
- [ ] Mark PM-023 complete and synchronize documentation with implemented names.

**Verification:** `go test ./... && go vet ./... && cd web && npm run typecheck && npm test -- --run && npm run build`

**Commit:** `PM-023: Complete domain backend migration`
