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
<!-- keywords: issue key, item creation, local plan, seed fields -->

[live] Workstream looks up an issue through the workspace's Jira settings. It can seed a new local item from the summary, description, status, type, assignee and labels. Evidence: `internal/jira/jira_service.go:109`, `web/src/features/workstream/NewWorkItemDialog.tsx:45`.

The item belongs to the repository and follows its source-structure rules. Jira stays the origin reference, never the store for planning state.

## Read Jira Beside The Work
<!-- chunkId: workspace-jira-workstream-reading -->
<!-- keywords: Jira panel, description, attachments, lazy thumbnails -->

[live] An item shows normalized Jira details. Attachments load on request, and thumbnails load lazily. The description renders as Markdown. Evidence: `web/src/features/jira/JiraItemPanel.tsx:10`.

## Jira Can Inform A Session Prompt
<!-- chunkId: workspace-jira-workstream-ai-context -->
<!-- keywords: Jira description, AI prompt, launch composer, explicit inclusion -->

[live] When the person chooses it, the AI launch flow appends the Jira description to the prompt. Evidence: `web/src/features/ai-session/AISessionLaunchControl.tsx:77`.

## Not supported
<!-- chunkId: workspace-jira-workstream-not-supported -->
<!-- keywords: Jira mutation, status sync, attachment upload -->

[live] Editing issues, changing Jira status and uploading attachments. Evidence: `internal/server/api/gin_routes.go:143`.

## Known conflicts
<!-- chunkId: workspace-jira-workstream-known-conflicts -->
<!-- keywords: item panel only, intake, prompt context -->

- "Jira shows only as a panel in Item Workspace." It also seeds new items and session prompts, through the same read-only connection.

## Related pages

See [[workspace-jira-workstream-reference]] and [[workspace-ai-session-launch]].
