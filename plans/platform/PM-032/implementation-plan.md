# Implementation Plan: PM-032 - Cloud Mode With Local Agent Execution

## Overview

Implement two runtime modes: Local and Cloud. Cloud mode provides hosted authentication, role policy, metadata storage,
workspace UI, and command routing. The Cloud Agent is a required Cloud component for repository access, Git operations,
terminal, AI CLI, scanning, runtime commands, and verification. The hosted Cloud VM never clones repositories and never
executes workspace commands.

## Release Rule

Cloud mode is releasable when the hosted app, Cloud Agent connection, local workspace registration, and agent-routed
command flow work together. Cloud workspaces require a connected owner agent for command-capable actions.

## Terminology Lock

All code, fields, API params, and TS types must use:

- `RuntimeMode`
- `local`, `cloud`
- `CloudAgent`
- `AgentConnection`
- `CloudWorkspace`
- `WorkspaceLocation`
- `cloud_agent`
- `Capability`
- `CloudUser`

Avoid:

- `agent` as a runtime mode.
- Runtime names beyond `local` and `cloud`.
- Hosted workspace execution names.
- Hosted repository storage names.
- Database service requirement for Cloud v1.
- `project` for workspaces.
- `sync` when the behavior is scan, route, or publish.

## Phases Summary

| Phase | Name                                     | Track    | Status  |
|-------|------------------------------------------|----------|---------|
| B1    | Runtime mode and capability model        | Backend  | Done    |
| B2    | Cloud auth, roles, and route policy      | Backend  | Done    |
| B3    | Cloud Agent connection foundation        | Backend  | Done    |
| B4    | Agent-backed workspace API foundation    | Backend  | Done    |
| B5    | Agent command routing API boundary       | Backend  | Done    |
| F1    | Runtime state and shared frontend types  | Frontend | Done    |
| F2    | Cloud Agent connection UX                | Frontend | Done    |
| F3    | Agent-backed workspace UX                | Frontend | Done    |
| F4    | Role-aware command and offline UX        | Frontend | Done    |
| C1    | Cloud container and metadata deployment  | DevOps   | Done    |
| C2    | Cloud Agent packaging and install docs   | DevOps   | Done    |
| C3    | Release documentation and smoke checks   | DevOps   | Done    |
| C4    | OAuth2Proxy cloud auth boundary          | DevOps   | Done    |
| C5    | Local OAuth2Proxy and Keycloak stack     | DevOps   | Done    |
| A1    | Cloud Agent domain and CLI runtime       | Agent    | Done    |
| A2    | Agent connection, heartbeat, local smoke | Agent    | Done    |
| A3    | Agent workspace metadata publish         | Agent    | Done    |
| A4    | Agent command dispatch bridge            | Agent    | Planned |
| A5    | Agent packaging, deep link, reconnect    | Agent    | Planned |

## Backend Phases

### Phase B1: Runtime Mode And Capability Model

**Deliverables:**

- [x] Add runtime mode config resolver for `local` and `cloud`.
- [x] Add bind address handling while preserving local default `127.0.0.1`.
- [x] Extend app state response with mode, user, role, capability map, and agent availability.
- [x] Add workspace location model for `local_path` and `cloud_agent`.
- [x] Add tests for mode defaults, invalid mode rejection, bind defaults, and app state shape.

**Verification:** `rtk go test ./internal/system/... ./internal/server/... ./internal/workspace/...`

**Commit:** `PM-032: Add cloud runtime capability model`

---

### Phase B2: Cloud Auth, Roles, And Route Policy

**Deliverables:**

- [x] Add Cloud session middleware at the Gin API boundary.
- [x] Add OIDC login, callback, logout, and session cookie handling.
- [x] Add admin bootstrap from configured user allowlist.
- [x] Add role policy for viewer, editor, and admin.
- [x] Enforce policy on read, write, Git, system, terminal, AI, runtime, and verification routes.
- [x] Add CSRF protection for Cloud mutating requests.
- [x] Add tests for unauthenticated, viewer, editor, admin, CSRF, and WebSocket access behavior.

**Verification:** `rtk go test ./internal/server/api/... ./internal/common/...`

**Commit:** `PM-032: Add cloud auth and route policy`

---

### Phase B3: Cloud Agent Connection Foundation

**Deliverables:**

