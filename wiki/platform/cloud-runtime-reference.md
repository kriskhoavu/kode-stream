---
slug: platform-cloud-runtime-reference
title: Cloud Runtime Reference
pageType: REFERENCE
topics: cloud, authentication, agent, snapshot, authorization
summary: Reference for Cloud authentication, Agent-backed metadata, Remote Snapshot reads, and delivery boundaries.
sourceRef: |
  PM-032 (plan retired) | Cloud runtime
  PM-032 (plan retired) | Authentication and Agent protocol
  PM-032 (plan retired) | Cloud user flow
  PM-032 (plan retired) | Deployment
  PM-032 (plan retired) | Release gates
  PM-032 (plan retired) | Component boundary
  PM-034 (plan retired) | Remote Snapshot
lastVerified: 2026-11-24
---

## Cloud Model
<!-- chunkId: platform-cloud-runtime-reference-cloud-model -->
<!-- keywords: control plane, postgres, workspace access mode, trust boundary -->

Cloud is a hosted control plane, not a hosted checkout or terminal. It stores workspace metadata, derived indexes, audit events and settings in Postgres. Each workspace picks one access mode.

| Access mode     | Needs a Cloud Agent | Repository source                     | Available now                                                     |
|-----------------|---------------------|---------------------------------------|-------------------------------------------------------------------|
| Agent-Backed    | Yes                 | The owner's machine                   | Agent pairing, heartbeat and workspace metadata. Commands return 503 |
| Remote Snapshot | No                  | A Git provider at a resolved commit   | Commit-pinned metadata, tree and file reads                       |

The Cloud API never receives an Agent-Backed checkout, SSH key, terminal transcript or provider token. Agents connect only to the Cloud API, never to Postgres. Agent-Backed records use `WorkspaceLocation=cloud_agent`. Remote Snapshot records use `cloud_remote_snapshot` and never hold a local path, agent ID, command envelope or credential.

## Authentication Boundary
<!-- chunkId: platform-cloud-runtime-reference-authentication-boundary -->
<!-- keywords: oauth2 proxy, trusted cidr, identity headers, role -->

[live] Cloud mode accepts only `KODE_STREAM_AUTH_MODE=oauth2_proxy`, requires trusted proxy CIDRs that are not catch-all, and rejects `app_oidc` at startup. Evidence: `internal/system/runtime_config.go:118`, `internal/server/api/cloud_auth.go:142`.

Identity headers count only from a remote address inside a trusted proxy network. The user's role then maps to route capabilities.

## Agent-backed Workspaces
<!-- chunkId: platform-cloud-runtime-reference-agent-backed-workspaces -->
<!-- keywords: cloud agent, heartbeat, deep link, command delivery -->

[live] Agent pairing, the outbound connection to `/api/agents/channel`, heartbeat and metadata publication work. Workspace command delivery returns 503 until a durable command lifecycle exists. The default dispatcher also reports dispatch as unimplemented. Evidence: `internal/server/api/cloud_workspace_access.go:49`, `internal/agent/protocol.go:55`.

[live] The `connected` frame lists each workspace the Agent has published, with its revision. The Agent publishes again with that revision, so a restarted Agent updates the workspace it published before. Evidence: `internal/server/api/cloud_workspace.go:46`, `internal/agent/client.go:77`.

"Connected" is a prerequisite, not proof that file, Git, terminal, runtime, AI or verification commands can run.

The Cloud UI pairs an Agent with `kodestream://connect?token=<short-lived-token>`. The Agent uses the token for an authenticated outbound WebSocket. The operator procedure is `deploy/cloud-deployment.md`.

## Remote Snapshot Workspaces
<!-- chunkId: platform-cloud-runtime-reference-remote-snapshot-workspaces -->
<!-- keywords: commit sha, provider authorization, read only, encrypted connection -->

[live] `/api/workspaces/{id}/snapshot`, `/snapshot/tree` and `/snapshot/files` read metadata, tree and files at the workspace's stored commit SHA. Each call reauthorizes the provider connection. Evidence: `internal/server/api/gin_routes.go:127`, `internal/server/api/cloud_snapshot.go:21`.

[live] Provider connections are stored encrypted per user. Evidence: `internal/server/api/cloud_provider.go:64`.

Workspace details show the repository, ref, commit and local-terminal guidance. A provider outage or rate limit has no local fallback. A response never includes a local path, dirty state, agent ID, command envelope or credential.

## Not supported
<!-- chunkId: platform-cloud-runtime-reference-not-supported -->
<!-- keywords: app oidc, command queue, hosted terminal, provider write -->

[live] App-owned OIDC, Agent command delivery, hosted repository execution, provider writes, self-service provider registration and a snapshot-backed board or search. Evidence: `internal/system/runtime_config.go:118`, `internal/server/api/cloud_workspace_access.go:49`.

## Known conflicts
<!-- chunkId: platform-cloud-runtime-reference-known-conflicts -->
<!-- keywords: command availability, oidc claim, connection persistence -->

- "An Agent-Backed workspace can run file, Git and terminal actions." Its commands return 503.
- "Encrypted provider connections are future work." They are stored today.

## Related pages

See [[platform-storage-reference]].
