---
slug: workspace-lifecycle
title: Workspace Lifecycle And Safe Changes
pageType: CONCEPT
topics: workspace registration, import, source structure, editing, git, safety
summary: Workspaces enter Kode Stream through local registration, managed cloning, or import, then expose guarded content and Git changes against the current checkout.
sourceRef: |
  PM-001 (plan retired) | Read-only workspace model
  PM-002 (plan retired) | Guarded editing and Git operations
  PM-004 (plan retired) | Reliability and safety model
  PM-014 (plan retired) | Source structure proposals
  PM-016 (plan retired) | Remote clone registration
  PM-024 (plan retired) | Existing workspace import
lastVerified: 2026-09-12
---

# Workspace Lifecycle And Safe Changes

## Ways A Workspace Enters The Application
<!-- chunkId: workspace-lifecycle-registration -->
<!-- keywords: local path, remote clone, import, registration -->

[live] PM-001, PM-016, and PM-024 workspace registration supports an existing local path, a managed clone from a Git URL, and discovery plus selective import from another Kode Stream data source. Evidence: `internal/common/models/models.go:236`, `internal/workspace/workspace_lifecycle_service.go:46`, `internal/workspace/workspace_import_service.go:26`.

A managed clone records its origin and managed-root identity so removal can distinguish application-owned clone data
from a user-owned checkout. Import begins with a preview: candidates, duplicates, conflicts, and invalid records are
visible before selected workspaces are registered.

## Source Structure Becomes Workspace Policy
<!-- chunkId: workspace-lifecycle-source-structure -->
<!-- keywords: source proposal, source settings, plan roots, preview -->

[live] PM-014 source-structure proposals are generated from the scanned repository and can be previewed, accepted, replaced, or reset per workspace. Evidence: `internal/common/models/models.go:470`, `internal/server/api/gin_routes.go:128`, `internal/server/api/gin_routes.go:157`.

The accepted settings define where structured items are discovered and how new item paths and fields are produced.
They are configuration, not a second copy of repository content.

## Changes Stay Attached To The Checkout
<!-- chunkId: workspace-lifecycle-safe-changes -->
<!-- keywords: file save, revert, git commit, checkout, audit -->

[live] PM-002 and PM-004 file saves, reverts, commits, synchronization operations, and branch switches pass through workspace-aware services instead of allowing arbitrary filesystem or Git commands. Evidence: `internal/item/item_service.go:166`, `internal/item/item_service.go:234`, `internal/git/git_service.go:243`, `internal/git/git_service.go:292`, `internal/git/git_service.go:341`.

The current checkout remains the operational context. Dirty-tree and synchronization conflicts return recovery details,
and mutation outcomes are recorded for workspace history. Branch review is a separate read-only context documented in
[[workspace-branch-context]].

## Not supported
<!-- chunkId: workspace-lifecycle-not-supported -->
<!-- keywords: arbitrary path, implicit checkout, destructive cleanup -->

[live] PM-002 does not authorize writes outside configured workspace sources, implicit branch switching, or destructive cleanup of user-owned repositories. Evidence: `internal/item/writer/writer.go:243`, `internal/git/git_service.go:341`, `internal/workspace/registry/registry.go:450`.

## Known conflicts
<!-- chunkId: workspace-lifecycle-known-conflicts -->
<!-- keywords: read-only mvp, kanban, legacy workflow -->

[legacy] PM-001 describes the product as a read-only MVP and PM-002 describes the original Kanban-era screens. The current implementation supports guarded mutation and uses Workstream and Item Workspace instead.

## Related pages

See [[workspace-lifecycle-reference]], [[workspace-branch-context]], and [[workspace-workstream-explorer]].
