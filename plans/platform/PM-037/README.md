# PM-037: Terminal Canvas Workspace Orchestrator

## Overview

PM-037 adds a focused spatial workbench for the daily workspace -> plan -> session loop. Users can arrange workspace,
plan, and durable session nodes; launch a branch-safe embedded terminal from a plan; inspect Git and verification state;
and return to the same layout. Canvas remains a projection of real workspace state. It owns placements and presentation
only, while repositories, Git, verification, session metadata, and live process managers remain authoritative for their
domains.

PM-037 establishes the foundation for a larger Canvas product without shipping a general graph editor in the first
release. Groups, notes, artifacts, custom edges, multiple canvases, snapshots, and collaborative layouts remain explicit
follow-up investments.

> **Branch-context follow-up:** [PM-038](../PM-038/README.md) makes Canvas resolve the current checkout itself before
> indexing and seeding plan nodes. Canvas no longer accepts a page-selected snapshot branch; non-checkout plans are
> reviewed separately.

## MVP Outcome

A successful first release proves this loop:

> Open a workspace Canvas -> arrange workspace, plan, and session nodes -> launch a terminal on the correct branch ->
> inspect Git state -> run verification -> see verification become stale after repository change -> reload and restore
> the saved arrangement.

## Scope

### Goals

- Add one default Canvas for each workspace and branch context.
- Make workspace, plan, and durable session nodes draggable.
- Persist node placements independently from the entities they reference.
- Restore saved placements while resolving titles, status, capabilities, Git state, verification freshness, and live
  process availability from their authoritative services.
- Launch an embedded terminal or AI session only after server-side workspace and branch validation.
- Separate durable session metadata from live terminal processes, channel grants, and terminal bytes.
- Record the repository fingerprint verified by each result and report whether that result is fresh or stale.
- Derive actions from current workspace capabilities instead of deployment or access-mode names.
- Support the current Local data-dir and SQLite app-state stores through one Canvas repository boundary.
- Provide clear keyboard access, save state, reset-layout recovery, stale-reference handling, and non-color status cues.

### Non-Goals For PM-037

- No group, note, or artifact nodes.
- No user-authored edges or arbitrary relationship editing.
- No multiple canvases, canvas copies, or collaborative layout merging.
- No cross-workspace or multi-branch Canvas.
- No automatic branch switching or worktree creation.
- No Agent-Backed Cloud execution delivery; the contracts must allow it without mode branching.
- No Agentless Remote Snapshot Canvas UX; snapshot-backed content is a separate provider capability phase.
- No full Item Workspace, Workstream, Jira, file editor, or diff UI embedded in Canvas.
- No transcript persistence, replay, or search.
- No unattended agent queue or autonomous multi-agent scheduler.
- No repository-side Canvas file.
- No touch-first mobile editor or arbitrary node plug-in system.

## Product Direction After PM-037

| Stage | Direction                             | Candidate Capabilities                                                                |
|-------|---------------------------------------|---------------------------------------------------------------------------------------|
| 1     | Dependable personal workbench         | PM-037 MVP workflow                                                                   |
| 2     | Execution orchestrator                | Cloud Agent execution, worktrees, richer session history, verification lineage        |
| 3     | Knowledge and coordination surface    | Groups, notes, artifacts, derived relationships, Canvas-only links, multiple canvases |
| 4     | Collaborative engineering environment | Shared layouts, presence, ownership, handoff, and multi-agent visibility              |

Later stages must reuse the PM-037 provider, capability, placement, session, and verification contracts rather than add
deployment-specific branches to Canvas components.

## Related Plans

