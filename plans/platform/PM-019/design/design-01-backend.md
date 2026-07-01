# Backend Design: Read-Only Jira Integration

## Overview

Extend workspace persistence with optional Jira connection metadata and add a `jira` application service behind deployment-specific clients. HTTP transport, cache, and attachment streaming are injected and bounded for tests. Jira responses are never added to the item index.

## Data Model

| Type                 | Key Fields                                                                                 |
|----------------------|--------------------------------------------------------------------------------------------|
| `JiraConnection`     | `deploymentType`, `baseURL`, `projectKey`, `accountEmail`, `tokenEnvVar`                   |
| `JiraIssue`          | `key`, `summary`, `status`, `description`, `issueType`, `assignee`, `reporter`, `priority` |
| `JiraAttachment`     | `id`, `filename`, `mediaType`, `sizeBytes`, `createdAt`, `author`                          |
| `JiraConnectionTest` | `ok`, `deploymentType`, `projectKey`, `message`, `recoveryHint`                            |
| `JiraIssueState`     | `state`, `issue`, `message`, `recoveryHint`, `refreshedAt`                                 |

`JiraConnection` is nested in `WorkspaceConfig` and `WorkspaceInput`. The registry persists configuration but never resolves or stores the token value.

## API Contract

| Method | Endpoint                                          | Request          | Response             |
|--------|---------------------------------------------------|------------------|----------------------|
| POST   | `/api/workspaces/{id}/jira/test`                  | `JiraConnection` | `JiraConnectionTest` |
| GET    | `/api/items/{id}/jira`                            | None             | `JiraIssueState`     |
| POST   | `/api/items/{id}/jira/refresh`                    | None             | `JiraIssueState`     |
| GET    | `/api/items/{id}/jira/attachments/{attachmentId}` | None             | Streamed attachment  |

Issue state values are `not_configured`, `invalid_identifier`, `project_mismatch`, `not_found`, `available`, `authentication_failed`, `forbidden`, and `unavailable`. Expected absence states return HTTP 200 with their state; transport or internal contract failures use normal API errors.

## Adapter Contract

- `JiraClient.TestConnection` validates authentication and configured project.
- `JiraClient.GetIssue` requests only fields needed by the normalized DTO.
- `JiraClient.OpenAttachment` returns headers and a bounded stream.
- Cloud uses account email plus API token when required and parses Atlassian Document Format.
- Server/Data Center uses PAT bearer authentication and accepts supported plain/wiki/rendered description shapes.
- Base URLs require HTTPS except loopback addresses used by tests.
- HTTP clients apply connection and response timeouts and never follow redirects to another origin.

## Matching and Cache

- Trim and uppercase `ItemSummary.Identifier`.
- Require exact `^<PROJECT_KEY>-[1-9][0-9]*$` shape.
- Reject a different project prefix before calling Jira.
- Cache normalized successful results and not-found results by connection identity and issue key for five minutes.
- Refresh invalidates the key before fetching.
- Configuration updates and workspace deletion clear related cache entries.

## Attachment Controls

- Validate attachment ID against the normalized issue attachment list.
- Apply maximum size before streaming when Jira provides a length and enforce a streamed byte limit otherwise.
- Restrict inline preview to an explicit safe media-type allowlist; otherwise force `attachment` disposition.
- Sanitize filenames, set `X-Content-Type-Options: nosniff`, and do not forward Jira cookies, authorization, or arbitrary headers.
- Abort on timeout, cross-origin redirect, or mismatched response metadata.

## Design Decisions

| Decision                     | Rationale                                                  |
|------------------------------|------------------------------------------------------------|
| One connection per workspace | Matches the existing workspace-to-project requirement      |
| Environment token reference  | Keeps secret lifecycle outside Plan Manager                |
| Typed issue states           | Makes expected absence distinct from server errors         |
| Backend attachment proxy     | Keeps authentication and validation outside the browser    |
| No persistent Jira cache     | Avoids stale remote data becoming a second source of truth |
