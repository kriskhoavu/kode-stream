---
slug: workspace-jira-workstream-reference
title: Jira Workstream Reference
pageType: REFERENCE
topics: Jira connection, issue lookup, attachment proxy, cache, Workstream item
summary: Reference for workspace-scoped Jira configuration, read-only issue lookup, bounded attachment access, and local item creation.
sourceRef: |
  PM-019 (plan retired) | Jira service and presentation contracts
  PM-025 (plan retired) | Jira intake and Workstream contracts
lastVerified: 2026-09-12
---

# Jira Workstream Reference

## Workspace Scope
<!-- chunkId: workspace-jira-workstream-reference-scope -->
<!-- keywords: workspace Jira, connection, issue lookup, cache -->

[live] Jira lookup takes its connection from the owning workspace. Item lookup and workspace-plus-key lookup share one service and cache. Evidence: `internal/jira/jira_service.go:91`, `internal/jira/jira_service.go:120`.

Configuration references the credentials. The browser never receives them. Authentication, permission and availability failures each return their own unavailable state with recovery guidance.

## Issue And Attachment Boundary
<!-- chunkId: workspace-jira-workstream-reference-attachments -->
<!-- keywords: attachment ID, redirect origin, size limit, media type -->

[live] An attachment is fetched only when its ID belongs to the issue already resolved for the item. Evidence: `internal/jira/jira_service.go:307`.

The proxy checks the content URL and redirect origin, caps size at 25 MiB, bounds the attachment count, and normalizes content type and filename before streaming.

## Local Item Creation
<!-- chunkId: workspace-jira-workstream-reference-creation -->
<!-- keywords: plan draft, source structure, collision validation -->

[live] Workstream resolves Jira first, then creates the item through the same source-aware path as manual intake. Jira fields never bypass identifier, path, source or collision checks. Evidence: `web/src/features/workstream/NewWorkItemDialog.tsx:45`, `internal/item/writer/writer_test.go:199`.

## Not supported
<!-- chunkId: workspace-jira-workstream-reference-not-supported -->
<!-- keywords: Jira write, webhook, bidirectional sync -->

[live] The route manifest has issue and attachment reads only. There is no Jira write, webhook or synchronization endpoint. Evidence: `internal/server/api/gin_routes.go:143`.

## Known conflicts
<!-- chunkId: workspace-jira-workstream-reference-known-conflicts -->
<!-- keywords: plan screenshots, component names, stale plan -->

- "Plan screenshots show the current Jira screens." They predate Jira-first creation. The read-only boundary is the same.

## Related pages

See [[workspace-jira-workstream]] and [[workspace-ai-launch-reference]].
