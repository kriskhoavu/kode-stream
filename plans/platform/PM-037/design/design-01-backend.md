# Backend Design: Focused Terminal Canvas

## Overview

PM-037 adds backend contracts for a branch-scoped Canvas without making Canvas authoritative for workspace entities.
The Canvas repository stores layout identity, viewport preference, and independently versioned placements. Read
responses resolve each placement against current workspace, plan, session, Git, and verification services.

The same design separates deployment topology, workspace content provider, execution provider, app-state datastore,
and authorization. Canvas consumes resolved action capabilities and never branches on Local, Cloud, Agentless,
data-dir, SQLite, or Postgres names.

PM-037 implements Local execution with data-dir and SQLite persistence. Future Agent and snapshot phases add provider
adapters and Postgres repositories without changing Canvas domain contracts.

## Domain Model

### Entity: Canvas Layout

| Field         | Type             | Purpose                                                       |
|---------------|------------------|---------------------------------------------------------------|
| `id`          | string           | Opaque stable layout identifier.                              |
| `ownerUserId` | string, optional | Reserved ownership scope; empty in Local mode.                |
| `workspaceId` | string           | Registered workspace shown by the layout.                     |
| `branchKey`   | string           | Normalized selected branch/ref context.                       |
| `viewport`    | viewport         | Last saved x, y, and zoom when server persistence is enabled. |
| `version`     | integer          | Metadata version, independent from placement revisions.       |
| `createdAt`   | timestamp        | Creation time.                                                |
| `updatedAt`   | timestamp        | Latest layout or placement update.                            |

There is one default layout for an owner, workspace, and branch key. PM-037 has no copies, names, list endpoint, or
metadata-only deletion UI.

### Entity: Placement

| Field       | Type      | Purpose                                                     |
|-------------|-----------|-------------------------------------------------------------|
| `nodeId`    | string    | Stable Canvas-local node identity.                          |
| `layoutId`  | string    | Owning branch-scoped layout.                                |
| `entityRef` | reference | Workspace, branch-aware plan, or durable session reference. |
| `position`  | point     | Finite absolute x and y coordinates.                        |
| `collapsed` | boolean   | Bounded presentation state.                                 |
| `revision`  | integer   | Optimistic revision for this placement.                     |
| `updatedAt` | timestamp | Latest successful placement update.                         |

Placement ordering is not authoritative. Node render order is derived in the frontend from kind, selection, and current
status. PM-037 does not persist node size, notes, parent IDs, groups, or arbitrary node payloads.

### Value: Entity Reference

| Kind        | Required Identity                                             | Resolution Notes                                                                                      |
|-------------|---------------------------------------------------------------|-------------------------------------------------------------------------------------------------------|
| `workspace` | Workspace ID                                                  | Resolves current configuration, branch, Git state, and capabilities.                                  |
| `plan`      | Workspace ID, item ID, item path, branch key, observed commit | Item ID is the current lookup key; path and identifier may support explicit stale-reference recovery. |
| `session`   | Durable session-record ID                                     | Resolves metadata first and an optional live process binding separately.                              |

The plan item ID remains branch- and path-derived in current code. A rename or move can therefore create a stale
reference. PM-037 may offer a replacement candidate by branch and identifier, but must not silently rebind it. A future
repository-stable plan identifier can replace this recovery rule without changing placement storage.

### Projection: Resolved Node

| Field          | Type             | Purpose                                             |
|----------------|------------------|-----------------------------------------------------|
| `nodeId`       | string           | Placement identity.                                 |
| `resolution`   | enum             | `resolved`, `stale`, `unavailable`, or `forbidden`. |
| `title`        | string           | Current authoritative title.                        |
| `subtitle`     | string, optional | Branch, commit, provider, or execution context.     |
| `status`       | string, optional | Current plan, Git, session, or verification state.  |
| `capabilities` | action map       | Current action states and recovery reasons.         |
| `recoveryHint` | string, optional | Safe guidance for stale or unavailable references.  |

Resolved fields are response-only and never accepted in placement writes.

## Independent Runtime Axes

### Deployment Topology

Controls identity, ownership, and routing only. It does not decide whether a terminal or repository read is possible.

### Workspace Content Provider

Owns repository reads, Git projections, and ref resolution. PM-037 uses the local checkout provider. Future providers
include an Agent checkout and a commit-pinned provider snapshot.

### Execution Provider