| Item                          | Relationship                    | Key Context                                                                                   |
|-------------------------------|---------------------------------|-----------------------------------------------------------------------------------------------|
| [PM-020](../PM-020/README.md) | Embedded terminal foundation    | Reuse bounded PTYs, grants, reconnect leases, cancellation, and shutdown behavior.            |
| [PM-027](../PM-027/README.md) | Session presentation foundation | Reuse the existing terminal surface without introducing a second process owner.               |
| [PM-032](../PM-032/README.md) | Cloud execution boundary        | Future Agent execution stays on the user's machine and routes through the Agent.              |
| [PM-033](../PM-033/README.md) | App-state storage boundary      | Add Canvas placements and session records through provider-selected repositories.             |
| [PM-034](../PM-034/README.md) | Remote Snapshot boundary        | Snapshot content and missing execution are separate provider capabilities, not a Canvas mode. |
| [PM-036](../PM-036/README.md) | E2E runbook surface             | Reuse provider-neutral playbooks and enrich durable journey coverage after implementation.    |

## Glossary

| Term                   | Meaning                                                                           | Maps To                              |
|------------------------|-----------------------------------------------------------------------------------|--------------------------------------|
| Terminal Canvas        | Spatial workbench for workspace, plan, and session orchestration.                 | `CanvasPage`                         |
| Canvas Layout          | App-owned identity and branch-scoped collection of placements.                    | `canvas.Layout`                      |
| Placement              | Canvas-local position and presentation for an entity reference.                   | `canvas.Placement`                   |
| Entity Reference       | Branch-aware pointer to an existing workspace, plan, or durable session.          | `canvas.EntityRef`                   |
| Workspace Context      | Workspace plus branch/ref and observed repository revision used by the Canvas.    | `canvas.WorkspaceContext`            |
| Action Capability      | Current action state with availability and a machine-readable denial reason.      | `workspace.ActionCapability`         |
| Session Record         | Durable, non-sensitive metadata describing requested and completed terminal work. | `ai.SessionRecord`                   |
| Process Binding        | In-memory PTY, process, subscribers, buffer, grants, and reconnect timers.        | Existing terminal manager            |
| Repository Fingerprint | Branch, HEAD, index, worktree, and verification configuration identity.           | `verification.RepositoryFingerprint` |
| Fresh Result           | Verification result whose fingerprint still matches the repository.               | Resolved verification projection     |
| Stale Reference        | Placement whose referenced entity no longer resolves or is no longer accessible.  | Placement resolution warning         |

## Concern Boundaries

| Concern                    | Examples                                          | Responsibility                                    |
|----------------------------|---------------------------------------------------|---------------------------------------------------|
| Deployment topology        | Local app, Cloud control plane                    | Authentication, ownership, and request routing    |
| Workspace content provider | Local checkout, Agent checkout, provider snapshot | Repository reads and Git state                    |
| Execution provider         | Local process, connected Agent, none              | Terminal, AI, runtime, and verification execution |
| App-state datastore        | Data directory, SQLite, Postgres                  | Canvas placement and durable metadata persistence |
| Authorization              | Local policy, Cloud role                          | What the current user may do                      |

Canvas may observe these axes through resolved capabilities, but domain and frontend components must not infer behavior
from deployment, access-mode, or datastore names.

## Capability States

| State         | Meaning                                                         | Example                                         |
|---------------|-----------------------------------------------------------------|-------------------------------------------------|
| `available`   | The action is supported, authorized, and ready now.             | Local terminal launch on the checked-out branch |
| `unavailable` | The action is supported but temporarily cannot run.             | Future owner Agent is offline                   |
| `unsupported` | The selected providers cannot perform the action.               | Provider snapshot with no execution provider    |
| `forbidden`   | The provider supports the action but the user lacks permission. | Viewer attempts layout edit                     |
| `conflicted`  | The action needs context recovery before it can run.            | Plan branch differs from current checkout       |

Capability responses include a stable action name, state, reason code, and safe recovery guidance. The backend
revalidates every action at execution time; frontend capability state is advisory and may change.

## Data Ownership

