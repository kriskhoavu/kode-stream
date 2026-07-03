# Backend Design: Guarded Jira Editing

## Overview

Extend the PM-019 Jira adapter and application service with edit metadata, field updates, transitions, uploads, and deletion. Cloud and Server/Data Center adapters preserve deployment-specific behavior behind normalized contracts.

## Data Model

| Type                   | Key Fields                                                     |
|------------------------|----------------------------------------------------------------|
| `JiraEditMetadata`     | `version`, `editableFields`, `transitions`, `attachmentPolicy` |
| `JiraFieldUpdate`      | `version`, `fields`                                            |
| `JiraTransitionInput`  | `version`, `transitionId`, optional supported fields           |
| `JiraAttachmentResult` | `filename`, `status`, `attachment`, `message`                  |

## API Contract

| Method | Endpoint                                          | Request                  | Response              |
|--------|---------------------------------------------------|--------------------------|-----------------------|
| GET    | `/api/items/{id}/jira/edit-metadata`              | None                     | `JiraEditMetadata`    |
| PATCH  | `/api/items/{id}/jira`                            | `JiraFieldUpdate`        | Refreshed `JiraIssue` |
| POST   | `/api/items/{id}/jira/transitions`                | `JiraTransitionInput`    | Refreshed `JiraIssue` |
| POST   | `/api/items/{id}/jira/attachments`                | Bounded multipart files  | Attachment results    |
| DELETE | `/api/items/{id}/jira/attachments/{attachmentId}` | Version and confirmation | Refreshed `JiraIssue` |

## Mutation Controls

- Fetch edit metadata and the current issue version before rendering the editor.
- Allow only configured fields supported by the selected deployment adapter.
- Submit status changes through Jira transition IDs, never by writing status text.
- Compare the supplied issue version immediately before mutation and return `409` on mismatch.
- Apply multipart body, file count, per-file size, aggregate size, filename, media-type, and timeout limits.
- Verify attachment membership before deletion and require explicit confirmation.
- Invalidate the PM-019 cache and fetch the normalized issue after every successful mutation.
- Audit issue key, operation, field names or attachment metadata, status, and duration; omit values and content.

## Design Decisions

| Decision                            | Rationale                                                        |
|-------------------------------------|------------------------------------------------------------------|
| Deployment adapters own Jira writes | Cloud and Server capabilities and payloads differ                |
| Refetch after mutation              | Prevents local assumptions about Jira automation and transitions |
| Reject stale versions               | Prevents silent overwrites of newer Jira state                   |
