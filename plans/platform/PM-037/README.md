# PM-037: Terminal Canvas Workspace Orchestrator

## Overview

PM-037 adds a spatial Canvas that organizes Kode Stream work as workspace -> plan -> terminal workbench. Users can pan,
zoom, arrange plans, launch or focus embedded AI sessions, inspect files and diffs, and see verification state without
leaving the Canvas. Canvas layout is app-owned metadata that references Git-backed work instead of copying repository
content. Local and Agent-Backed Cloud workspaces support the full execution flow. Remote Snapshot workspaces provide a
read-only planning surface with local or Agent connection guidance instead of process execution.

## Scope

### Goals

- Add a workspace-scoped infinite Canvas with semantic workspace, plan, terminal-session, and artifact nodes.
- Generate a useful default layout from the selected workspace and its indexed plans.
- Open the selected node in a resizable workbench for terminal, plan, files, diff, Git, and verification views.
- Launch an embedded AI session from a plan node and preserve its relationship to the plan while it is active.
- Persist Canvas documents through the existing app-state storage boundary.
- Support Local `datadir`, Local SQLite `database`, and Cloud Postgres `database` stores.
- Gate editing and execution from runtime and workspace capabilities rather than deployment-name checks.
- Keep terminal output, channel grants, credentials, and repository content outside Canvas persistence.
- Provide keyboard navigation, focus mode, reduced-motion behavior, and recoverable auto-layout.

### Non-Goals

- No unattended background-agent queue or autonomous multi-agent scheduler.
- No terminal or AI execution for Agentless Remote Snapshot workspaces.
- No terminal transcript persistence, replay, or search.
- No repository-side Canvas file by default.
- No real-time multi-user co-editing or CRDT in the first release.
- No arbitrary user-authored node plug-in system.
- No replacement of Workstream, Item Workspace, Knowledge, or the existing terminal dock.
- No provider-side Git mutations for Remote Snapshot workspaces.

## Related Plans

| Item                          | Relationship                 | Key Context                                                                                       |
|-------------------------------|------------------------------|---------------------------------------------------------------------------------------------------|
| [PM-020](../PM-020/README.md) | Embedded terminal foundation | Reuse bounded PTYs, channel grants, reconnect leases, cancellation, and the app-level session UI. |
| [PM-027](../PM-027/README.md) | Session layout foundation    | Reuse movable, resizable, minimized, maximized, and right-panel presentation behavior.            |
| [PM-032](../PM-032/README.md) | Cloud execution boundary     | Agent-Backed Cloud executes terminal, Git, AI, and verification on the user's machine.            |
| [PM-033](../PM-033/README.md) | App-state storage boundary   | Add Canvas repositories for Local data-dir, Local SQLite, and Cloud Postgres.                     |
| [PM-034](../PM-034/README.md) | Agentless workspace boundary | Remote Snapshot nodes are commit-pinned and read-only, with no terminal or AI execution.          |
| [PM-036](../PM-036/README.md) | E2E runbook surface          | Reuse plan-local browser playbooks and durable journey enrichment after implementation.           |

## Glossary

| Term                 | Meaning                                                                                        | Maps To (code)                  |
|----------------------|------------------------------------------------------------------------------------------------|---------------------------------|
| Terminal Canvas      | Spatial workspace surface that connects plans, sessions, and produced artifacts.               | `CanvasPage`                    |
| Canvas Document      | App-owned persisted viewport, nodes, edges, and display preferences.                           | `canvas.Document`               |
| Canvas Node          | Positioned reference to a workspace, plan, terminal session, artifact, note, or group.         | `canvas.Node`                   |
| Entity Reference     | Stable type and ID link from a Canvas node to an existing Kode Stream entity.                  | `canvas.EntityRef`              |
| Semantic Zoom        | Detail policy that changes node content by zoom level instead of only scaling it.              | `CanvasDetailLevel`             |
| Terminal Workbench   | Resizable detail surface that hosts the selected terminal or existing plan/file tooling.       | `TerminalWorkbench`             |
| Session Summary      | Non-sensitive lifecycle metadata for an active embedded session; never terminal output.        | `ai.Session` projection         |
| Capability Snapshot  | Effective actions allowed by runtime role, workspace access mode, and Agent availability.      | `CanvasCapabilities`            |
| Remote Snapshot Node | Read-only node backed by provider content at an immutable resolved commit.                     | `remote_snapshot` workspace     |
| Stale Reference      | Canvas entity reference whose source workspace, plan, session, or artifact no longer resolves. | Canvas resolution warning state |

