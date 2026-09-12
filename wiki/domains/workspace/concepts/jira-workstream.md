---
slug: workspace-jira-workstream
title: Jira-First Workstream Intake
pageType: CONCEPT
topics: Jira, workstream, item creation, read-only integration, settings
summary: A workspace can read Jira issue context and use it to seed a local Workstream item without mutating Jira.
sourceRef: |
  PM-019 (plan retired) | Read-only Jira integration
  PM-025 (plan retired) | Jira-first Workstream
lastVerified: 2026-09-12
---

# Jira-First Workstream Intake

## Jira Is Context For A Local Item
<!-- chunkId: workspace-jira-workstream-context -->
<!-- keywords: issue key, item creation, workspace, local plan -->

[live] PM-019 and PM-025 Workstream can look up an issue through workspace Jira settings and seed a new local structured item from the returned summary, description, status, type, assignee, and labels. Evidence: `internal/jira/jira_service.go:110`, `web/src/pages/WorkstreamPage.tsx:455`, `web/src/pages/WorkstreamPage.tsx:483`.

The resulting item belongs to the repository and follows its source-structure rules. Jira remains the origin reference,
not the persistence layer for Kode Stream planning state.

## Read Jira Beside The Work
<!-- chunkId: workspace-jira-workstream-reading -->
<!-- keywords: Jira panel, description, status, attachments -->

[live] PM-019 an item can display normalized Jira details and load its attachments on demand. Evidence: `web/src/features/jira/JiraItemPanel.tsx:7`, `web/src/features/jira/JiraItemPanel.tsx:32`.

Descriptions render through the shared Markdown path. Attachments remain collapsed until requested, and image thumbnails
are lazy rather than part of the initial issue fetch.

## Jira Can Inform A Session Prompt
<!-- chunkId: workspace-jira-workstream-ai-context -->
<!-- keywords: Jira description, AI prompt, launch composer, explicit inclusion -->

[live] PM-025 the AI launch flow can append the available Jira description when the user explicitly chooses that context. Evidence: `web/src/features/ai-session/AISessionLaunchControl.tsx:79`, `web/src/features/ai-session/AISessionLaunchDialog.test.tsx:107`.

## Not supported
<!-- chunkId: workspace-jira-workstream-not-supported -->
<!-- keywords: Jira mutation, status sync, attachment upload -->

[live] PM-019 and PM-025 do not edit Jira issues, transition Jira status, or upload attachments. Evidence: `internal/server/api/gin_routes.go:137`, `internal/server/api/gin_routes.go:144`.

## Known conflicts
<!-- chunkId: workspace-jira-workstream-known-conflicts -->
<!-- keywords: settings route, original panel, later intake -->

[legacy] PM-019 treats Jira primarily as an Item Workspace panel. PM-025 extended the same read-only connection into Workstream item creation and AI prompt context.

## Related pages

See [[workspace-jira-workstream-reference]], [[workspace-workstream-explorer]], and [[workspace-ai-session-launch]].
