# Implementation Plan: PM-032 - Cloud And Agent Modes

## Overview

Implement runtime mode support with Cloud mode first. Cloud mode adds hosted authentication, per-user storage, Git URL-only workspace registration, role/capability enforcement, and VM/container deployment. Agent mode is designed and then introduced as a later track so user-owned local repositories can stay on the user machine without moving SSH keys or Git credentials into Cloud.

## Terminology Lock

All code, fields, API params, and TS types must use:

- `RuntimeMode`
- `local`, `cloud`, `agent`
- `CloudWorkspace`
- `AgentWorkspace`
- `WorkspaceLocation`
- `Capability`
- `CloudUser`
- `AgentConnection`
- `cloud_clone`
- `agent_local`

Avoid:

- `remote mode` for Cloud mode.
- `desktop mode` for Agent mode.
- `project` for workspaces.
- `sync` when the behavior is scan, clone, or publish.

## Phases Summary

| Phase | Name                                            | Track    | Status  |
|-------|-------------------------------------------------|----------|---------|
| B1    | Runtime mode and capability model               | Backend  | Pending |
| B2    | Cloud auth, roles, and route policy             | Backend  | Pending |
| B3    | Cloud per-user storage and Git URL registration | Backend  | Pending |
| B4    | Cloud command and credential safety             | Backend  | Pending |
| F1    | Runtime state and shared frontend types         | Frontend | Pending |
| F2    | Cloud workspace registration UX                 | Frontend | Pending |
| F3    | Role-aware feature gating                       | Frontend | Pending |
| C1    | Cloud container and VM deployment               | DevOps   | Pending |
| C2    | Cloud release documentation and smoke checks    | DevOps   | Pending |
| B5    | Agent connection backend foundation             | Backend  | Pending |
| F4    | Agent connect and offline UX                    | Frontend | Pending |
| C3    | macOS agent packaging plan and smoke            | DevOps   | Pending |

## Backend Phases

### Phase B1: Runtime Mode And Capability Model

**Deliverables:**

- [ ] Add runtime mode config resolver for `local`, `cloud`, and `agent`.
- [ ] Add bind address handling while preserving local default `127.0.0.1`.
- [ ] Extend app state response with mode, user, role, and capability map.
- [ ] Add workspace location model fields while preserving existing workspace compatibility.
- [ ] Add tests for mode defaults, invalid mode rejection, bind defaults, and app state shape.

**Verification:** `rtk go test ./internal/system/... ./internal/server/... ./internal/workspace/...`

**Commit:** `PM-032: Add runtime mode capability model`

---

### Phase B2: Cloud Auth, Roles, And Route Policy

**Deliverables:**

- [ ] Add Cloud session middleware at the Gin API boundary.
- [ ] Add OIDC login, callback, logout, and session cookie handling.
- [ ] Add admin bootstrap from configured user allowlist.
- [ ] Add role policy for viewer, editor, and admin.
- [ ] Enforce policy on representative read, write, Git, system, terminal, AI, runtime, and verification routes.
- [ ] Add CSRF protection for Cloud mutating requests.
- [ ] Add tests for unauthenticated, viewer, editor, admin, CSRF, and WebSocket access behavior.

**Verification:** `rtk go test ./internal/server/api/... ./internal/common/...`

**Commit:** `PM-032: Add cloud auth and route policy`

---

### Phase B3: Cloud Per-User Storage And Git URL Registration

**Deliverables:**

- [ ] Add Cloud path resolver for per-user registry, indexes, audit, settings, and clone roots.
- [ ] Route workspace services through user-scoped storage in Cloud mode.
- [ ] Restrict Cloud registration to Git URL workspaces.
- [ ] Reject local paths, `file://`, relative paths, and existing workspace import in Cloud mode.
- [ ] Allocate clone destination server-side under the current user's clone root.
- [ ] Preserve streaming clone progress while redacting credentials.
- [ ] Add tests for per-user isolation, Cloud registration validation, clone path allocation, and import/system route blocking.

**Verification:** `rtk go test ./internal/workspace/... ./internal/server/api/... ./internal/system/...`

**Commit:** `PM-032: Add cloud workspace registration`

---

### Phase B4: Cloud Command And Credential Safety

**Deliverables:**

- [ ] Add credential reference model and provider abstraction for Cloud Git operations.
- [ ] Support first Cloud credential path selected during implementation, preferring provider OAuth/App or encrypted token over SSH key upload.
- [ ] Ensure Git clone/fetch/pull/push use credential references without logging secret values.
- [ ] Gate terminal, AI CLI, runtime command execution, and verification start to Cloud admin.
- [ ] Preserve editor access to guarded file edits and Git operations when credentials are valid.
- [ ] Add tests for credential redaction, role denials, Git operation authorization, and command route gating.

**Verification:** `rtk go test ./internal/git/... ./internal/ai/... ./internal/runtime/... ./internal/verification/... ./internal/server/api/...`

**Commit:** `PM-032: Harden cloud git and command execution`

---

### Phase B5: Agent Connection Backend Foundation

**Deliverables:**

