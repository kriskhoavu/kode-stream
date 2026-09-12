---
slug: platform-cloud-runtime-reference
title: Cloud Runtime Reference
pageType: REFERENCE
topics: cloud, authentication, agent, snapshot, authorization
summary: Reference for Cloud authentication, Agent-backed metadata, Remote Snapshot reads, and delivery boundaries.
sourceRef: |
  plans/platform/PM-032/README.md | Cloud runtime
  plans/platform/PM-032/design/design-01-backend.md | Authentication and Agent protocol
  plans/platform/PM-032/design/design-02-frontend.md | Cloud user flow
  plans/platform/PM-032/design/design-03-infrastructure.md | Deployment
  plans/platform/PM-032/design/design-04-pipeline.md | Release gates
  plans/platform/PM-032/design/cloud-agent-component.mmd | Component boundary
  plans/platform/PM-034/design/design-04-agentless-cloud.md | Remote Snapshot
lastVerified: 2026-09-12
---

## Authentication Boundary
<!-- chunkId: platform-cloud-runtime-reference-authentication-boundary -->
<!-- keywords: oauth2 proxy, trusted cidr, cloud identity, role -->

[live] Cloud mode accepts only `oauth2_proxy`, requires non-catch-all trusted proxy CIDRs, and rejects `app_oidc` as unimplemented. Evidence: `internal/system/runtime_config.go:92`, `internal/server/api/cloud_auth.go:142`.

Identity headers are accepted only from a request whose remote address belongs to a configured trusted proxy network. Authorization then resolves the authenticated user's role into route capabilities.

## Agent-backed Workspaces
<!-- chunkId: platform-cloud-runtime-reference-agent-backed-workspaces -->
<!-- keywords: cloud agent, heartbeat, workspace metadata, command delivery -->

[live] Agent pairing, outbound connection, heartbeat, and workspace metadata publication exist, but workspace command delivery deliberately returns 503 until a durable command lifecycle exists. Evidence: `internal/server/api/cloud_workspace_access.go:25`, `internal/agent/protocol.go:39`.

Connected status is a capability prerequisite, not proof that file, Git, terminal, runtime, AI, or verification commands can be dispatched. The default Agent command dispatcher also reports that dispatch is unimplemented.

## Remote Snapshot Workspaces
<!-- chunkId: platform-cloud-runtime-reference-remote-snapshot-workspaces -->
<!-- keywords: remote snapshot, commit sha, provider authorization, read only -->

[live] Remote Snapshot endpoints provide metadata, tree, and file reads against the workspace's stored commit SHA and reauthorize the provider connection on each call. Evidence: `internal/server/api/gin_routes.go:120`, `internal/server/api/cloud_snapshot.go:38`.

The mode exposes repository reads and snapshot selection while withholding mutation and process capabilities. Provider connections can be persisted through encrypted Cloud state, but self-service provider registration and snapshot-backed board/search remain incomplete.

## Not supported
<!-- chunkId: platform-cloud-runtime-reference-not-supported -->
<!-- keywords: app oidc, command queue, hosted terminal, provider write -->

[live] App-owned OIDC, durable Agent command delivery, hosted repository execution, and provider writes are not supported. Evidence: `internal/system/runtime_config.go:117`, `internal/server/api/cloud_workspace_access.go:48`.

## Known conflicts
<!-- chunkId: platform-cloud-runtime-reference-known-conflicts -->
<!-- keywords: documentation drift, command availability, oidc claim -->

[live] `docs/domain/cloud/cloud-deployment.md` incorrectly presents `app_oidc` as available, while several Cloud pages describe command-capable Agents beyond the implemented metadata channel. Evidence: `internal/system/runtime_config.go:117`, `internal/server/api/cloud_workspace_access.go:48`.

## Related pages

See [[platform-storage-reference]], [[platform-extension-reference]], and [[workspace-terminal-canvas-reference]].
