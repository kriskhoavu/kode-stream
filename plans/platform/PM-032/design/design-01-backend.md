# Backend Design: Cloud Mode With Local Agent Execution

## Overview

The backend supports two runtime modes: Local and Cloud. Local mode keeps trusted user-machine behavior. Cloud mode adds
trusted-proxy identity, authorization, user state, agent connection management, and command routing. OAuth2Proxy owns the
public login boundary and redirects to Keycloak. Kode Stream stays behind that proxy, reads trusted identity headers, and
does not need to understand Keycloak directly for the default deployment. Workspace files, Git commands, terminal
sessions, AI CLI sessions, scanning, and verification commands execute only through the user's connected Cloud Agent.
The Cloud Agent is a native local process backed by the `internal/agent` domain package; it is not a hosted runtime mode
and does not duplicate Kode Stream's existing local services.

## Runtime Configuration

| Setting                          | Local Default | Cloud Requirement             | Purpose                                                 |
|----------------------------------|---------------|-------------------------------|---------------------------------------------------------|
| `KODE_STREAM_MODE`               | `local`       | `cloud`                       | Selects Local or Cloud server behavior.                 |
| `KODE_STREAM_AUTH_MODE`          | `local`       | `oauth2_proxy` by default     | Trusts OAuth2Proxy identity headers in Cloud mode.      |
| `KODE_STREAM_PORT`               | `4317`        | configured per deployment     | Keeps the API and embedded frontend reachable.          |
| `KODE_STREAM_BIND_ADDR`          | `127.0.0.1`   | `0.0.0.0` or explicit address | Allows hosted deployment binding on a private app port. |
| `KODE_STREAM_DATA_DIR`           | OS config dir | mounted persistent volume     | Stores Cloud users, workspace metadata, audit.          |
| `KODE_STREAM_PUBLIC_URL`         | optional      | required                      | Builds browser and agent connection URLs.               |
| `KODE_STREAM_COOKIE_SECRET`      | unused        | required                      | Signs fallback Cloud session cookies.                   |
| `KODE_STREAM_OIDC_ISSUER`        | unused        | only for `app_oidc`           | Identity provider discovery when Kode Stream owns OIDC. |
| `KODE_STREAM_OIDC_CLIENT_ID`     | unused        | only for `app_oidc`           | OIDC client identity when Kode Stream owns OIDC.        |
| `KODE_STREAM_OIDC_CLIENT_SECRET` | unused        | only for `app_oidc`           | OIDC client secret when Kode Stream owns OIDC.          |
| `KODE_STREAM_ADMIN_USERS`        | unused        | required for admin bootstrap  | Grants initial admin role by email or subject.          |

## Data Model

### Runtime Identity

| Field          | Type   | Purpose                                                            |
|----------------|--------|--------------------------------------------------------------------|
| `mode`         | string | `local` or `cloud`.                                                |
| `user`         | object | Authenticated Cloud user, or trusted local user context.           |
| `role`         | string | `admin`, `editor`, or `viewer`.                                    |
| `capabilities` | map    | Feature availability by role, runtime, workspace, and agent state. |

### Cloud User

| Field        | Type   | Purpose                                                               |
|--------------|--------|-----------------------------------------------------------------------|
| `id`         | string | Stable subject/header value or internal derived ID.                   |
| `email`      | string | Display and admin allowlist matching from forwarded identity headers. |
| `name`       | string | User-facing identity label.                                           |
| `role`       | string | Effective role used by authorization policy.                          |
| `createdAt`  | time   | Account creation timestamp.                                           |
| `lastSeenAt` | time   | Session activity timestamp.                                           |

### Cloud Agent Connection

| Field        | Type   | Purpose                                           |
|--------------|--------|---------------------------------------------------|
| `agentId`    | string | Stable user-machine agent identity.               |
| `userId`     | string | Cloud user who owns the agent.                    |
| `name`       | string | Device label shown in UI.                         |
| `platform`   | string | macOS, Windows, or Linux.                         |
| `status`     | string | `offline`, `connecting`, `connected`, or `stale`. |
| `lastSeenAt` | time   | Connection freshness.                             |

### Cloud Workspace

