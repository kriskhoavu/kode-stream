# Implementation Plan: PM-034 - Chrome Extension Local Showcase

## Overview

Implement a local unpacked Chrome extension showcase for Kode Stream and an agentless Cloud remote-snapshot workspace.
The extension bundles the existing React app and routes API calls to the local server. The Cloud path reads an approved
Git-provider snapshot without a Cloud Agent. A common workspace access interface keeps the existing PM-032 agent-backed
command path separate from the new provider-read path.

## Terminology Lock

All code, fields, API params, and docs must use:

- `Extension Surface`
- `Local API Origin`
- `API Origin Adapter`
- `Unpacked Extension`
- `Chrome Extension Showcase`
- `WorkspaceAccessMode`
- `agent_backed`
- `remote_snapshot`
- `WorkspaceAccessAdapter`
- `RemoteSnapshotAdapter`
- `AgentAccessAdapter`
- `GitProviderIntegration`
- `TerminalHandoff`

Avoid:

- `pure extension`
- `direct Git in Chrome`
- `file URL mode`
- `native messaging`
- `Chrome app`
- `agentless Git`
- `Cloud Git command`
- `remote clone`

## Phases Summary

| Phase | Name                                    | Track    | Status  |
|-------|-----------------------------------------|----------|---------|
| F1    | API origin adapter                      | Frontend | Done    |
| F2    | Extension surface behavior              | Frontend | Done    |
| C1    | Extension build artifact                | DevOps   | Done    |
| C2    | Showcase verification and documentation | DevOps   | Done    |
| B1    | Cloud workspace access adapters         | Backend  | Done    |
| B2    | Provider remote snapshot adapter        | Backend  | Planned |
| B3    | Snapshot read and capability API        | Backend  | Planned |
| F3    | Agentless workspace registration        | Frontend | Planned |
| F4    | Snapshot workspace capability UI        | Frontend | Planned |
| C3    | Provider authorization and Cloud smoke  | DevOps   | Planned |

## Frontend Phases

### Phase F1: API Origin Adapter

**Deliverables:**

