---
slug: platform-documentation-audit
title: Documentation Boundary
pageType: REFERENCE
topics: documentation, wiki, architecture, operations
summary: Which repository folder owns which kind of documentation, so each fact has one home.
sourceRef: |
  docs/README.md | Documentation map
  docs/architecture/README.md | Architecture documents
  wiki/platform/documentation-audit.md | Boundary
lastVerified: never
---

## Boundary
<!-- chunkId: platform-documentation-audit-boundary -->
<!-- keywords: wiki authority, architecture diagrams, operator procedures -->

| Folder               | Owns                                                       |
|----------------------|------------------------------------------------------------|
| `wiki/`              | Product behavior and its code evidence                     |
| `docs/architecture/` | System overview, mode diagrams and adapter class diagrams  |
| `docs/development.md`| Build and test guide for contributors                      |
| `deploy/`            | Operator procedures beside the assets they drive           |
| `README.md`          | The human entry point                                      |

A fact lives in one folder. Other folders link to it.

## Not supported
<!-- chunkId: platform-documentation-audit-not-supported -->
<!-- keywords: duplicate authority, docs domain, second corpus -->

`docs/` holds no product behavior. Storage, Cloud, Canvas sessions, session changes and workspace import live in [[platform-storage-reference]], [[platform-cloud-runtime-reference]], [[workspace-terminal-canvas-reference]], [[workspace-session-focus-reference]] and [[workspace-lifecycle-reference]].

## Shared Lint Follow-up
<!-- chunkId: platform-documentation-audit-shared-lint-follow-up -->
<!-- keywords: shared lint, valid prefixes, slug prefix -->

[open] When should the shared `wiki_lint.py` add `workspace` and `knowledge` to `VALID_PREFIXES`? [[wiki-conventions]] declares them, but the script reports every such slug as unknown.