Owns terminal, AI, runtime, and verification execution. PM-037 uses the local process provider. Future values include a
connected Agent and no provider. Absence of an execution provider does not remove content-provider capabilities.

### App-State Datastore

Owns Canvas layouts, placements, and durable session records. Domain services depend on repository interfaces and never
inspect driver or storage-option names.

### Authorization

Filters supported provider actions for the current identity. Authorization cannot turn an unsupported action into a
supported one.

## Action Capability Contract

| Field             | Purpose                                                                                    |
|-------------------|--------------------------------------------------------------------------------------------|
| `action`          | Stable name such as `layout.move`, `git.status`, `terminal.launch`, or `verification.run`. |
| `state`           | `available`, `unavailable`, `unsupported`, `forbidden`, or `conflicted`.                   |
| `reasonCode`      | Stable machine-readable explanation.                                                       |
| `message`         | Safe user-facing summary.                                                                  |
| `recoveryActions` | Supported next actions, such as refresh or open branch controls.                           |

Capability composition follows this order:

1. Determine whether the content or execution provider supports the action.
2. Apply current provider availability, including tool and connection health.
3. Apply authorization.
4. Apply entity context such as source editability and branch match.
5. Return the most specific non-available state and recovery guidance.

The frontend uses this projection for presentation. Every service revalidates its own guard immediately before action
execution to prevent time-of-check/time-of-use errors.

## Branch-Safe Launch

The launch request includes workspace ID, plan ID, expected branch key, observed commit, and an idempotency key.

Before starting a process, the backend:

1. Resolves the current plan and workspace.
2. Confirms the plan belongs to the requested workspace and branch context.
3. Reads the current checkout branch from Git.
4. Rejects a mismatch with `terminal_branch_mismatch` and returns expected branch, current branch, and dirty-state
   guidance.
5. Revalidates execution-provider support, tool availability, authorization, and bounded-session limits.
6. Creates one durable session record for the idempotency key.
7. Starts the ephemeral process binding and updates the record to running only after PTY start succeeds.

PM-037 never switches branches automatically. A future worktree provider may satisfy launch for another branch without
changing this contract.

## Durable Session Metadata

### Entity: Session Record

| Field             | Type                             | Purpose                                                                   |
|-------------------|----------------------------------|---------------------------------------------------------------------------|
| `id`              | string                           | Stable session identity referenced by Canvas.                             |
| `workspaceId`     | string                           | Owning workspace.                                                         |
| `planRef`         | branch-aware reference, optional | Plan that initiated the work.                                             |
| `provider`        | string                           | Safe provider label.                                                      |
| `intent`          | string                           | Bounded non-sensitive intent category, not the prompt.                    |
| `requestedBranch` | string                           | Branch validated at launch.                                               |
| `observedCommit`  | string, optional                 | HEAD observed before launch.                                              |
| `state`           | enum                             | `starting`, `running`, `exited`, `cancelled`, `failed`, or `interrupted`. |
| `startedAt`       | timestamp                        | Launch time.                                                              |
| `endedAt`         | timestamp, optional              | Terminal lifecycle completion.                                            |
| `exitCode`        | integer, optional                | Safe process outcome.                                                     |
| `lastKnownAt`     | timestamp                        | Latest durable lifecycle observation.                                     |

The record excludes executable arguments, prompts, environment variables, grants, output, input, buffers, credentials,
and file content.

### Ephemeral: Process Binding

The existing terminal manager continues to own the process, PTY, output buffer, channel subscribers, grants, reconnect
timer, and cancellation. It exposes a safe lookup by session-record ID. Canvas never persists or reconstructs a process
binding.

At application startup, records left in `starting` or `running` with no corresponding live binding become
`interrupted`. Page reload within the same process can reconnect under existing grant and lease rules. Application
restart restores honest metadata but does not promise terminal reconnection.

Removing a placement does not cancel its process. Cancelling a process does not remove its placement.

## Verification Freshness

### Value: Repository Fingerprint

| Field               | Purpose                                                                                      |
|---------------------|----------------------------------------------------------------------------------------------|
| `branch`            | Current checkout branch.                                                                     |
| `headCommit`        | Current HEAD commit SHA.                                                                     |
| `indexHash`         | Digest representing staged content.                                                          |
| `worktreeHash`      | Digest representing relevant tracked and untracked working-tree content.                     |
| `configurationHash` | Digest of the selected verification profile, commands, environment name, and selected specs. |