- [x] Add Cloud Agent connect-token endpoint for authenticated Cloud users.
- [x] Add agent metadata store scoped per Cloud user.
- [x] Add outbound agent WebSocket channel with authenticated agent identity.
- [x] Use outbound HTTPS WebSocket to the Cloud public URL; do not require port forwarding or inbound user-machine
  access.
- [x] Add reverse proxy requirements for WebSocket upgrade, auth headers, and long idle timeouts.
- [x] Add agent status, heartbeat, reconnect, and stale detection.
- [x] Add tests for token expiry, user scoping, connection state, and WebSocket authorization.

**Verification:** `rtk go test ./internal/server/api/... ./internal/workspace/...`

**Commit:** `PM-032: Add cloud agent connection foundation`

---

### Phase B4: Agent-Backed Workspace API Foundation

**Deliverables:**

- [x] Add Cloud workspace registry for metadata, agent ownership, redacted path label, remote URL, and published
  summaries.
- [x] Add API contract for agent-owned workspace metadata publication.
- [x] Store agent-published Git root metadata without storing executable local paths.
- [x] Reject direct browser local paths and Git URL requests that ask Cloud to clone.
- [x] Add tests for metadata storage, agent ownership, direct path rejection, and offline workspace state.

**Verification:** `rtk go test ./internal/workspace/... ./internal/server/api/... ./internal/system/...`

**Commit:** `PM-032: Add agent-backed cloud workspace API foundation`

---

### Phase B5: Agent Command Routing API Boundary

**Deliverables:**

- [x] Add command envelope model for file, Git, terminal, AI, runtime, and verification requests.
- [x] Bind command envelopes to user, workspace, agent, role, and capability.
- [x] Accept command-capable requests only when the owner Cloud Agent is connected.
- [x] Return scoped command envelopes at the Cloud API boundary.
- [x] Redact streamed logs and errors from agent command responses.
- [x] Add tests for command authorization, agent offline behavior, credential redaction, and hosted execution denial.

**Verification:** `rtk go test ./internal/git/... ./internal/ai/... ./internal/runtime/... ./internal/verification/... ./internal/server/api/...`

**Commit:** `PM-032: Add cloud command routing API boundary`

The full command bridge that streams these envelopes through the native local agent and delegates to existing local
workspace, Git, file, terminal, AI, runtime, and verification services is Phase A4.

## Frontend Phases

### Phase F1: Runtime State And Shared Frontend Types

**Deliverables:**

- [x] Add TypeScript types for runtime mode, user, role, capabilities, workspace location, and agent status.
- [x] Extend API state normalization to include runtime and agent context.
- [x] Extend `useAppState` to expose runtime context without changing local route behavior.
- [x] Add tests for local fallback state and Cloud state normalization.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/app web/src/shared web/src/lib`

**Commit:** `PM-032: Add cloud runtime frontend types`

---

### Phase F2: Cloud Agent Connection UX

**Deliverables:**

- [x] Add connect local workspace flow that requests a short-lived agent connect token.
- [x] Add `kodestream://connect` launch behavior and reconnect action.
- [x] Add network recovery copy for Cloud reachability, WebSocket proxy issues, and optional VPN policy.
- [x] Add install guidance for macOS Homebrew first, with Windows and Linux marked planned.
- [x] Add connected, connecting, offline, stale, and unsupported states.
- [x] Add tests for agent not installed, offline agent, reconnect, and connected state labels.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/pages web/src/features/workspaces`

**Commit:** `PM-032: Add cloud agent connection UI`

---

### Phase F3: Agent-Backed Workspace UX

**Deliverables:**

- [x] Update `WorkspacesPage` to render Cloud Agent workspace registration in Cloud mode.
- [x] Hide direct local path fields, Git URL registration fields, path reveal, drag-and-drop path, and workspace import
  in Cloud mode.
- [x] Show agent device label, redacted local path label, remote URL, and scan status.
- [x] Render published metadata and board refresh when agent scan completes.
- [x] Add tests for Cloud Agent registration controls, hidden direct path fields, and error states.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/pages/WorkspacesPage`

**Commit:** `PM-032: Add agent-backed workspace UI`

---

### Phase F4: Role-Aware Command And Offline UX

**Deliverables:**

