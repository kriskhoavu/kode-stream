# Frontend Design: Read-Only Jira Integration

## Overview

Add Jira fields to workspace create/edit settings and a Jira section to the item workspace right panel. Jira loads independently from local item detail so remote latency or failure cannot block local files, metadata, diff, or Git controls.

## State Management

| State                 | Owner           | Behavior                                                |
|-----------------------|-----------------|---------------------------------------------------------|
| Jira connection draft | Workspace form  | Validate deployment-specific required fields            |
| Connection test       | Workspace form  | Explicit request; reset when connection fields change   |
| Item issue state      | Jira item hook  | Load on item change; cancel stale requests              |
| Refresh state         | Jira item hook  | Disable duplicate refresh and preserve prior issue      |
| Attachment action     | Attachment list | Explicit open/download; display proxy errors separately |

## Workspace Configuration

- Jira is optional and disabled by default.
- Deployment selection controls Cloud email requirements.
- Token input stores an environment-variable name, never the value.
- Test Connection reports missing process environment, authentication, project, and network failures.
- Saving is allowed only after local validation; a failed connection test requires explicit confirmation to retain settings.

## Item Side Panel

- Add a `Jira` section alongside existing item information without replacing local metadata.
- Display issue key/link, summary, status, type, people, priority, labels, timestamps, and normalized description.
- Render descriptions through safe React nodes; do not use raw HTML injection.
- Show state-specific messages for not configured, malformed identifier, project mismatch, missing issue, authentication, forbidden access, and outage.
- Show attachment filename, type, size, author, and date only. Fetch content after an explicit action.
- Attachment failures do not collapse or replace issue details.

## Accessibility and Layout

- Long summaries, descriptions, and filenames wrap without expanding the right panel beyond its configured width.
- Attachment controls have descriptive accessible names and keyboard focus.
- Loading and refresh states use live status text without repeatedly announcing unchanged issue details.

## Design Decisions

| Decision                        | Rationale                                                   |
|---------------------------------|-------------------------------------------------------------|
| Load Jira independently         | Remote failure must not break local-first item workflows    |
| Keep attachments metadata-first | Avoid broken previews and unrequested network transfer      |
| Use a side-panel section        | Jira enriches the item without becoming its source of truth |