The fingerprint algorithm must be deterministic and conservative. It may reuse Git plumbing and bounded file hashing,
but must not rely only on timestamps or `git status` text. Ignored files are excluded unless the verification
configuration explicitly includes them.

Each verification job captures a start fingerprint and a completion fingerprint. The result projection is:

| Freshness      | Condition                                                                                      |
|----------------|------------------------------------------------------------------------------------------------|
| `fresh`        | Start and completion fingerprints match, and the current fingerprint still matches completion. |
| `stale`        | The current repository or configuration fingerprint differs from the completed result.         |
| `inconclusive` | Repository state changed during execution or a complete fingerprint could not be produced.     |

PM-037 extends the current in-memory verification job with fingerprints and freshness projection. Durable verification
history and lineage are follow-up scope. Browser reload can recover a result while the service remains running;
application restart does not restore verification jobs in PM-037.

## Persistence

### Data-Dir Store

- Add Canvas layouts and placements to app-owned Canvas storage under the effective data directory.
- Add safe session records to app-owned session metadata storage.
- Use guarded atomic replacement and repository-level validation.
- Include both repositories in Local data-dir to SQLite sync and pre-replacement backups.
- Never place Canvas or session metadata inside a registered workspace.

### SQLite

Add the next Local database migration with normalized layout, placement, and session-record tables.

```sql
CREATE TABLE IF NOT EXISTS canvas_layouts (
  id TEXT PRIMARY KEY,
  owner_user_id TEXT NOT NULL DEFAULT '',
  workspace_id TEXT NOT NULL,
  branch_key TEXT NOT NULL,
  viewport_json TEXT NOT NULL,
  version INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS canvas_layouts_owner_workspace_branch
  ON canvas_layouts (owner_user_id, workspace_id, branch_key);

CREATE TABLE IF NOT EXISTS canvas_placements (
  layout_id TEXT NOT NULL,
  node_id TEXT NOT NULL,
  entity_kind TEXT NOT NULL,
  entity_ref_json TEXT NOT NULL,
  x REAL NOT NULL,
  y REAL NOT NULL,
  collapsed INTEGER NOT NULL DEFAULT 0,
  revision INTEGER NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (layout_id, node_id)
);

CREATE TABLE IF NOT EXISTS ai_session_records (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL,
  plan_ref_json TEXT NOT NULL,
  provider TEXT NOT NULL,
  intent TEXT NOT NULL,
  requested_branch TEXT NOT NULL,
  observed_commit TEXT NOT NULL,
  state TEXT NOT NULL,
  started_at TEXT NOT NULL,
  ended_at TEXT NOT NULL,
  exit_code INTEGER,
  last_known_at TEXT NOT NULL
);
```

Postgres compatibility is a repository-contract requirement but its migration and Cloud ownership rollout are deferred
to the Agent-Backed Canvas phase.

## API Contract

| Method  | Endpoint                                                   | Request                                   | Response                                                   |
|---------|------------------------------------------------------------|-------------------------------------------|------------------------------------------------------------|
| `POST`  | `/api/canvas/resolve`                                      | Workspace ID and branch                   | Existing or created default layout with resolved nodes     |
| `GET`   | `/api/canvas/{layoutId}`                                   | None                                      | Layout, placements, resolved projections, and capabilities |
| `PATCH` | `/api/canvas/{layoutId}/placements`                        | Placement changes with expected revisions | Updated placement revisions and layout timestamp           |
| `PATCH` | `/api/canvas/{layoutId}/viewport`                          | Expected layout version and viewport      | Updated layout version                                     |
| `GET`   | `/api/ai/session-records?workspaceId={id}&branch={branch}` | Workspace and branch filter               | Safe durable session records with live-binding state       |

Existing guarded terminal, Git, and verification endpoints remain the action owners. Canvas does not add proxy action
endpoints that bypass those services.

### Placement Conflict

- Each placement change includes its loaded revision.
- Updates succeed independently when stored revisions match.
- A mismatch returns `409 canvas_placement_conflict` with only the conflicting node IDs and current revisions.
- Non-conflicting placement updates in the same batch may succeed only if the API clearly returns per-item outcomes;
  PM-037 may instead reject the complete small batch for simpler atomic drag behavior.
- Recovery offers reload of affected placements and reset-layout preview. PM-037 does not create Canvas copies.

## Initial Placement And Discovery

