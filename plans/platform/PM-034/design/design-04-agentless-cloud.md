# Design: Agentless Cloud Remote Snapshots

## Overview

Cloud workspace routes depend on `WorkspaceAccessAdapter`, not directly on a Cloud Agent. The resolver chooses an
adapter from `WorkspaceAccessMode`. `AgentAccessAdapter` preserves PM-032's agent registry and command-envelope path.
`RemoteSnapshotAdapter` uses a `GitProviderIntegration` to read one authorized provider repository at an immutable
commit and returns no local-process capability.

## Workspace Model

| Field                | Purpose                                                        |
|----------------------|----------------------------------------------------------------|
| `accessMode`         | `agent_backed` or `remote_snapshot`.                           |
| `provider`           | Initial approved Git provider for a remote snapshot workspace. |
| `providerRepository` | Provider repository identity; never an executable local path.  |
| `selectedRef`        | User-selected branch, tag, or commit.                          |
| `resolvedCommitSHA`  | Commit resolved before every snapshot read.                    |
| `agentId`            | Required only by `agent_backed`.                               |

## Adapter Responsibilities

| Responsibility            | `AgentAccessAdapter`                             | `RemoteSnapshotAdapter`                                |
|---------------------------|--------------------------------------------------|--------------------------------------------------------|
| Workspace state           | Owner agent availability and published metadata. | Provider authorization and repository access.          |
| Snapshot choices          | Existing agent-backed Git information.           | Provider branches, tags, and commit metadata.          |
| Tree, file, board, search | Agent-routed or agent-published data.            | Provider reads or sanitized, commit-keyed Cloud cache. |
| Capability map            | Role and connected-agent capabilities.           | Reads, snapshot selection, and terminal handoff only.  |
| Command execution         | Existing command envelope.                       | Stable unsupported-capability result.                  |

## Capability Policy

| Capability                                    | Agent-Backed                           | Remote Snapshot                       |
|-----------------------------------------------|----------------------------------------|---------------------------------------|
| Browse plans and files                        | Available when agent data is available | Available with provider authorization |
| Select branch, tag, or commit                 | Available                              | Available                             |
| Local dirty state                             | Available when agent supports it       | Unavailable                           |
| File write and Git mutations                  | Role/agent-gated                       | Unavailable                           |
| Terminal, AI, runtime, verification execution | Role/agent-gated                       | Unavailable                           |
| Terminal handoff                              | Configured action                      | Copy/open-local instruction only      |

## Provider Boundary

| Area                 | Requirement                                                                       |
|----------------------|-----------------------------------------------------------------------------------|
| Authorization        | One provider adapter with read-only OAuth/App repository access.                  |
| Token handling       | Encrypt stored authorization material; never return or log token values.          |
| Snapshot consistency | Resolve a selected ref to a commit SHA before tree, file, board, or search reads. |
| Caching              | Bound and sanitize commit-keyed read models; never create a hosted checkout.      |
| Failure behavior     | Reconnect, forbidden, missing-ref, rate-limit, and outage states are explicit.    |

## UI Contract

| Area                   | Agentless Cloud Behavior                                               |
|------------------------|------------------------------------------------------------------------|
| Workspace registration | Select Remote Snapshot, authorize provider, choose repository and ref. |
| Workspace label        | Show provider repository, selected ref, and resolved commit SHA.       |
| Explorer and board     | Read-only and keyed by the resolved commit SHA.                        |
| Unsupported controls   | Hide local writes, dirty state, Git mutations, and process controls.   |
| Terminal handoff       | Explain that the user runs local Git or terminal work outside Cloud.   |

## Design Decisions

| Decision                                 | Rationale                                                                      |
|------------------------------------------|--------------------------------------------------------------------------------|
| Resolver depends on an adapter interface | Keeps agent/provider details out of API routes and UI state.                   |
| Snapshot reads are commit-pinned         | Avoids mixed views when a branch advances during concurrent requests.          |
| First provider is read-only              | Prevents Cloud remote operations from being mistaken for local Git changes.    |
| Existing agent path is an adapter        | Avoids a PM-034 regression and makes both modes testable through one contract. |
