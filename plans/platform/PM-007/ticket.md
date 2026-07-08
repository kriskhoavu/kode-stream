# PM-007: Workspace Explorer

## Status

Done

## User Story

As a developer, I want to browse every registered workspace from one global Explorer so that I can inspect and edit planning files without changing the active Kanban workspace.

## Problem

Kode Stream only exposed indexed item folders. Users could not browse the real repository tree, preview files outside items, or use the existing Markdown workflow across a workspace.

## Scope

- Show every registered workspace as a top-level root.
- Load one directory level at a time.
- Hide Git-ignored entries by default.
- Protect `.git`, traversal paths, and outside symlinks.
- Preview supported text formats through the shared content viewer.
- Edit Markdown with autosave and stale-hash protection.
- Show selected-file diff and support confirmed revert.
- Decorate indexed item directories with planning context.
- Persist expansion, selection, ignored mode, and pane sizes.
- Support keyboard navigation and responsive layouts.

## Acceptance Criteria

- [x] Explorer is available at `/explorer` on desktop and mobile navigation.
- [x] Every registered workspace appears without changing the active Kanban workspace.
- [x] Expanding a node requests only its immediate children.
- [x] `.git`, traversal paths, and outside symlinks are inaccessible.
- [x] Ignored files are hidden by default and available through an explicit toggle.
- [x] Supported text files use the PM-006 content viewer.
- [x] Only Markdown files are editable.
- [x] Markdown saves require the expected content hash and preserve permissions.
- [x] Diff and confirmed revert operate on one guarded workspace-relative file.
- [x] Indexed item directories show identifier, title, and status context.
- [x] Expansion, route selection, and pane preferences persist locally.
- [x] Existing Kanban and item detail behavior remains unchanged.
- [x] Backend, frontend, and production-build checks pass.

## Delivery

| Area       | Result                                                                   |
|------------|--------------------------------------------------------------------------|
| Backend    | Added guarded workspace listing, read, save, diff, and revert services   |
| Frontend   | Added lazy tree state, shared editor session, Explorer UI, and inspector |
| Safety     | Added path, symlink, ignore, hash, binary, and Markdown-only tests       |
| Validation | 116 Go tests, 50 frontend tests, TypeScript, and production build pass   |

## Related Plans

- [PM-006](../PM-006/README.md): secure content rendering.
- [PM-008](../PM-008/README.md): Explorer productivity follow-up.