| Data                                                  | Authority                 | Canvas Persistence               |
|-------------------------------------------------------|---------------------------|----------------------------------|
| Plan content, title, status, repository relationships | Repository and item index | Reference only                   |
| Branch, commit, dirty state, changed files            | Git                       | Never                            |
| Verification result and fingerprint                   | Verification domain       | Reference/status projection only |
| Safe session lifecycle metadata                       | Session record repository | Reference only                   |
| PTY, process, grants, output, prompt, arguments       | Live process manager      | Never                            |
| Node position and collapsed presentation              | Canvas repository         | Yes                              |
| Current resolved labels and capabilities              | Resolver                  | Never                            |

## Node Movement Semantics

- Moving any workspace, plan, or session node changes only that node's placement.
- A workspace node is a semantic anchor, not a spatial parent. Moving it never moves plan or session nodes.
- Repository containment does not create drag parenting.
- Removing a placement never deletes or mutates the referenced entity or live process.
- A removed placement remains hidden for that layout and is never recreated by automatic placement.
- Newly discovered entities receive deterministic positions silently without moving saved nodes or showing a prompt.
- Reset layout previews a deterministic placement set before replacing saved positions.
- Future groups will be explicit visual frames: moving a group applies a delta to members, deleting it leaves members in
  place, and overlap alone never creates membership.

## Relationship Provenance

| Origin        | Owner                          | MVP Behavior                                            |
|---------------|--------------------------------|---------------------------------------------------------|
| `repository`  | Repository or item index       | Derived, read-only, and not persisted by Canvas         |
| `application` | Session or verification domain | Derived from durable application metadata               |
| `canvas`      | Canvas layout                  | Reserved for later visual links; not authored in PM-037 |

PM-037 may render workspace-to-plan and plan-to-session connections for orientation. Users cannot edit them, and
hiding or removing a placement does not mutate the relationship source.

## Data Flow

> Canvas route -> resolve workspace and branch context -> load placements -> resolve workspace, plan, session, Git, and
> verification projections -> attach current action capabilities -> render nodes -> select node -> open the focused
> workspace/plan panel, select a session without changing its presentation, or use the session's top-right action to
> expand its terminal in place.

> Drag end -> update affected placement locally -> debounced placement patch with expected placement revision -> Canvas
> repository -> saved state.

> Plan launch -> server revalidates execution capability and checkout branch -> create durable session record -> start
> ephemeral process binding -> attach terminal channel -> update safe lifecycle metadata.

> Verification -> capture start fingerprint -> execute -> capture completion fingerprint -> store result fingerprint ->
> compare with current repository fingerprint on every read -> return fresh, stale, or inconclusive status.

## Design Decisions

| Decision                                                   | Alternatives Considered                         | Rationale                                                                                |
|------------------------------------------------------------|-------------------------------------------------|------------------------------------------------------------------------------------------|
| Ship a focused workbench MVP                               | General graph editor in the first release       | Validates the daily orchestration loop before expanding node and relationship authoring. |
| Separate topology, providers, datastore, and authorization | Deployment-mode support matrix                  | Supports new provider combinations without spreading mode checks.                        |
| Persist placements separately from entities                | Persist resolved graph snapshots                | Preserves source-of-truth boundaries and avoids stale copied data.                       |
| Patch affected placements                                  | Replace one large Canvas document               | Reduces save payloads and unrelated drag conflicts.                                      |
| Make the default Canvas branch-scoped                      | Mix branches in one initial graph               | Gives terminal launch a clear execution context and bounds the first workflow.           |
| Block branch-mismatched launch                             | Silently launch or switch checkout              | Protects user trust and repository safety.                                               |
| Separate session records from process bindings             | Treat in-memory terminal sessions as durable    | Restores honest lifecycle metadata without persisting terminal secrets.                  |
| Fingerprint verification inputs                            | Keep the last result green until rerun          | Prevents stale verification from appearing current.                                      |
| Defer groups and custom edges                              | Ship generic spatial authoring immediately      | Avoids ambiguous movement and relationship semantics.                                    |
| Treat snapshots as provider composition                    | Treat Agentless as a restricted deployment mode | Allows future read and execution capability combinations independently.                  |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [UI Automation](automation/README.md)
- [Implementation Plan](implementation-plan.md)