- [ ] Add agent connect-token endpoint for authenticated Cloud users.
- [ ] Add agent metadata store scoped per Cloud user.
- [ ] Add outbound agent WebSocket channel with authenticated agent identity.
- [ ] Add command envelope model for scan, file, Git, terminal, AI, runtime, and verification requests.
- [ ] Add unavailable responses for Agent workspaces when no live connection exists.
- [ ] Add tests for token expiry, user scoping, offline behavior, and command authorization.

**Verification:** `rtk go test ./internal/server/api/... ./internal/workspace/...`

**Commit:** `PM-032: Add agent connection foundation`

## Frontend Phases

### Phase F1: Runtime State And Shared Frontend Types

**Deliverables:**

- [ ] Add TypeScript types for runtime mode, user, role, capabilities, workspace location, and agent status.
- [ ] Extend API state normalization to include runtime context.
- [ ] Extend `useAppState` to expose runtime context without changing existing local route behavior.
- [ ] Add tests for local fallback state and Cloud state normalization.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/app web/src/shared web/src/lib`

**Commit:** `PM-032: Add runtime state frontend types`

---

### Phase F2: Cloud Workspace Registration UX

**Deliverables:**

- [ ] Update `WorkspacesPage` to render Git URL-only registration in Cloud mode.
- [ ] Hide or disable local path picker, path reveal, drag-and-drop path, and existing workspace import in Cloud mode.
- [ ] Show Git credential/provider status and recovery hints.
- [ ] Reuse streaming create UI for Cloud clone progress.
- [ ] Add tests for Cloud registration controls, hidden local fields, and error states.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/pages/WorkspacesPage`

**Commit:** `PM-032: Add cloud workspace registration UI`

---

### Phase F3: Role-Aware Feature Gating

**Deliverables:**

- [ ] Gate item workspace file, metadata, Git, terminal, AI, runtime, and verification controls by server capability map.
- [ ] Add signed-in user, role, mode label, and logout surface in Cloud mode.
- [ ] Add workspace location badges for Local, Cloud clone, and Agent local.
- [ ] Preserve current local mode UX with all existing local actions.
- [ ] Add tests for viewer, editor, admin, local, Cloud unavailable, and Agent offline states.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/pages web/src/features`

**Commit:** `PM-032: Add role-aware cloud feature gating`

---

### Phase F4: Agent Connect And Offline UX

**Deliverables:**

- [ ] Add connect local workspace flow that requests a short-lived agent connect token.
- [ ] Add `kodestream://connect` launch behavior and reconnect action.
- [ ] Add install guidance for macOS Homebrew first, with Windows and Linux marked planned.
- [ ] Add Agent workspace offline, connecting, connected, and unavailable states.
- [ ] Add tests for agent not installed, offline workspace, and connected workspace labels.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/pages web/src/features/workspaces`

**Commit:** `PM-032: Add agent connection UI`

## DevOps Phases

### Phase C1: Cloud Container And VM Deployment

**Deliverables:**

- [ ] Add `Dockerfile` with frontend build, Go build, non-root runtime, Git, and CA certificates.
- [ ] Add `.dockerignore`.
- [ ] Add compose example for VM deployment with data and clone root volumes.
- [ ] Add healthcheck using `/api/health`.
- [ ] Add startup validation for required Cloud env vars.
- [ ] Add local container smoke instructions.

**Verification:** `rtk npm run build && rtk go build -o ./bin/kode-stream ./cmd/kode-stream` plus container build and `/api/health` smoke.

**Commit:** `PM-032: Add cloud container deployment`

---

### Phase C2: Cloud Release Documentation And Smoke Checks

**Deliverables:**

- [ ] Add Cloud deployment guide covering OIDC, reverse proxy TLS, env vars, volumes, backups, upgrades, and rollback.
- [ ] Update README and architecture docs with Local, Cloud, and Agent mode summaries.
- [ ] Add troubleshooting for OIDC failures, Git credential failures, clone failures, and role denials.
- [ ] Add release checklist entries for Cloud image and deployment smoke.

**Verification:** `rtk go test ./... && rtk npm run typecheck`

**Commit:** `PM-032: Document cloud deployment mode`

---

### Phase C3: macOS Agent Packaging Plan And Smoke

**Deliverables:**

- [ ] Add `kode-stream agent` CLI command skeleton with `start`, `status`, and `doctor`.
- [ ] Add macOS Homebrew packaging plan or formula update for the agent path.
- [ ] Add deep-link registration plan for `kodestream://connect`.
- [ ] Add Windows and Linux packaging notes as planned follow-up, not Cloud blockers.
- [ ] Add local smoke for `agent doctor` and Cloud reachability.

**Verification:** focused Go tests for CLI parsing and manual macOS Homebrew smoke when packaging is active.

**Commit:** `PM-032: Add macOS agent packaging foundation`

## Post-Implementation Checklist

- [ ] Local mode remains backward-compatible and loopback by default.
- [ ] Cloud mode accepts Git URL workspaces only.
- [ ] Cloud users cannot register arbitrary server filesystem paths.
- [ ] Cloud state and clones are scoped per user unless explicitly shared.
- [ ] User SSH keys and Git credential helper output are not stored by Kode Stream for Agent workspaces.
- [ ] Terminal, AI CLI, runtime, and verification start are Cloud admin-only.
- [ ] Agent mode is represented in API/UI capabilities even before full agent execution ships.
- [ ] Docs explain macOS-first agent install and Windows/Linux follow-up.
