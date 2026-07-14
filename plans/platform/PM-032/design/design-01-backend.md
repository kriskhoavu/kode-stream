# Backend Design: Cloud And Agent Modes

## Overview

The backend adds a runtime mode boundary around the existing Gin API. Local mode keeps current behavior. Cloud mode adds authentication, authorization, per-user path resolution, Cloud-safe workspace registration, and capability-aware route enforcement. Agent mode is designed as a local command runner that connects outbound to Cloud and executes user-machine operations without moving Git credentials into Cloud.

## Runtime Configuration

| Setting                          | Local Default   | Cloud Requirement             | Purpose                                          |
|----------------------------------|-----------------|-------------------------------|--------------------------------------------------|
| `KODE_STREAM_MODE`               | `local`         | `cloud`                       | Selects local, Cloud, or agent process behavior. |
| `KODE_STREAM_PORT`               | `4317`          | configured per deployment     | Keeps existing port behavior.                    |
| `KODE_STREAM_BIND_ADDR`          | `127.0.0.1`     | `0.0.0.0` or explicit address | Allows Cloud container/VM binding.               |
| `KODE_STREAM_DATA_DIR`           | OS config dir   | mounted persistent volume     | Stores app-owned state.                          |
| `KODE_STREAM_CLONE_ROOT`         | data clone root | mounted persistent volume     | Stores Cloud-managed repository clones.          |
| `KODE_STREAM_PUBLIC_URL`         | optional        | required                      | Builds OIDC redirects and agent connection URLs. |
| `KODE_STREAM_COOKIE_SECRET`      | unused          | required                      | Signs Cloud session cookies.                     |
| `KODE_STREAM_OIDC_ISSUER`        | unused          | required                      | Identity provider discovery.                     |
| `KODE_STREAM_OIDC_CLIENT_ID`     | unused          | required                      | OIDC client identity.                            |
| `KODE_STREAM_OIDC_CLIENT_SECRET` | unused          | required                      | OIDC client secret from deployment secrets.      |
| `KODE_STREAM_ADMIN_USERS`        | unused          | required for admin bootstrap  | Grants initial admin role by email or subject.   |

## Data Model

### Runtime Identity

| Field          | Type   | Purpose                                                          |
|----------------|--------|------------------------------------------------------------------|
| `mode`         | string | `local`, `cloud`, or `agent`.                                    |
| `user`         | object | Authenticated user profile in Cloud; trusted local user locally. |
| `role`         | string | `admin`, `editor`, or `viewer`.                                  |
| `capabilities` | map    | Feature availability by action and runtime.                      |

### Cloud User

| Field        | Type   | Purpose                                         |
|--------------|--------|-------------------------------------------------|
| `id`         | string | Stable provider subject or internal derived ID. |
| `email`      | string | Display and admin allowlist matching.           |
| `name`       | string | User-facing identity label.                     |
| `role`       | string | Effective role used by authorization policy.    |
| `createdAt`  | time   | First-seen timestamp.                           |
| `lastSeenAt` | time   | Recent session activity.                        |

### Workspace Location

| Field              | Type   | Purpose                                                     |
|--------------------|--------|-------------------------------------------------------------|
| `location`         | string | `local_path`, `cloud_clone`, or `agent_local`.              |
| `ownerUserId`      | string | Cloud owner for per-user state and Agent workspace routing. |
| `agentId`          | string | Live or remembered agent identity for Agent workspaces.     |
| `remoteUrl`        | string | Git URL used for Cloud clone or local agent remote context. |
| `cloudClonePath`   | string | Server-managed path for Cloud workspaces.                   |
| `publishedSummary` | bool   | Whether Agent metadata can be cached for Cloud display.     |

Existing `WorkspaceConfig` already has `remoteUrl`, `registrationMode`, and `clonePathManaged`. PM-032 should extend rather than replace it.

### Cloud Credential Reference

| Field         | Type   | Purpose                                                        |
|---------------|--------|----------------------------------------------------------------|
| `id`          | string | Stable reference stored on a workspace or user.                |
| `provider`    | string | GitHub, GitLab, Bitbucket, or generic token provider.          |
| `kind`        | string | OAuth/App installation, encrypted token, or encrypted SSH key. |
| `ownerUserId` | string | User or admin scope that owns the credential.                  |
| `createdAt`   | time   | Audit and rotation support.                                    |
| `rotatedAt`   | time   | Last rotation timestamp when available.                        |

Cloud v1 should prefer provider OAuth/App integration or encrypted token references. Raw SSH private key upload is a fallback design only, not the first implementation path.

### Agent Connection

| Field        | Type   | Purpose                                           |
|--------------|--------|---------------------------------------------------|
| `agentId`    | string | Stable local agent identity.                      |
| `userId`     | string | Cloud user who owns the connection.               |
| `name`       | string | Device label shown in UI.                         |
| `platform`   | string | macOS, Windows, or Linux.                         |
| `status`     | string | `offline`, `connecting`, `connected`, or `stale`. |
| `lastSeenAt` | time   | Connection freshness.                             |

## API Contract

### Runtime And Auth

| Method | Endpoint         | Request            | Response                                      |
|--------|------------------|--------------------|-----------------------------------------------|
| GET    | `/api/state`     | authenticated user | app version, mode, user, role, capabilities   |
| GET    | `/auth/login`    | redirect target    | redirects to OIDC provider                    |
| GET    | `/auth/callback` | OIDC callback      | validates user and creates session            |
| POST   | `/auth/logout`   | CSRF-protected     | clears Cloud session                          |
| GET    | `/api/health`    | none               | remains available for deployment healthchecks |