- [x] Add a shared API URL resolver used by all frontend API requests.
- [x] Keep relative `/api/*` behavior for localhost and Vite dev surfaces.
- [x] Resolve extension-surface API calls to `localStorage.kodeStreamApiOrigin` or `http://127.0.0.1:4317`.
- [x] Update direct fetch helpers and attachment URLs to use the resolver.
- [x] Add focused tests for URL resolution and representative Files + Git endpoints.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/shared/api/index.test.ts`

**Commit:** `PM-034: Add extension API origin adapter`

---

### Phase F2: Extension Surface Behavior

**Deliverables:**

- [x] Detect the extension surface without affecting normal local web mode.
- [x] Add a local-server health check state for extension startup.
- [x] Show a clear unavailable state when the configured local API origin is not reachable.
- [x] Hide or disable embedded terminal/AI streaming controls in extension mode.
- [x] Add component tests for unavailable state and unsupported streaming controls.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Commit:** `PM-034: Add extension surface guards`

---

## DevOps Phases

### Phase C1: Extension Build Artifact

**Deliverables:**

- [x] Add an extension Vite build mode with relative asset paths and separate output directory.
- [x] Add an MV3 manifest for local unpacked loading.
- [x] Add `npm run build:extension`.
- [x] Ensure the normal `npm run build` output for Go embed remains unchanged.
- [x] Confirm the extension artifact can be loaded with Chrome Developer Mode.

**Verification:** `rtk npm run build && rtk npm run build:extension`

**Commit:** `PM-034: Add unpacked extension build`

---

### Phase C2: Showcase Verification And Documentation

**Deliverables:**

- [x] Document manual load steps for `dist/chrome-extension`.
- [x] Document local server startup and configurable API origin.
- [x] Document Files + Git acceptance scenarios.
- [x] Document v1 limits: no direct file URL permission, no downloads permission, no native messaging, no embedded terminal streaming.
- [x] Add troubleshooting for stopped server, wrong port, and localhost permission issues.

**Verification:** `rtk npm run build:extension && rtk go test ./...`

**Commit:** `PM-034: Document Chrome extension showcase`

---

## Agentless Cloud Phases

### Phase B1: Cloud Workspace Access Adapters

**Deliverables:**

- [x] Add `WorkspaceAccessMode` to Cloud workspace persistence and API types; migrate current `cloud_agent` workspaces to `agent_backed`.
- [x] Define `WorkspaceAccessAdapter` responsibilities for workspace state, snapshots, read models, capabilities, and command execution.
- [x] Implement `AgentAccessAdapter` by delegating to the existing PM-032 registry and command-envelope path.
- [x] Add one resolver at the Cloud API/service boundary; remove route-level assumptions that every Cloud workspace has an agent ID.
- [x] Add tests for migration, adapter selection, owner isolation, and unchanged agent-backed command behavior.

**Verification:** `rtk go test ./internal/common/... ./internal/workspace/... ./internal/server/api/...`

**Commit:** `PM-034: Add Cloud workspace access adapters`

---

### Phase B2: Provider Remote Snapshot Adapter

**Deliverables:**

- [ ] Define `GitProviderIntegration` for authorization state, repository discovery, ref resolution, tree reads, file reads, and commit metadata.
- [ ] Implement one approved provider adapter with read-only authorization and strict repository ownership checks.
- [ ] Implement `RemoteSnapshotAdapter`; it must not invoke Git or access local paths.
- [ ] Persist provider repository identity, selected ref, resolved commit SHA, and authorization state without exposing tokens.
- [ ] Resolve selected branches and tags to immutable commit SHAs before returning content.
- [ ] Add fake-provider contract tests for revoked access, missing refs, forbidden repositories, and snapshot resolution.

**Verification:** `rtk go test ./internal/provider/... ./internal/workspace/... ./internal/server/api/...`

**Commit:** `PM-034: Add remote snapshot workspace adapter`

---

### Phase B3: Snapshot Read And Capability API

**Deliverables:**

- [ ] Route tree, file, plan, board, and search reads through `WorkspaceAccessAdapter`; key remote results by resolved commit SHA.
- [ ] Add bounded, sanitized remote snapshot caching with provider outage and rate-limit behavior.
- [ ] Return a capability map that enables only reads, snapshot selection, and terminal handoff for `remote_snapshot`.
- [ ] Return stable unsupported results for agentless Git mutations, file writes, and process execution; never forward them to provider writes.
- [ ] Prove remote workspaces never expose local path, dirty state, agent ID, token, or command envelope.

**Verification:** `rtk go test ./internal/server/api/... ./internal/workspace/... ./internal/search/...`

**Commit:** `PM-034: Add agentless snapshot read API`

---

### Phase F3: Agentless Workspace Registration

**Deliverables:**

- [ ] Extend workspace and runtime types with access mode, provider repository identity, selected ref, resolved commit SHA, and capabilities.
- [ ] Add an explicit Cloud workspace choice: Agent-Backed or Remote Snapshot.
- [ ] Reuse agent pairing only for Agent-Backed selection.
- [ ] Add provider connection, repository selection, and ref selection only for Remote Snapshot selection.
- [ ] Add tests for mode switching, validation, provider reconnect, and no-agent registration.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/pages/WorkspacesPage web/src/shared`

**Commit:** `PM-034: Add Cloud workspace access-mode registration`

---

### Phase F4: Snapshot Workspace Capability UI

**Deliverables:**

- [ ] Render a Remote Snapshot badge, provider repository, selected ref, and resolved commit SHA in Cloud workspace views.
- [ ] Key workspace read queries by commit SHA and invalidate them after ref changes.
- [ ] Render read-only explorer, plan, board, and search views from normalized snapshot responses.
- [ ] Hide local dirty state, file mutations, Git mutations, embedded terminal, AI, runtime, and verification controls when unsupported.
- [ ] Provide terminal-handoff guidance that does not imply Cloud can launch or observe a local terminal.
- [ ] Add agent-backed workspace regression tests.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/pages web/src/features web/src/shared`

**Commit:** `PM-034: Add agentless Cloud snapshot workspace UI`

---

### Phase C3: Provider Authorization And Cloud Smoke

**Deliverables:**

- [ ] Configure first-provider OAuth/App credentials through deployment secrets; use read-only repository scopes and encrypt stored authorization material.
- [ ] Document provider reconnect, rotation, revocation, cache, and outage behavior without exposing secrets.
- [ ] Verify an operator-owned provider test repository for registration, ref selection, commit-pinned reads, and recovery states.
- [ ] Verify agentless workspaces reject Git mutation and process commands before any provider write call.
- [ ] Run the PM-032 agent-backed smoke to prove adapter isolation.

**Verification:** `rtk go test ./... && rtk npm run typecheck && rtk npm test -- --run`, plus the documented Cloud/provider smoke.

**Commit:** `PM-034: Verify agentless Cloud remote workspaces`

## Post-Implementation Checklist

- [x] Update `plans/platform/PM-034/` docs to reflect final file names and commands.
- [x] Run Markdown formatting on all PM-034 Markdown files.
- [x] Run `rtk npm run typecheck`.
- [x] Run `rtk npm test -- --run`.
- [x] Run `rtk go test ./...`.
- [ ] PR description references PM-034 planning docs.
