# Backend Design: Terminal Canvas Workspace Orchestrator

## Overview

Add an `internal/canvas` domain that persists Canvas documents through the same provider-selected app-state boundary as
workspaces, navigation, AI settings, and indexes. A document stores layout and stable entity references. Read responses
resolve those references against current workspace, item, verification, Git, and active-session state. Existing guarded
services remain responsible for repository and process actions.

The Canvas API is available in Local and Cloud runtimes. Effective capabilities determine whether a referenced
workspace supports repository mutations, terminal launch, AI, Git, runtime, and verification. Agentless Remote Snapshot
documents can be edited as app-owned state, but their repository entities are read-only and cannot start processes.

## Domain Model

### Entity: Canvas Document

| Field         | Type             | Purpose                                                                             |
|---------------|------------------|-------------------------------------------------------------------------------------|
| `id`          | string           | Opaque stable document identifier.                                                  |
| `ownerUserId` | string, optional | Authenticated Cloud owner; empty in single-user Local mode.                         |
| `name`        | string           | User-visible Canvas name.                                                           |
| `scope`       | enum             | V1 value is `workspace`.                                                            |
| `workspaceId` | string           | Registered workspace used to resolve the default Canvas and its entities.           |
| `documentKey` | string           | `default` for the primary Canvas or an opaque key for a user-created copy.          |
| `version`     | integer          | Optimistic concurrency version incremented after every successful update.           |
| `viewport`    | viewport         | Saved x, y, and zoom values.                                                        |
| `preferences` | preferences      | Focus, grid, minimap, semantic zoom, and reduced-motion-compatible display choices. |
| `nodes`       | Canvas Node list | Ordered positioned entity references and app-owned note/group nodes.                |
| `edges`       | Canvas Edge list | Typed relationships between existing node IDs.                                      |
| `createdAt`   | timestamp        | Creation time.                                                                      |
| `updatedAt`   | timestamp        | Last successful document update.                                                    |

### Value: Canvas Node

| Field       | Type                | Purpose                                                              |
|-------------|---------------------|----------------------------------------------------------------------|
| `id`        | string              | Document-local stable node identifier.                               |
| `kind`      | enum                | `workspace`, `plan`, `session`, `artifact`, `note`, or `group`.      |
| `entityRef` | reference, optional | Existing entity type, ID, workspace ID, and optional commit context. |
| `position`  | point               | Finite Canvas x and y coordinates.                                   |
| `size`      | size, optional      | Bounded user size for supported node kinds.                          |
| `parentId`  | string, optional    | Group membership; must identify a group node in the same document.   |
| `note`      | string, optional    | Bounded app-owned text allowed only for note nodes.                  |
| `collapsed` | boolean             | Saved presentation state.                                            |

### Value: Canvas Edge

| Field    | Type             | Purpose                                                                          |
|----------|------------------|----------------------------------------------------------------------------------|
| `id`     | string           | Document-local stable edge identifier.                                           |
| `source` | string           | Existing source node ID.                                                         |
| `target` | string           | Existing target node ID.                                                         |
| `kind`   | enum             | `contains`, `implements`, `produced`, `verifies`, `depends_on`, or `blocked_by`. |
| `label`  | string, optional | Bounded user label for relationships that need clarification.                    |

### Projection: Resolved Canvas Node

Read responses pair persisted node layout with a non-persisted current-state projection.

| Field          | Type              | Purpose                                                                                  |
|----------------|-------------------|------------------------------------------------------------------------------------------|
| `resolution`   | enum              | `resolved`, `stale`, `unavailable`, or `forbidden`.                                      |
| `title`        | string            | Current entity title, not copied into persistent layout.                                 |
| `subtitle`     | string, optional  | Workspace, branch, commit, provider, or contextual label.                                |
| `status`       | string, optional  | Current plan, health, verification, or session lifecycle state.                          |
| `capabilities` | capability object | Current readable/editable/executable actions after role, mode, access, and Agent checks. |
| `recoveryHint` | string, optional  | Safe user guidance for stale references or unavailable execution.                        |

## Persistence

### Data-dir Store

- Add `canvases.yaml` under the effective Kode Stream data directory.
- Use a Canvas repository with the same mutex, validation, atomic replacement, and empty-list behavior as other
  file-backed repositories.