- [x] Gate item workspace file, metadata, Git, terminal, AI, runtime, and verification controls by server capability map
  and agent availability.
- [x] Add signed-in user, role, mode label, agent status, and logout surface in Cloud mode.
- [x] Add workspace labels for Local and Cloud Agent backed workspaces.
- [x] Preserve local mode UX with local actions.
- [x] Add tests for viewer, editor, admin, local, agent offline, and agent rejection states.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/pages web/src/features`

**Commit:** `PM-032: Add role-aware cloud command UI`

## DevOps Phases

### Phase C1: Cloud Container And Metadata Deployment

**Deliverables:**

- [x] Add `Dockerfile` with frontend build, Go build, non-root runtime, and CA certificates.
- [x] Add `.dockerignore`.
- [x] Add compose example for VM deployment with metadata data volume.
- [x] Document that Cloud v1 uses file-backed metadata and does not require a database service.
- [x] Add healthcheck using `/api/health`.
- [x] Add startup validation for required Cloud env vars.
- [x] Document that the Cloud deployment does not clone repositories or expose hosted terminal execution.
- [x] Add local container smoke instructions.

**Verification:** `rtk npm run build && rtk go build -o ./bin/kode-stream ./cmd/kode-stream` plus container build and `/api/health` smoke.

**Commit:** `PM-032: Add cloud metadata deployment`

---

### Phase C2: Cloud Agent Packaging And Install Docs

**Deliverables:**

- [x] Add `kode-stream agent` CLI command with `start`, `status`, and `doctor`.
- [x] Add macOS Homebrew packaging plan or formula update for the agent path.
- [x] Add deep-link registration plan for `kodestream://connect`.
- [x] Add Windows and Linux packaging notes as planned supported targets.
- [x] Add local smoke for `agent doctor`, deep link, Cloud reachability, and local repo scan.

**Verification:** focused Go tests for CLI parsing and manual macOS Homebrew smoke when packaging is active.

**Commit:** `PM-032: Add cloud agent packaging foundation`

---

### Phase C3: Release Documentation And Smoke Checks

**Deliverables:**

- [x] Add Cloud deployment guide covering OIDC, reverse proxy TLS, env vars, metadata volume, backups, upgrades, and
  rollback.
- [x] Update README and architecture docs with Local and Cloud mode summaries.
- [x] Add Cloud Agent install and reconnect guide.
- [x] Add troubleshooting for OIDC failures, agent connection failures, deep-link issues, role denials, WebSocket proxy
  issues, and optional VPN policy.
- [x] Add release checklist entries for Cloud image, Cloud Agent package, and agent-backed workspace smoke.

**Verification:** `rtk go test ./... && rtk npm run typecheck`

**Commit:** `PM-032: Document cloud mode`

---

### Phase C4: OAuth2Proxy Cloud Auth Boundary

**Deliverables:**

- [x] Default Cloud auth mode to `oauth2_proxy`.
- [x] Keep Kode Stream app port private in the compose example.
- [x] Expose OAuth2Proxy as the public browser entry point.
- [x] Trust OAuth2Proxy identity headers in Cloud middleware.
- [x] Keep app-owned OIDC available as `KODE_STREAM_AUTH_MODE=app_oidc`.
- [x] Document that token introspection or JWT validation inside Kode Stream is optional for a later hardening ticket.
- [x] Document local smoke versus full OAuth2Proxy/Keycloak login setup.

**Verification:** `rtk go test ./...` plus `rtk docker compose -f deploy/cloud/compose.yaml config`

**Commit:** `PM-032: Support oauth2 proxy cloud auth`

---

### Phase C5: Local OAuth2Proxy And Keycloak Stack

**Deliverables:**

- [x] Add Docker Compose stack for local Keycloak, OAuth2Proxy, and private Kode Stream app.
- [x] Add Keycloak `kode-stream` realm import with local admin, editor, and viewer users.
- [x] Keep Kode Stream reachable only through OAuth2Proxy in the local stack.
- [x] Add local run, healthcheck, login, stop, and reset instructions.
- [x] Verify OAuth2Proxy can reach Keycloak discovery and redirects browser login to Keycloak.

**Verification:** `rtk docker compose -f deploy/cloud/local-compose.yaml up -d --build` plus health and redirect smoke.

**Commit:** `PM-032: Add local cloud auth compose stack`

