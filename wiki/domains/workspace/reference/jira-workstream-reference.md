---
slug: workspace-jira-workstream-reference
title: Jira Workstream Reference
pageType: REFERENCE
topics: Jira connection, issue lookup, attachment proxy, cache, Workstream item
summary: Reference for workspace-scoped Jira configuration, read-only issue lookup, bounded attachment access, and local item creation.
sourceRef: |
  plans/platform/PM-019/design/design-01-backend.md
  plans/platform/PM-019/design/design-02-frontend.md
  plans/platform/PM-025/design/design-01-backend.md
  plans/platform/PM-025/design/design-02-frontend.md
lastVerified: 2026-09-12
---

# Jira Workstream Reference

## Workspace Scope
<!-- chunkId: workspace-jira-workstream-reference-scope -->
<!-- keywords: workspace Jira, connection, issue lookup, cache -->

[live] PM-019 Jira lookup resolves the connection from the owning workspace; item lookup and workspace-plus-key lookup share the same service and cache behavior. Evidence: `internal/jira/jira_service.go:92`, `internal/jira/jira_service.go:110`, `internal/jira/jira_service.go:201`.

Credentials are referenced through configuration rather than returned to the browser. Authentication, permission, and
availability failures become distinct unavailable states with recovery guidance.

## Issue And Attachment Boundary
<!-- chunkId: workspace-jira-workstream-reference-attachments -->
<!-- keywords: issue API, attachment ID, origin, size limit, media type -->

[live] PM-019 item attachments are fetched only when the attachment ID belongs to the issue already resolved for that item. Evidence: `internal/jira/jira_service.go:253`, `internal/jira/jira_service_test.go:242`.

The proxy validates the content URL and redirect origin, enforces a 25 MiB maximum, bounds attachment count, and
normalizes content type and filename before streaming. Browser clients do not receive Jira credentials.

## Local Item Creation
<!-- chunkId: workspace-jira-workstream-reference-creation -->
<!-- keywords: Jira issue, plan draft, source structure, duplicate -->

[live] PM-025 Workstream resolves Jira before constructing the local draft and then creates the item through the same source-aware item creation path as manual intake. Evidence: `web/src/pages/WorkstreamPage.tsx:483`, `web/src/pages/WorkstreamPage.tsx:500`, `internal/item/writer/writer_test.go:201`.

Jira fields seed local values but do not bypass identifier, path, source, or collision validation.

## Not supported
<!-- chunkId: workspace-jira-workstream-reference-not-supported -->
<!-- keywords: Jira write, webhook, bidirectional sync -->

[live] PM-019 the route manifest exposes issue and attachment reads but no Jira write or synchronization endpoint. Evidence: `internal/server/api/gin_routes.go:137`, `internal/server/api/gin_routes.go:144`.

## Known conflicts
<!-- chunkId: workspace-jira-workstream-reference-known-conflicts -->
<!-- keywords: issue cache, attachment behavior, stale plan -->

[legacy] Earlier plan screenshots and component names predate Jira-first creation. The service's read-only and workspace-scoped boundary remains unchanged.

## Related pages

See [[workspace-jira-workstream]], [[workspace-lifecycle-reference]], and [[workspace-ai-launch-reference]].