- Include Canvas documents in `datadir -> database` and `database -> datadir` snapshots and pre-replacement backups.
- Never place this file inside a registered workspace.

### SQLite And Postgres

Add migration version 2 for both SQL drivers. The existing migration conversion continues to normalize timestamp and
boolean types for Postgres.

```sql
CREATE TABLE IF NOT EXISTS canvas_documents (
  id TEXT PRIMARY KEY,
  owner_user_id TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL,
  scope TEXT NOT NULL,
  workspace_id TEXT NOT NULL,
  document_key TEXT NOT NULL,
  version INTEGER NOT NULL,
  document_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS canvas_documents_owner_workspace_scope
  ON canvas_documents (owner_user_id, workspace_id, scope, document_key);

CREATE INDEX IF NOT EXISTS canvas_documents_owner_updated
  ON canvas_documents (owner_user_id, updated_at);
```

The JSON payload contains viewport, preferences, nodes, and edges. Indexed columns support ownership, workspace default
resolution, and recent ordering without normalizing every spatial update into multiple writes.

### Repository Boundary

Extend `storage.RepositoryBundle` with a Canvas repository. Both provider composition and manual sync must treat Canvas
documents as app-owned state. Domain services depend on this repository interface and do not inspect storage option or
driver names.

## API Contract

| Method | Endpoint                                     | Request                                               | Response                                                 |
|--------|----------------------------------------------|-------------------------------------------------------|----------------------------------------------------------|
| GET    | `/api/canvases?workspaceId={id}`             | Query scope                                           | Owned Canvas summaries                                   |
| POST   | `/api/canvases/resolve`                      | Workspace ID and optional name                        | Existing or newly created `default` resolved document    |
| GET    | `/api/canvases/{canvasId}`                   | None                                                  | Resolved document with current entity projections        |
| PUT    | `/api/canvases/{canvasId}`                   | Expected version, viewport, preferences, nodes, edges | Updated persisted document and next version              |
| POST   | `/api/canvases/{canvasId}/copies`            | Name and expected source version                      | New owned Canvas with an opaque document key             |
| DELETE | `/api/canvases/{canvasId}`                   | None                                                  | `204`; deletes only Canvas metadata                      |
| GET    | `/api/ai/sessions?workspaceId={id}&active=1` | Workspace filter                                      | Safe active embedded-session summaries; no grants/output |

`POST /api/canvases/resolve` is idempotent for one owner, workspace, `workspace` scope, and `default` document key.
Concurrent resolve calls return the document selected by that unique key. Copies receive opaque document keys and do
not replace the default Canvas.

### Update Conflict

- Client sends the version it loaded.
- Repository updates only when the stored version matches.
- A mismatch returns `409` with error code `canvas_version_conflict` and the current version.
- The backend does not merge coordinates, nodes, notes, or edges.
- Frontend may reload latest or create a copy after explicit user choice.

## Entity Resolution

The service resolves node references in bounded batches:

1. Load and authorize the Canvas document.
2. Group references by workspace, plan, session, and artifact kind.
3. Read workspace configuration and indexed plan state through existing repositories.
4. Read safe in-memory session summaries from the embedded session manager.
5. Attach effective capabilities and current labels/status without changing the stored document.
6. Mark missing or inaccessible references explicitly; never delete them as a read side effect.

V1 artifact resolution covers verification jobs and Git-change summaries already available through workspace services.
File contents, diffs, and terminal output load only when the Workbench requests them.

## Capability Policy

| Condition                             | Layout edit | Plan read | Repo write | Terminal / AI | Git / verify |
|---------------------------------------|-------------|-----------|------------|---------------|--------------|
| Local admin, writable workspace       | yes         | yes       | yes        | yes           | yes          |
| Local read-only workspace or role     | policy      | yes       | no         | no            | no           |
| Agent-Backed, owner Agent connected   | role-based  | yes       | role-based | role-based    | role-based   |
| Agent-Backed, owner Agent offline     | role-based  | cached    | no         | no            | no           |
| Remote Snapshot                       | role-based  | yes       | no         | no            | no           |
| Missing or forbidden entity reference | role-based  | no        | no         | no            | no           |