## Agent Runtime Phases

### Phase A1: Cloud Agent Domain And CLI Runtime

**Deliverables:**

- [x] Add `internal/agent` for connect-token parsing, deep-link parsing, channel URL construction, frame contracts,
  connection lifecycle, workspace metadata publishing, and command-dispatch interfaces.
- [x] Wire `kode-stream agent start --connect <deep-link-or-token> --cloud-url <url> --repo <path>` as a foreground
  local process.
- [x] Keep Cloud API code focused on hosted auth, tokens, agent channel endpoints, role policy, workspace metadata, and
  command routing boundaries.
- [x] Keep workspace, Git, file, terminal, AI, runtime, and verification behavior owned by existing local services.

**Verification:** `rtk go test ./internal/agent/... ./cmd/kode-stream/... ./internal/server/api/...`

**Commit:** `PM-032: Add cloud agent domain runtime`

---

### Phase A2: Agent Connection, Heartbeat, And Local Smoke

**Deliverables:**

- [x] Connect outbound to `/api/agents/channel` with a raw token or `kodestream://connect?token=...` link.
- [x] Convert `http`/`https` Cloud URLs to `ws`/`wss` channel URLs.
- [x] Read the connected frame and send heartbeat frames.
- [x] Handle heartbeat acknowledgements from Cloud.
- [x] Add local WebSocket smoke coverage for connection and heartbeat.

**Verification:** `rtk go test ./internal/agent/...`

**Commit:** `PM-032: Add cloud agent heartbeat smoke`

---

### Phase A3: Agent-Backed Workspace Metadata Publish

**Deliverables:**

- [x] Validate a local repo through the existing Git adapter.
- [x] Publish workspace metadata to `/api/workspaces/from-agent`.
- [x] Publish redacted-safe fields only: name, baseline branch, detected source roots, remote URL, root label, and scan
  status.
- [x] Add temp Git repo coverage for metadata publishing.

**Verification:** `rtk go test ./internal/agent/... ./internal/server/api/...`

**Commit:** `PM-032: Publish cloud agent workspace metadata`

---

### Phase A4: Agent Command Dispatch Bridge

**Deliverables:**

- [ ] Route Cloud command frames through `internal/agent` dispatch interfaces.
- [ ] Add adapters that call existing local workspace, Git, file, terminal, AI, runtime, and verification services.
- [ ] Stream command results, terminal output, cancellation, and errors back through the Cloud channel.
- [ ] Preserve existing path safety, write guards, Git guards, and runtime verification contracts.
- [ ] Add integration tests for file, Git, terminal, AI, runtime, and verification command envelopes.

**Verification:** `rtk go test ./internal/agent/... ./internal/server/api/... ./internal/git/... ./internal/ai/... ./internal/runtime/... ./internal/verification/...`

**Commit:** `PM-032: Bridge cloud commands to local services`

---

### Phase A5: Agent Packaging, Deep Link, Reconnect, And Docs

**Deliverables:**

- [ ] Add durable agent credentials after first pairing.
- [ ] Add reconnect backoff and credential refresh policy.
- [ ] Register `kodestream://connect` for macOS packaging, then Windows and Linux packages.
- [ ] Add background service packaging where appropriate.
- [ ] Update smoke docs for connected-agent and command-dispatch verification.

**Verification:** local compose smoke plus package-specific installer checks.

**Commit:** `PM-032: Package cloud agent reconnect flow`

## Post-Implementation Checklist

- [x] Local mode remains backward-compatible and loopback by default.
- [x] Cloud mode exposes hosted UI, auth, metadata, and command routing API boundaries.
- [x] Cloud v1 runs with a persistent metadata volume and no required database service.
- [x] Cloud workspace metadata publication is performed by a connected Cloud Agent.
- [x] Cloud users cannot register direct browser local paths.
- [x] Cloud does not clone repositories onto the hosted VM.
- [x] Cloud does not execute terminal, AI CLI, Git, runtime, or verification commands on the hosted VM.
- [x] User SSH keys and Git credential helper output are not stored by Cloud.
- [x] Agent command envelopes are scoped to user, workspace, agent, role, and capability at the Cloud API boundary.
- [x] Agent offline state disables command-capable controls.
- [x] Docs explain Local mode and Cloud mode without describing Agent as a runtime mode.