| Field              | Type   | Purpose                                                        |
|--------------------|--------|----------------------------------------------------------------|
| `id`               | string | Stable workspace identifier.                                   |
| `ownerUserId`      | string | Cloud user who owns the workspace metadata and agent routing.  |
| `agentId`          | string | Agent that owns local workspace execution.                     |
| `name`             | string | User-facing workspace name.                                    |
| `remoteUrl`        | string | Git remote reported by the agent when available.               |
| `localRootLabel`   | string | Redacted path label safe for UI display.                       |
| `publishedSummary` | bool   | Whether Cloud can show cached metadata while agent is offline. |

Cloud stores workspace metadata and optional published summaries. Cloud does not store local root paths as executable
server paths and does not store repository contents.

## API Contract

### Runtime And Auth

| Method | Endpoint         | Request                  | Response                                    |
|--------|------------------|--------------------------|---------------------------------------------|
| GET    | `/api/state`     | proxy-authenticated user | app version, mode, user, role, capabilities |
| GET    | `/auth/login`    | app-owned OIDC mode only | redirects to OIDC provider                  |
| GET    | `/auth/callback` | app-owned OIDC mode only | validates user and creates session          |
| POST   | `/auth/logout`   | proxy or app session     | proxy sign-out hint or clears app session   |
| GET    | `/api/health`    | none                     | deployment healthcheck                      |

In the default `oauth2_proxy` auth mode, the middleware accepts trusted identity headers such as
`X-Auth-Request-User`, `X-Auth-Request-Email`, `X-Forwarded-User`, and `X-Forwarded-Email`. Token introspection or JWT
validation inside Kode Stream is optional for a later hardening ticket and is not required for PM-032.

### Cloud Agent

| Method | Endpoint                    | Request            | Response                           |
|--------|-----------------------------|--------------------|------------------------------------|
| POST   | `/api/agents/connect-token` | current Cloud user | short-lived deep-link token        |
| GET    | `/api/agents`               | current Cloud user | user-owned agent connection list   |
| GET    | `/api/agents/channel`       | agent bearer token | outbound WebSocket command channel |

Agent connections use `wss://` over the Cloud public URL. The hosted API never requires inbound network reachability to
the user machine. Reusable frame contracts live in the agent domain; token signing, role policy, channel upgrade, and
hosted metadata ownership stay in the Cloud API package.

### Workspaces

| Method | Endpoint                       | Request                         | Response                                   |
|--------|--------------------------------|---------------------------------|--------------------------------------------|
| GET    | `/api/workspaces`              | user session                    | workspaces scoped to current user and role |
| POST   | `/api/workspaces/from-agent`   | agent workspace registration    | workspace metadata                         |
| GET    | `/api/workspaces/:id/status`   | user session                    | workspace and agent availability           |
| POST   | `/api/workspaces/:id/commands` | command envelope                | accepted command or streamed result        |
| POST   | `/api/system/select-directory` | blocked in Cloud mode           | capability error                           |
| POST   | `/api/system/open-path`        | routed through owner agent only | streamed agent result                      |

Cloud workspace registration starts from a connected agent. The browser asks Cloud for a connect token, the native local
agent validates the token, the user chooses a local repository through native UI, and the agent sends workspace metadata
to Cloud. The first implemented local runtime publishes metadata from `kode-stream agent start --repo`; full native
folder picking and command streaming are follow-on agent phases.

## Authorization Policy

| Capability                     | Local | Cloud Viewer | Cloud Editor       | Cloud Admin        |
|--------------------------------|-------|--------------|--------------------|--------------------|
| Read board and cached metadata | yes   | yes          | yes                | yes                |
| Connect own Cloud Agent        | no    | yes          | yes                | yes                |
| Register own local workspace   | yes   | no           | yes, through agent | yes, through agent |
| File edit and metadata         | yes   | no           | yes, through agent | yes, through agent |
| Git status and diff            | yes   | yes          | yes, through agent | yes, through agent |
| Git commit, pull, push         | yes   | no           | yes, through agent | yes, through agent |
| AI Session Terminal            | yes   | no           | yes, through agent | yes, through agent |
| Runtime and verification start | yes   | no           | yes, through agent | yes, through agent |
| Workspace delete               | yes   | no           | own workspace      | yes                |
| Integration settings           | yes   | no           | no                 | yes                |

The API enforces this policy independently from frontend state. Command-capable routes also require a connected owner
agent.

## Storage Layout