Layout editing uses app-state authorization. Repository and runtime actions use the target workspace's access adapter
and effective runtime capabilities. The API returns denial reasons suitable for inline UI guidance.

## Validation And Limits

Initial server-configured defaults:

| Limit                    | Default | Purpose                                                 |
|--------------------------|---------|---------------------------------------------------------|
| Documents per owner      | 100     | Bound list and storage growth.                          |
| Nodes per document       | 1,000   | Keep save, resolve, and render operations bounded.      |
| Edges per document       | 2,000   | Bound relationship validation and payload size.         |
| Serialized document size | 2 MiB   | Reject unexpected or abusive app-state payloads.        |
| Document name            | 120     | Keep menus and API logs bounded.                        |
| Note text per node       | 8 KiB   | Support useful notes without becoming a document store. |

Validation rejects unknown kinds, duplicate IDs, dangling edge endpoints, invalid parents, cycles in group ancestry,
non-finite coordinates, out-of-range zoom, entity references outside the document workspace, and terminal data or
grant-like fields in persisted node payloads.

## Security And Privacy

- Authorize every Canvas by owner before returning whether it exists.
- Resolve only workspaces accessible to the current Local runtime or Cloud identity.
- Treat names, labels, notes, and entity titles as untrusted text.
- Never persist terminal output, input, environment variables, executable arguments, prompts, grant tokens, credentials,
  repository contents, diffs, or file bodies in Canvas storage.
- Reuse existing path guards, role checks, access adapters, launch validation, audit policy, PTY limits, and WebSocket
  origin/token checks for actions initiated from Canvas.
- Canvas delete affects only app-owned metadata and must not cascade into repository or runtime entities.

## Failure Handling

| Failure                        | API behavior                              | User-visible recovery                                    |
|--------------------------------|-------------------------------------------|----------------------------------------------------------|
| Canvas not found               | `404`                                     | Return to workspace Canvas list.                         |
| Canvas version conflict        | `409 canvas_version_conflict`             | Reload latest or save current layout as a copy.          |
| Invalid document               | `400` with stable field-level issue codes | Keep unsaved state and identify invalid node/edge.       |
| Owner Agent offline            | Read succeeds; execution capabilities off | Reconnect Agent; do not discard layout.                  |
| Remote Snapshot action attempt | `403` or capability-specific unavailable  | Connect Agent or open locally.                           |
| Missing entity                 | Read succeeds with stale resolution       | Remove reference or locate replacement.                  |
| Storage unavailable            | Existing storage error mapping            | Retry; keep unsaved frontend state until user navigates. |
| Embedded session expired       | Summary resolves as ended/stale           | Remove node or launch a new session from the plan.       |

## Observability And Tests

- Audit Canvas create, update, delete, conflict, and blocked execution intent without node note content.
- Record duration and counts, not serialized documents or terminal data.
- Add domain tests for validation, ownership, default resolution, version conflicts, and deletion isolation.
- Add repository contract tests for data-dir, SQLite, and Postgres-compatible SQL behavior.
- Add storage sync tests proving Canvas round trips and backup behavior.
- Add API tests for Local, Agent-Backed online/offline, Remote Snapshot, role denial, stale references, and safe session
  projection.
- Retain existing AI terminal lifecycle, grant, loopback, limit, cancellation, and shutdown tests unchanged.

## Design Decisions

| Decision                                       | Rationale                                                                                    |
|------------------------------------------------|----------------------------------------------------------------------------------------------|
| New `internal/canvas` domain                   | Keeps spatial metadata separate from workspace files, item indexing, and terminal runtime.   |
| Provider-backed Canvas repository              | Gives data-dir, SQLite, and Postgres equal behavior without mode branching in services.      |
| Versioned document JSON                        | Spatial updates remain atomic while indexed ownership fields keep lookup efficient.          |
| Resolve current state on reads                 | Saved layout remains stable while Git-backed and runtime information stays authoritative.    |
| Safe active-session list endpoint              | Restores live terminal nodes after navigation without exposing channel grants or output.     |
| Capability composition before action rendering | Prevents unsupported operations from appearing enabled in Agentless or offline-Agent states. |
| No backend coordinate merge                    | Automatic spatial merges are surprising; explicit reload/copy recovery avoids silent loss.   |
