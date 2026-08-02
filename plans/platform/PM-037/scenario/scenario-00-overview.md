# Scenarios: PM-037 Overview

## Scenario List

| #   | Title                            | Description                                                                             |
|-----|----------------------------------|-----------------------------------------------------------------------------------------|
| 0   | First Canvas                     | User opens a workspace that has no saved Canvas document.                               |
| 1   | Plan-to-terminal orchestration   | User launches and operates an embedded AI session from a plan node.                     |
| 2   | Restore spatial workspace        | User returns and receives saved positions plus current entity state.                    |
| 3   | Agent-Backed Cloud execution     | Cloud user works through a connected owner Agent and handles Agent loss.                |
| 4   | Agentless Remote Snapshot        | Cloud user explores commit-pinned plans without repository writes or process execution. |
| 5   | Concurrent or stale Canvas state | User recovers from document conflicts, deleted entities, and ended terminal sessions.   |
| 6   | Accessible Canvas navigation     | Keyboard and reduced-motion users navigate, focus, and operate Canvas nodes.            |

## Scenario 0: First Canvas

### Goal

> Give the user an immediately useful workspace map without requiring manual node creation.

### Starting State

| #   | Area        | State                                                                          |
|-----|-------------|--------------------------------------------------------------------------------|
| 1   | Workspace   | A registered workspace is selected and its plans are indexed.                  |
| 2   | App state   | No Canvas document exists for the user and workspace.                          |
| 3   | Sessions    | Zero or more embedded sessions may already be active.                          |
| 4   | Permissions | User can read the workspace; layout editing follows the user's app-state role. |

### Visual State Before

> Kode Stream -> Workstream is selected -> no Canvas route or spatial arrangement exists.

### Flow 0.1: Resolve Default Workspace Canvas

1. User selects **Canvas** in the Workspace navigation.
2. Frontend resolves the current user's default Canvas for the selected workspace.
3. Backend creates a minimal document when none exists and returns its version.
4. Frontend loads current workspace, plan, and safe active-session projections.
5. Auto-layout places the workspace node first, plans by status, and active sessions next to their plans.
6. The viewport fits visible content and announces that the Canvas is ready.

### Visual State After

> Workspace node -> plan nodes grouped by status -> active session nodes attached to their plan -> selected node opens
> in the Workbench.

### Edge Cases

- With no indexed plans, show the workspace node, an explanatory empty state, and a link to scan or configure sources.
- If the workspace is unavailable, retain saved positions and show a recoverable workspace warning.
- If document creation is forbidden, render an unsaved read-only projection without offering layout changes.
- Auto-layout never overwrites a previously saved manual layout without confirmation.

## Scenario 1: Plan-To-Terminal Orchestration

### Goal

> Start an embedded AI session from a plan and keep execution context visibly connected to that plan.

### Starting State

| #   | Area       | State                                                          |
|-----|------------|----------------------------------------------------------------|
| 1   | Canvas     | A workspace Canvas contains at least one editable plan node.   |
| 2   | Runtime    | Effective capabilities allow terminal and AI actions.          |
| 3   | AI tooling | A supported provider is installed, enabled, and authenticated. |

### Flow 1.1: Launch From Plan Node

1. User selects a plan node and chooses **Open AI session**.
2. Existing launch controls collect provider, prompt, context, and terminal surface.
3. Backend validates workspace containment and starts the existing bounded embedded session.
4. Canvas adds a terminal node with an `implements` relationship to the plan.
5. Terminal Workbench opens and attaches to the granted WebSocket channel.
6. Lifecycle state changes are reflected on the node and in the Workbench.

### Flow 1.2: Inspect Produced Work

1. User switches the selected node from terminal to plan or artifact.
2. Workbench reuses existing file, Markdown, diff, Git, and verification components.
3. Canvas keeps the terminal transport alive while another node is selected.
4. User returns to the terminal node and reconnects within the existing lease rules.

### Flow 1.3: Finish Or Cancel

1. A session exits, fails, or the user confirms cancellation.
2. Canvas shows the terminal node's final lifecycle state.
3. User may remove the transient node without deleting any repository content.
4. The document may retain only the safe session reference and outcome until the user removes it.

### Edge Cases

- Launch errors remain attached to the initiating plan and do not create a running terminal node.
- Closing an active terminal requires the existing confirmation; hiding a node does not cancel its process.
- Losing the browser channel follows PM-020 reconnect and lease behavior.
- Terminal bytes and grant tokens are never included in Canvas save requests.

## Scenario 2: Restore Spatial Workspace

### Goal

> Restore the user's spatial arrangement while refreshing every referenced entity from its authoritative source.