## Components

| Layer    | Component                 | Purpose                                                                                             |
|----------|---------------------------|-----------------------------------------------------------------------------------------------------|
| Domain   | `internal/canvas`         | Validate documents, node kinds, entity references, ownership, limits, and optimistic versions.      |
| Storage  | Canvas repositories       | Persist app-owned documents in `canvases.yaml`, SQLite, or Postgres and include them in sync.       |
| Service  | Canvas service            | Resolve the default workspace Canvas and enrich references with current plan/session state.         |
| API      | Canvas endpoints          | List, create, read, update, delete, and resolve Canvas documents with capability-aware responses.   |
| AI       | Active session projection | List safe session summaries so Canvas terminals survive navigation and can reconnect while alive.   |
| Frontend | Canvas feature            | Render React Flow viewport, semantic nodes, connections, selection, focus mode, and auto-layout.    |
| Frontend | Terminal workbench        | Reuse the embedded terminal and item tooling in a Canvas-owned detail surface.                      |
| Frontend | Capability presentation   | Disable unsupported actions and explain Agentless, offline-Agent, role, and stale-reference states. |

## Data Flow

> Canvas route -> resolve workspace Canvas -> Canvas repository -> saved document -> entity resolver -> current
> workspace, item, session, Git, and verification projections -> React Flow nodes -> selected node -> contextual
> workbench -> explicit file/Git/session action -> existing guarded service.

> Layout change -> debounced versioned Canvas update -> selected app-state repository -> updated document version.

> Plan launch -> existing embedded AI launch -> safe active-session summary -> terminal node relationship -> existing
> bounded WebSocket channel -> Terminal Workbench. Terminal bytes never enter the Canvas document.

## Support Matrix

| Deployment / workspace mode     | App-state store       | Canvas layout | Plan content                | Terminal / AI workbench            |
|---------------------------------|-----------------------|---------------|-----------------------------|------------------------------------|
| Local application               | `datadir`             | Full          | Read and guarded write      | Full, on the local machine         |
| Local application               | SQLite `database`     | Full          | Read and guarded write      | Full, on the local machine         |
| Cloud Agent-Backed              | Postgres `database`   | Full          | Through owner Cloud Agent   | Full when owner Agent is connected |
| Cloud Agentless Remote Snapshot | Postgres `database`   | Full metadata | Commit-pinned and read-only | Unavailable; show handoff guidance |
| Local Docker                    | Selected local option | Full          | Mounted workspace           | Runs inside the container          |

Canvas layout edits are app-state writes. They remain available for an editable Canvas containing read-only Remote
Snapshot entities. Repository mutations and process actions are independently disabled.

## Design Decisions

| Decision                                      | Alternatives Considered                            | Rationale                                                                                       |
|-----------------------------------------------|----------------------------------------------------|-------------------------------------------------------------------------------------------------|
| Persist references, not copied entity content | Store plan/session snapshots in every node         | Git, indexes, and runtime services remain authoritative and Canvas data stays small.            |
| Reuse `@xyflow/react`                         | Build pan, zoom, selection, and edges from scratch | The dependency and Knowledge Graph patterns already exist in the frontend.                      |
| Start workspace-scoped                        | Ship cross-workspace free-form canvases first      | It proves workspace -> plan -> terminal orchestration with clear ownership and bounded queries. |
| Keep the existing views                       | Replace Workstream and Item Workspace              | Canvas is an orchestrator; established detailed workflows remain reusable.                      |
| Persist one versioned document blob           | Normalize every node and edge into separate rows   | Matches current JSON-backed stores and keeps v1 migrations and data-dir parity manageable.      |
| Use optimistic document versions              | Last writer silently wins                          | Cloud users receive a visible conflict instead of losing another update.                        |
| Persist safe session references only          | Persist transcript or PTY buffer                   | Preserves the existing terminal secrecy and lifecycle boundaries.                               |
| Derive actions from capabilities              | Branch UI directly on Local/Cloud mode names       | Supports Local, Agent-Backed, Remote Snapshot, roles, and future adapters consistently.         |
| Make semantic edges typed                     | Allow decorative untyped connections               | Typed relationships remain understandable and can drive focus/filter behavior.                  |
| Auto-layout is recoverable                    | Force manual arrangement                           | New users get immediate value while saved spatial memory remains under user control.            |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [UI Automation](automation/README.md)
- [Implementation Plan](implementation-plan.md)