- First resolve creates the workspace placement and current branch plan placements with deterministic coordinates.
- Existing durable sessions for the workspace and branch receive deterministic initial placements near their plan.
- A newly indexed plan or newly launched session receives an unplaced projection until the client accepts its suggested
  position; existing placements never move during background resolution.
- Missing entities remain stale placements and are never deleted as a read side effect.

## Validation And Limits

| Limit                              | Initial Default | Purpose                                                      |
|------------------------------------|-----------------|--------------------------------------------------------------|
| Placements per layout              | 300             | Bound Local MVP resolution and rendering.                    |
| Placement update batch             | 50              | Support multi-select movement without full-layout writes.    |
| Coordinate magnitude               | 1,000,000       | Reject non-finite or abusive geometry.                       |
| Viewport zoom                      | 0.1 to 2.0      | Keep recovery and rendering usable.                          |
| Safe session records per workspace | 500             | Bound durable metadata growth before archival policy exists. |

Validation rejects unknown entity kinds, cross-workspace references, branch-context violations, duplicate IDs,
non-finite coordinates, invalid revisions, and any terminal or repository content in app-state payloads.

## Security And Privacy

- Authorize layout and session records before revealing whether they exist.
- Resolve only workspaces and entities accessible to the current identity.
- Never return a cached stale title for an entity that has become forbidden.
- Treat resolved titles and labels as untrusted text.
- Do not log placement payloads, prompts, arguments, environment variables, grants, terminal bytes, repository content,
  diffs, or verification artifact bodies.
- Reuse existing path guards, Git safety checks, terminal limits, WebSocket origin checks, and execution-provider guards.
- Placement removal and layout reset never mutate repository, session, or verification entities.

## Failure Handling

| Failure                                | API Behavior                                             | User Recovery                                                    |
|----------------------------------------|----------------------------------------------------------|------------------------------------------------------------------|
| Placement revision conflict            | `409 canvas_placement_conflict`                          | Reload affected placement or reset with preview.                 |
| Branch mismatch at launch              | `409 terminal_branch_mismatch`                           | Review expected/current branch and open guarded branch controls. |
| Unsupported execution provider         | Capability state plus guarded action rejection           | Use a workspace with an execution provider.                      |
| Missing plan after rename or deletion  | Resolve stale placement                                  | Remove placement or confirm a suggested replacement.             |
| Lost process after restart             | Reconcile durable record to `interrupted`                | Launch a new session; do not offer reconnect.                    |
| Repository changes after verification  | Return `stale` freshness                                 | Rerun verification on the current fingerprint.                   |
| Repository changes during verification | Return `inconclusive` freshness                          | Stabilize the repository and rerun.                              |
| App-state store unavailable            | Preserve in-memory client edits and return storage error | Retry without mutating entities.                                 |

## Tests

- Repository contract tests for data-dir and SQLite layout, placement, revision, session-record, sync, and backup behavior.
- Capability composition tests covering unsupported, unavailable, forbidden, conflicted, and available actions.
- Branch-safe launch tests, including dirty-tree mismatch, idempotent double launch, and server-side revalidation.
- Session reconciliation tests proving no prompt, argument, grant, output, or buffer enters durable storage.
- Verification fingerprint tests for commit, branch, staged, unstaged, untracked, configuration, and during-run changes.
- Resolver tests for rename/delete staleness, forbidden references, new unplaced work, and session live-binding state.
- Existing terminal limit, lease, grant, cancellation, and shutdown tests remain authoritative and unchanged.

## Design Decisions

| Decision                                      | Rationale                                                                 |
|-----------------------------------------------|---------------------------------------------------------------------------|
| One default layout per workspace and branch   | Keeps first-release execution context explicit and bounded.               |
| Placement-level revisions                     | Ordinary drags do not conflict with unrelated nodes or viewport changes.  |
| Repository interfaces hide datastore drivers  | Local and future Cloud storage do not change Canvas domain behavior.      |
| Capability resolver composes independent axes | Provider combinations evolve without mode-specific Canvas code.           |
| Durable session record plus ephemeral binding | Restores honest work history while keeping terminal secrets in memory.    |
| Server-side branch validation                 | UI state cannot authorize execution in the wrong checkout.                |
| Conservative verification fingerprints        | False-stale is safer than a false-current green result.                   |
| No persisted or authored edges in MVP         | Relationship meaning remains owned by repository and application domains. |