Cloud v1 uses file-backed metadata under `KODE_STREAM_DATA_DIR`. It does not require Postgres, Redis, or another database
service. Keep storage behind repository interfaces so a database can be introduced when Cloud needs multi-instance
hosting, complex queries, or stronger concurrent write guarantees.

| Path Type          | Cloud Layout                                  | Purpose                                  |
|--------------------|-----------------------------------------------|------------------------------------------|
| User metadata      | `<data-root>/users/<user-id>/profile.json`    | Cloud user profile and role state.       |
| Workspace registry | `<data-root>/users/<user-id>/workspaces.yaml` | Workspace metadata and agent ownership.  |
| Agent registry     | `<data-root>/users/<user-id>/agents.yaml`     | Agent identity and connection state.     |
| Audit log          | `<data-root>/users/<user-id>/audit-log.jsonl` | Auth, routing, and command audit events. |
| Published cache    | `<data-root>/users/<user-id>/cache/`          | Optional summaries safe for offline UI.  |

Use filesystem-safe derived user IDs. Do not use raw email as a directory name.

## Cloud Workspace Rules

- Cloud workspace registration requires a connected Cloud Agent.
- Cloud rejects raw local paths submitted directly from the browser.
- Cloud rejects Git URL registration that asks the hosted server to clone a repository.
- Cloud stores redacted labels and metadata, not executable local paths.
- Cloud accepts file, Git, terminal, AI, runtime, and verification requests only for the connected owner agent and
  emits scoped command envelopes for the local command bridge.
- Cloud returns `unavailable` when the owner agent is offline.
- Cloud must route every workspace command through the owner agent once the command bridge is enabled; until then,
  hosted execution remains blocked.

## Agent Security Boundary

- Agent opens outbound WebSocket to Cloud; Cloud does not initiate inbound connections to the user machine.
- Agent authenticates with a short-lived connect token created by the logged-in Cloud user.
- Agent upgrades to a rotated agent credential after pairing so reconnect does not require a browser every time.
- Agent commands are scoped to the user, agent, workspace, and capability.
- Agent validates local paths and Git roots locally.
- Agent prompts or policy-checks command-capable operations on the user machine.
- Agent never returns SSH keys, Git credential helper output, environment secrets, or arbitrary host files.
- Terminal and AI CLI streams are proxied only with explicit owner action and policy checks.

## Network Model

| Option                   | Backend Position                                                    |
|--------------------------|---------------------------------------------------------------------|
| Outbound HTTPS WebSocket | Supported default for agent control, terminal, and command streams. |
| Tailscale or VPN         | Allowed deployment layer when operators want private Cloud access.  |
| Port forwarding          | Local testing only, not product behavior.                           |

The WebSocket channel carries typed command envelopes, heartbeats, stream frames, and cancellation messages. Reverse
proxy configuration must support WebSocket upgrade and idle timeouts long enough for terminal and AI sessions.

## Error Handling

| Condition                    | Code           | Status |
|------------------------------|----------------|--------|
| Missing proxy identity       | `unauthorized` | 401    |
| Role lacks capability        | `forbidden`    | 403    |
| Agent offline                | `unavailable`  | 503    |
| Browser submits local path   | `validation`   | 400    |
| Workspace has no owner agent | `unavailable`  | 503    |
| Agent rejects command        | `forbidden`    | 403    |
| Agent command fails          | `agent_error`  | 502    |

## Design Decisions

| Decision                                  | Rationale                                                                                                      |
|-------------------------------------------|----------------------------------------------------------------------------------------------------------------|
| Add middleware at Gin boundary            | PM-031 made Gin the API router, so route policy belongs at this boundary.                                      |
| Keep Local mode unauthenticated           | Local workflow is trusted and single-user.                                                                     |
| Make Cloud Agent required for workspaces  | Cloud must not own repository source trees or user credentials.                                                |
| Route command envelopes through the agent | Workspace command execution belongs on the user machine and is bridged by the native `internal/agent` adapter. |
| Use outbound agent WebSocket              | Avoids inbound ports, port forwarding, and direct Cloud access to user hosts.                                  |
| Use file-backed metadata for Cloud v1     | Keeps deployment simple while preserving a repository boundary for later DBs.                                  |
| Store metadata in Cloud                   | Cloud can support UI, status, audit, and collaboration without source hosting.                                 |
| Default to OAuth2Proxy auth               | Kode Stream stays independent of Keycloak and trusts the private proxy boundary.                               |