### Cloud Workspaces

| Method | Endpoint                         | Request                 | Response                                   |
|--------|----------------------------------|-------------------------|--------------------------------------------|
| GET    | `/api/workspaces`                | user session            | workspaces scoped to current user and role |
| POST   | `/api/workspaces`                | Git URL workspace input | workspace and optional clone operation log |
| POST   | `/api/workspaces/stream-create`  | Git URL workspace input | clone progress stream and final workspace  |
| POST   | `/api/workspaces/import-preview` | blocked in Cloud mode   | authorization or capability error          |
| POST   | `/api/system/select-directory`   | blocked in Cloud mode   | capability error                           |
| POST   | `/api/system/open-path`          | blocked in Cloud mode   | capability error                           |

### Agent

| Method | Endpoint                     | Request                         | Response                                           |
|--------|------------------------------|---------------------------------|----------------------------------------------------|
| POST   | `/api/agents/connect-token`  | current Cloud user              | short-lived deep-link token                        |
| GET    | `/api/agents`                | current Cloud user              | user-owned agent connection list                   |
| GET    | `/api/agents/:id/workspaces` | current Cloud user              | local workspaces advertised by the connected agent |
| POST   | `/api/agents/:id/workspaces` | local path registration request | command accepted or streamed agent result          |
| GET    | `/api/agents/channel`        | agent bearer token              | outbound WebSocket command channel                 |

Agent endpoints are design commitments for the follow-up phase. Cloud v1 can expose only discovery and unavailable states until agent runtime work starts.

## Authorization Policy

| Capability                     | Local | Cloud Viewer | Cloud Editor  | Cloud Admin | Agent Owner            |
|--------------------------------|-------|--------------|---------------|-------------|------------------------|
| Read board and items           | yes   | yes          | yes           | yes         | yes                    |
| Register Cloud Git URL         | yes   | no           | yes           | yes         | no                     |
| Register local path            | yes   | no           | no            | no          | yes, via agent         |
| File edit and metadata         | yes   | no           | yes           | yes         | yes, via agent         |
| Git status and diff            | yes   | yes          | yes           | yes         | yes, via agent         |
| Git commit, pull, push         | yes   | no           | yes           | yes         | yes, via agent         |
| Terminal and AI CLI            | yes   | no           | no            | yes         | yes, via agent policy  |
| Runtime and verification start | yes   | no           | no            | yes         | yes, via agent policy  |
| Workspace delete               | yes   | no           | own workspace | yes         | own workspace metadata |
| Integration settings           | yes   | no           | no            | yes         | owner-local only       |

The API must enforce this policy independently from frontend state.

## Storage Layout

Cloud mode must resolve paths through a user-scoped path service rather than the current single global `ResolvePaths` result.

| Path Type       | Cloud Layout                                   | Purpose                             |
|-----------------|------------------------------------------------|-------------------------------------|
| User registry   | `<data-root>/users/<user-id>/workspaces.yaml`  | Private workspace registry.         |
| User index      | `<data-root>/users/<user-id>/item-index.yaml`  | Private item index.                 |
| User audit      | `<data-root>/users/<user-id>/audit-log.jsonl`  | Private audit history.              |
| User clone root | `<clone-root>/users/<user-id>/<workspace-id>/` | Server-managed repository clone.    |
| Shared registry | `<data-root>/shared/workspaces.yaml`           | Optional admin-managed team spaces. |
| Agent registry  | `<data-root>/users/<user-id>/agents.yaml`      | Agent metadata and last-seen state. |

Use filesystem-safe derived user IDs; do not use raw email as a directory name.

## Cloud Workspace Rules

- `local_path` and `existing_workspace` registration modes are rejected in Cloud mode.
- Cloud registration accepts Git URL and configured source paths only.
- Clone destination is allocated by the server and never accepted from request input.
- Git URL parsing must reject local paths, `file://`, relative paths, and ambiguous path-like values.
- Deleting a Cloud workspace may delete managed clones after explicit confirmation.
- Cloud clone operations must capture progress logs without exposing credential values.

## Agent Security Boundary

- Agent opens outbound WebSocket to Cloud; Cloud does not call the user machine directly.
- Agent authenticates with a short-lived connect token created by the logged-in Cloud user.
- Agent commands are scoped to that user and that agent.
- Agent validates local paths and Git roots locally.
- Agent never returns SSH keys, Git credential helper output, environment secrets, or arbitrary host files.
- Terminal and AI CLI streams are proxied only after explicit user action and policy checks.

## Error Handling

Use existing typed error mapping where possible.

| Condition                          | Code           | Status |
|------------------------------------|----------------|--------|
| Missing Cloud session              | `unauthorized` | 401    |
| Role lacks capability              | `forbidden`    | 403    |
| Feature unavailable in mode        | `unavailable`  | 503    |
| Cloud registration contains path   | `validation`   | 400    |
| Agent offline                      | `unavailable`  | 503    |
| Clone or Git provider auth failure | `unauthorized` | 401    |

## Design Decisions

| Decision                                    | Rationale                                                                      |
|---------------------------------------------|--------------------------------------------------------------------------------|
| Add middleware at Gin boundary              | PM-031 made Gin the only API router, so route policy belongs at this boundary. |
| Keep Local mode unauthenticated             | Current local workflow is trusted and should remain fast.                      |
| Scope Cloud state per user                  | Prevents accidental workspace and index sharing.                               |
| Design Agent as an outbound command channel | Works behind NAT/firewalls and keeps user credentials local.                   |
| Treat Agent mode as designed but phased     | Cloud mode can ship first without forcing credential-local execution yet.      |