### Flow 2.1: Reload Saved Canvas

1. User revisits the Canvas route.
2. Frontend loads the saved document, viewport, and display preferences.
3. Backend resolves references against current workspaces, item index, and active sessions.
4. Frontend preserves saved positions and updates labels, status, branch, health, and lifecycle badges.
5. Unresolved references render as stale nodes instead of disappearing silently.

### Edge Cases

- A renamed plan keeps its position because its stable item ID still resolves.
- A deleted plan becomes a stale reference with **Remove from Canvas** and **Locate replacement** actions.
- An ended or expired session cannot reconnect and shows its last safe lifecycle summary.
- A reset operation previews the new auto-layout and requires confirmation before replacing manual positions.

## Scenario 3: Agent-Backed Cloud Execution

### Goal

> Use the full Canvas and Terminal Workbench while privileged work remains on the user's machine.

### Starting State

| #   | Area    | State                                                                      |
|-----|---------|----------------------------------------------------------------------------|
| 1   | Cloud   | User is authenticated and owns or can access an Agent-Backed workspace.    |
| 2   | Storage | Canvas document is stored in Postgres under the authenticated owner scope. |
| 3   | Agent   | Owner Cloud Agent may be connected or offline.                             |

### Flow 3.1: Connected Agent

1. Browser loads Canvas metadata and indexed plan state from the Cloud API.
2. User launches a terminal or AI action from a plan node.
3. Cloud API authorizes the action and routes it through the owner Agent channel.
4. Agent executes in the local workspace and streams safe lifecycle/output frames back to the browser.
5. Postgres stores layout and safe entity references, never repository files or terminal content.

### Flow 3.2: Agent Goes Offline

1. Agent availability changes while the Canvas is open.
2. Terminal, Git, file mutation, AI, runtime, and verification actions become unavailable.
3. Existing nodes and read models remain visible with an **Agent offline** explanation.
4. After reconnection and capability refresh, supported actions become available without recreating the Canvas.

## Scenario 4: Agentless Remote Snapshot

### Goal

> Explore and organize a provider snapshot without implying that Cloud can run commands or mutate the repository.

### Flow 4.1: Inspect Snapshot Canvas

1. User opens a Canvas for a `remote_snapshot` workspace.
2. Backend resolves workspace and plan data at the stored immutable commit.
3. Frontend labels the workspace as **Remote Snapshot** and shows the selected ref and abbreviated commit.
4. User may arrange nodes, add app-owned notes, open read-only plan content, and save Canvas layout.
5. Terminal and AI actions are replaced by **Connect Agent** or **Open locally** guidance.

### Edge Cases

- Canvas layout remains editable when the user's role permits app-state writes, even though repository content is read-only.
- Missing provider authorization shows stale snapshot metadata and recovery guidance.
- A new provider commit does not mutate an existing snapshot node; selecting a new snapshot refreshes its entity reference.
- No terminal placeholder may be represented as running or connected.

## Scenario 5: Concurrent Or Stale Canvas State

### Goal

> Avoid silent layout loss when app state changes concurrently or entity references expire.

### Flow 5.1: Optimistic Version Conflict

1. Two browser views load the same document version.
2. First view saves and receives the next version.
3. Second view attempts to save the old version and receives a conflict.
4. Frontend pauses automatic saving and offers **Reload latest** or **Keep my layout as a copy**.
5. No update silently overwrites the newer document.

### Limits And Validation

- Reject duplicate node IDs, invalid node/edge kinds, non-finite coordinates, dangling edge endpoints, and oversized documents.
- Bound document name, note text, node count, edge count, viewport, and serialized document size.
- Deleting a workspace does not cascade-delete a Canvas without an explicit app-state cleanup policy.
- Deleting a Canvas never deletes a plan, workspace, session, file, or Git object.

## Scenario 6: Accessible Canvas Navigation

### Goal

> Operate the Canvas without precise pointer input and without relying on animation or color alone.

### Flow 6.1: Keyboard Focus

1. User tabs to the Canvas toolbar and chooses **Search nodes**.
2. User filters by workspace, plan, or session name.
3. Selecting a result focuses the node, updates an accessible status region, and opens its Workbench.
4. Arrow or documented shortcut actions move between connected nodes.
5. Escape returns focus from Workbench controls to the selected node.

### Acceptance Notes

- Every node exposes its kind, name, lifecycle/status, and available actions through an accessible label.
- Selection, blocked state, and session lifecycle use text/icon cues in addition to color.
- Reduced-motion preference removes animated edges and uses static running indicators.
- Zoom controls, fit-to-content, focus mode, auto-layout, and reset are available as visible buttons.
