# Backend Design: Jira-First Workspace

## Overview

Add a workspace-scoped Jira lookup that does not require an existing item, then extend item creation so it can scaffold a structured item from normalized Jira context. The implementation reuses PM-019 Jira connection, token, client, cache, and issue normalization behavior. It does not write back to Jira.

## Data Model

| Type                   | Key Fields                                                   | Purpose                                                     |
|------------------------|--------------------------------------------------------------|-------------------------------------------------------------|
| `JiraIssueLookupState` | `state`, `issue`, `message`, `recoveryHint`, `refreshedAt`   | Workspace-level issue lookup response before an item exists |
| `NewItemInput`         | existing fields, `jiraKey`, `initialReadme`, `owner`, `tags` | Item creation request with optional Jira context            |
| `AIPlanPreset`         | `id`, `name`, `prompt`, `contextMode`, optional `provider`   | Named prompt template exposed to the frontend               |
| `WriteResult`          | existing item detail and scan timestamp                      | Returned after Jira-backed or blank item creation           |

## API Contract

| Method | Endpoint                                      | Request                 | Response                 |
|--------|-----------------------------------------------|-------------------------|--------------------------|
| GET    | `/api/workspaces/{id}/jira/issues/{issueKey}` | None                    | `JiraIssueLookupState`   |
| POST   | `/api/items`                                  | Extended `NewItemInput` | `WriteResult`            |
| GET    | `/api/ai/presets`                             | None                    | `AIPlanPreset[]`         |
| POST   | `/api/items/{id}/ai-sessions`                 | Extended launch input   | Existing launch result   |
| POST   | `/api/items/{id}/ai-sessions/embedded`        | Extended launch input   | Existing embedded result |

## Jira Lookup

- Resolve the workspace from the registry.
- Require workspace Jira configuration.
- Normalize and uppercase the requested key.
- Reject invalid Jira issue-key shape before calling Jira.
- Reject keys outside the configured project before calling Jira.
- Use the existing Jira client and normalized `Issue`.
- Reuse the PM-019 five-minute cache with a workspace, base URL, and issue-key cache key.
- Return PM-019 state names for auth, forbidden, not found, unavailable, and available responses.

## Item Creation

- Keep blank item creation valid with the current required fields.
- When `jiraKey` is supplied, require `identifier` to match the normalized Jira key unless the user explicitly chooses a different identifier in the intake form.
- Write README with Jira context only when `initialReadme` is supplied by the trusted frontend flow.
- Keep attachments as links and metadata references. Do not download or commit Jira attachments.
- Rescan through the existing writer and index refresh.
- Do not persist Jira issue data into the app index beyond item metadata already used for display.

## AI Presets

- Store v1 presets as built-in backend defaults, not user-editable files.
- Preserve existing provider and terminal settings.
- Launch requests may include a free prompt or preset ID.
- Expand prompt placeholders with existing workspace and item values.
- Keep `workspace_only` and `card_context` as the only context modes.

## Design Decisions

| Decision                                  | Rationale                                                                   |
|-------------------------------------------|-----------------------------------------------------------------------------|
| Workspace-scoped Jira lookup              | A ticket can exist before any Plan Manager item exists                      |
| No new Jira persistence                   | Jira remains authoritative and PM-019 avoids a stale local issue cache      |
| README context instead of ticket snapshot | Gives AI enough context while keeping the plan folder simple                |
| Built-in presets for v1                   | Enables guided AI planning without creating a new registry or settings file |
| Remove `/kanban` API assumptions          | User requested no backward compatibility for the old surface name           |
