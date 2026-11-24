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
<!-- keywords: local path, remote clone, registry import, managed root -->

[live] A workspace is registered from an existing local path, cloned from a Git URL into a managed root, or imported from another Kode Stream registry file. Evidence: `internal/workspace/workspace_lifecycle_service.go:46`, `internal/workspace/workspace_import_service.go:26`.

A managed clone records its origin and managed root, so removal can tell Kode Stream's clone from a user's checkout. Import starts with a preview of candidates, duplicates and invalid records. A person then picks which to register.

## Source Structure Becomes Workspace Policy
<!-- chunkId: workspace-lifecycle-source-structure -->
<!-- keywords: source proposal, plan roots, item paths, preview -->

[live] Kode Stream proposes a source structure from the scanned repository. A person previews it, then accepts, replaces or resets it per workspace. Evidence: `internal/common/models/source_structure.go:3`.

The accepted settings say where items are found and how new item paths and fields are made. They are configuration, not a copy of repository content.

## Changes Stay Attached To The Checkout
<!-- chunkId: workspace-lifecycle-safe-changes -->
<!-- keywords: file save, revert, commit, audit history -->

[live] File saves, reverts, commits, synchronization and branch switches go through workspace-aware services. No arbitrary filesystem or Git command runs. Evidence: `internal/item/item_service.go:162`, `internal/git/git_service.go:338`.

Dirty-tree and synchronization conflicts return recovery details. Workspace history records each outcome. Branch review is a separate read-only context: see [[workspace-branch-context]].

## Not supported
<!-- chunkId: workspace-lifecycle-not-supported -->
<!-- keywords: arbitrary path, implicit checkout, destructive cleanup -->

[live] Writes outside configured sources, implicit branch switches and destructive cleanup of a user-owned repository. Evidence: `internal/item/writer/paths.go:27`, `internal/workspace/workspace_lifecycle_service.go:86`.

## Known conflicts
<!-- chunkId: workspace-lifecycle-known-conflicts -->
<!-- keywords: read-only mvp, kanban board, legacy screens -->

- "Kode Stream is read-only, with a Kanban board." It edits through guarded actions, and its surfaces are Workstream and Item Workspace.

## Related pages

See [[workspace-lifecycle-reference]] and [[workspace-workstream-explorer]].
