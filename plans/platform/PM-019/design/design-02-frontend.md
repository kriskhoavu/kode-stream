# Frontend Design: Jira Integration and Workspace Settings

## Overview

Keep the existing Jira item section, then replace the Workspaces page's permanent create form and expanding edit cards with a master-detail workspace manager. Workspace configuration is split into Overview, Sources, Integrations, and Health. Jira loads independently from local item detail so remote latency or failure cannot block local files, metadata, diff, or Git controls.

## Information Architecture

```text
Workspaces
├── Header: Scan all, Add workspace
├── Workspace list
│   ├── Search and explicit bulk-selection mode
│   └── Compact rows: name, location type, health, item count
└── Selected workspace detail
    ├── Overview: identity, location, branch, registration, removal
    ├── Sources: source directories and source structure
    ├── Integrations: Jira connection and connection test
    └── Health: scan state, checks, warnings, logs, and Scan

Application Settings
└── Storage: data directory and managed clone directory
```

## Component Boundaries

| Component               | Responsibility                                                        |
|-------------------------|-----------------------------------------------------------------------|
| `WorkspacesPage`        | Load page selection and coordinate list, detail, dialogs, and notices |
| `WorkspaceList`         | Search, select, summarize, and explicitly enter bulk-selection mode   |
| `WorkspaceDetails`      | Own active detail tab and unsaved-change navigation guard             |
| `WorkspaceOverview`     | Edit name, path, baseline branch, and show registration metadata      |
| `WorkspaceSources`      | List sources and open the existing source-structure editor            |
| `WorkspaceIntegrations` | Edit optional Jira configuration and test the connection              |
| `WorkspaceHealth`       | Show detailed checks, scan state, warnings, logs, and scan action     |
| `AddWorkspaceDialog`    | Register a local folder or clone a remote repository                  |
| `StorageSettings`       | Edit application-wide data and managed clone directories              |

## State Management

| State                 | Owner            | Behavior                                                       |
|-----------------------|------------------|----------------------------------------------------------------|
| Selected workspace    | Workspaces page  | Default to first available workspace; preserve valid selection |
| Active detail tab     | Workspace detail | Keep workspace settings domains separate                       |
| Dirty draft           | Detail tab       | Save per domain and guard tab/workspace navigation             |
| Add workspace draft   | Add dialog       | Reset only after successful registration or explicit close     |
| Bulk-selection mode   | Workspace list   | Hidden by default; contains selection and removal actions      |
| Jira connection draft | Integrations tab | Validate deployment-specific required fields                   |
| Connection test       | Integrations tab | Explicit request; reset when connection fields change          |
| Item issue state      | Jira item hook   | Load on item change; cancel stale requests                     |
| Refresh state         | Jira item hook   | Disable duplicate refresh and preserve prior issue             |
| Attachment action     | Attachment list  | Explicit open/download; display proxy errors separately        |

## Workspace Configuration

- Jira is optional and disabled by default.
- Jira configuration lives under the selected workspace's `Integrations` tab.
- Deployment selection controls Cloud email requirements.
- Token input stores an environment-variable name, never the value.
- Test Connection reports missing process environment, authentication, project, and network failures.
- Saving is allowed only after local validation; a failed connection test requires explicit confirmation to retain settings.

## Workspace Manager

- Use a two-column master-detail layout on desktop and list-to-detail navigation on narrow screens.
- Keep list rows compact. Show name, health state, item count or last scan, and local/remote type.
- The selected row has one clear visual state and remains visible while settings are edited.
- Put `Add workspace` in the page header as the primary action and `Scan all` as secondary.
- Use a labeled overflow menu for reveal, bulk-selection entry, and other secondary actions.
- Do not render forms, health checks, or Jira fields inside list rows.
- Put workspace removal at the bottom of Overview and retain the existing confirmation semantics.
- Show notices near the action that produced them; do not use one page-wide status for unrelated operations.
- Track busy state per operation so scanning one workspace does not disable unrelated navigation.

## Add Workspace Dialog

- Start with a clear `Local folder` / `Remote Git URL` choice.
- Infer the workspace name from the selected folder or repository URL.
- Detect or suggest baseline branch and common sources where supported.
- Keep branch and sources under reviewable defaults instead of chip-plus-free-text controls.
- Place Jira and uncommon fields under `Advanced settings`; configuration can also be completed later.
- Keep clone progress, logs, failures, and retry inside the dialog.
- After success, close the dialog and select the newly registered workspace.

## Global Storage Settings

- Move Data Directory from Workspaces to application Settings under `Storage`.
- Display the managed clone directory as a derived or separately supported path according to the existing API contract.
- Explain restart requirements before save and again in the successful result.
- Preserve current browse and reveal capabilities with visible text labels or accessible names.

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
- Tabs use standard keyboard navigation and expose the selected state.
- On narrow screens, Back to workspaces returns to the prior list position and selection.
- Icon-only actions require tooltips and accessible names; primary operations use visible labels.
- Destructive controls are not adjacent to Save and cannot be triggered through an ambiguous icon.

## Design Decisions

| Decision                        | Rationale                                                             |
|---------------------------------|-----------------------------------------------------------------------|
| Load Jira independently         | Remote failure must not break local-first item workflows              |
| Keep attachments metadata-first | Avoid broken previews and unrequested network transfer                |
| Use a side-panel section        | Jira enriches the item without becoming its source of truth           |
| Use master-detail navigation    | Keeps workspace context visible while containing settings complexity  |
| Save settings per detail tab    | Makes scope and validation clear and avoids one oversized draft       |
| Use an add-workspace dialog     | Registration is occasional and should not occupy permanent space      |
| Move storage to app settings    | The data directory is global rather than workspace-specific           |
| Keep source editor as a dialog  | Source structure is a focused advanced task with an existing workflow |
