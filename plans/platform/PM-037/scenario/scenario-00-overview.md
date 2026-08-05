# Scenarios: PM-037 Focused Terminal Canvas

## Scenario List

| #   | Title                                   | Description                                                                 |
|-----|-----------------------------------------|-----------------------------------------------------------------------------|
| 0   | First branch-scoped Canvas              | Resolve one default layout and render current plans in workspace context.   |
| 1   | Arrange and restore work                | Move plan and session nodes, then restore their positions.                  |
| 2   | Branch-safe plan launch                 | Launch once on a matching checkout and block launch after branch mismatch.  |
| 3   | Durable session and live process        | Keep safe session history distinct from reconnectable terminal state.       |
| 4   | Git and verification freshness          | Show current Git state and invalidate verification after repository change. |
| 5   | Capability and stale-reference recovery | Explain unavailable actions and missing entities without mode-specific UI.  |
| 6   | Keyboard and narrow-window operation    | Operate essential Canvas actions without precise pointer input.             |

## Scenario 0: First Branch-Scoped Canvas

### Goal

> Give the user a useful spatial workbench for the selected workspace and branch without copying repository data into
> Canvas storage.

### Starting State

| #   | Area          | State                                                   |
|-----|---------------|---------------------------------------------------------|
| 1   | Workspace     | A registered Local workspace is selected.               |
| 2   | Branch        | The selected branch is known and its plans are indexed. |
| 3   | App state     | No Canvas layout exists for this workspace and branch.  |
| 4   | Authorization | The user can read entities and edit app-owned layout.   |

### Flow 0.1: Resolve Default Layout

1. User opens **Workbench** from Workspace navigation.
2. Backend resolves the selected workspace and branch context.
3. Backend creates one default layout when none exists.
4. Resolver projects the workspace and current branch plans with capabilities and current state.
5. Frontend assigns deterministic suggested positions and persists accepted placements.
6. The viewport fits visible content and announces that Canvas is ready.

### Expected State

> Current-branch plan nodes are visible inside workspace and service presentation sections. There are no note,
> artifact, or editable-edge controls.

### Edge Cases

- With no plans, show existing scan/source guidance within the workspace context.
- A failure to read Git state does not delete or rearrange placements.
- A user without layout permission receives a read-only projected layout.
- Changing the selected branch resolves another branch-scoped layout instead of mixing plans into the current layout.

## Scenario 1: Arrange And Restore Work

### Goal

> Preserve spatial memory while keeping node movement independent from entity and relationship state.

### Starting State

| #   | Area       | State                                                        |
|-----|------------|--------------------------------------------------------------|
| 1   | Canvas     | Workspace and at least two plan nodes have saved placements. |
| 2   | Session    | A durable session record may have a placement.               |
| 3   | Repository | Repository entities and relationships remain authoritative.  |

### Flow 1.1: Move Every Relevant Node Kind

1. User moves a plan node and then a session node.
2. Frontend patches only the changed placements with expected revisions.
3. Save state changes from **Saving** to **Saved**.
4. User reloads the page.
5. Backend resolves current entity state while frontend restores the saved coordinates.

### Flow 1.2: New And Removed Work

1. A repository scan discovers a new plan.
2. Canvas silently persists a deterministic position for the new plan without moving saved nodes.
3. No placement action, new-node count, or notification is shown.
4. User removes a plan placement.
5. The plan remains in the repository, while its hidden placement prevents automatic placement from recreating it.

### Flow 1.3: Placement Conflict

1. Two views move the same node from the same placement revision.
2. The first save succeeds.
3. The second receives `canvas_placement_conflict` for that node only.
4. Unrelated node positions remain saved and interactive.
5. User reloads the affected position or explicitly reapplies their position to the latest revision.

### Edge Cases

- Viewport updates do not conflict with node placement updates.
- Removing a running session placement does not cancel its process.
- Reset layout shows a preview and requires confirmation.
- Background refresh never runs auto-layout over saved positions.

## Scenario 1.4: Organize Work With Presentation Sections

1. Canvas generates a workspace section and service sections from the resolved plan set.
2. User can group plans by service and cycle the service grid between one and four columns.
3. User Cmd/Ctrl-clicks or box-selects at least two nodes, chooses **Create section**, and supplies a name.
4. Dragging the section header moves all of its member placements by the same delta.
5. Removing the section removes only the browser-local frame; its member placements remain where they are.

### Acceptance Notes

- Section definitions are browser-local presentation state for the Canvas layout.
- Sections do not create hierarchy, entity ownership, or persisted Canvas relationships.
- Filtering and search can tighten visible section bounds without modifying saved node positions.

## Scenario 2: Branch-Safe Plan Launch

### Goal

> Start one embedded terminal in the branch represented by the selected plan and never silently execute elsewhere.

### Starting State

| #   | Area       | State                                                               |
|-----|------------|---------------------------------------------------------------------|
| 1   | Plan       | A plan node references the current checkout branch.                 |
| 2   | Capability | `terminal.launch` is available.                                     |
| 3   | Tooling    | A supported terminal or AI provider is installed and authenticated. |

### Flow 2.1: Matching Branch Launch

1. User selects a plan and chooses **Open session**.
2. Frontend submits workspace, plan, expected branch, observed commit, provider choice, and one idempotency key.
3. Backend resolves the plan and rechecks current checkout, repository context, authorization, provider, and session
   limits.
4. Backend creates a durable session record.
5. Backend starts one ephemeral process binding.
6. Canvas places only the new session at a deterministic position when it is not already placed.
7. Canvas selects the new session and explicitly opens its node into the granted terminal channel.

### Flow 2.2: Double Submission

1. User double-clicks launch or the client retries after an uncertain response.
2. The same idempotency key reaches the backend more than once.
3. Backend returns the original session record and never starts a second process.

### Flow 2.3: Branch Changes Before Launch

1. Plan node was resolved for `feature/PM-037`.
2. Current checkout changes to `main` before the launch request is handled.
3. Backend rejects the request with `terminal_branch_mismatch`.
4. Canvas shows expected branch, current branch, and available dirty-state guidance.
5. No session record, placement, or process is created.
6. User may open existing branch controls or refresh Canvas.

### Edge Cases

- Canvas never switches branches automatically.
- A dirty or conflicted tree does not produce an unsafe one-click branch-switch action.
- A stale or forbidden plan cannot launch.
- Launch failure after record creation updates the record to `failed` without storing command arguments or prompt.

## Scenario 3: Durable Session And Live Process

### Goal

> Restore honest session history without treating an in-memory terminal process as durable.

### Flow 3.1: Select Independently From Terminal Disclosure

1. User launches a session and receives a running durable record plus live binding.
2. User selects another plan node; the expanded terminal remains unchanged.
3. The process continues under existing terminal lifecycle rules.
4. User selects the session node; selection alone neither expands nor collapses it.
5. User chooses the session's top-right expand or collapse action.
6. Canvas persists the disclosure state and reuses the same session; it does not relaunch or create another terminal
   owner.

### Flow 3.2: Page Reload

1. Browser reloads while the backend process remains alive.
2. Canvas reloads the durable session record and detects its live binding.
3. Existing grant and reconnect rules determine whether terminal attachment can resume.
4. The saved session placement remains unchanged.

### Flow 3.3: Application Restart

1. Application stops while a durable record is `running`.
2. Live process bindings and grants are lost.
3. On startup, reconciliation marks the record `interrupted`.
4. Canvas restores the session placement and explains that metadata survived but the terminal cannot reconnect.
5. User may remove the placement or launch a new session from the linked plan.

### Edge Cases

- Cancelling a process updates the durable record but leaves its placement.
- Removing a placement leaves the process and record unchanged.
- Active sessions not yet placed remain visible through **Active sessions**.
- Terminal bytes, input, prompt, arguments, grants, and credentials never appear in Canvas or session-record writes.

## Scenario 4: Git And Verification Freshness

### Goal

> Show whether a verification result applies to the repository state the user is currently viewing.

### Starting State

| #   | Area       | State                                                           |
|-----|------------|-----------------------------------------------------------------|
| 1   | Workspace  | Current branch and Git status are readable.                     |
| 2   | Plan       | Verification is configured and `verification.run` is available. |
| 3   | Repository | Repository state is stable at launch.                           |

### Flow 4.1: Fresh Passing Result

1. User selects the plan and starts verification.
2. Backend captures branch, HEAD, index, worktree, and verification configuration fingerprints.
3. Verification runs and captures a completion fingerprint.
4. Start and completion fingerprints match and the run passes.
5. Canvas shows **Passed · current** with an abbreviated verified revision.

### Flow 4.2: Repository Changes After Pass

1. User modifies, stages, creates, removes, or commits a relevant file, or changes branch.
2. Current repository fingerprint changes.
3. Resolver compares it with the completed verification fingerprint.
4. Canvas shows **Passed · stale** and removes current-success emphasis.
5. User may rerun verification when the capability is available.

### Flow 4.3: Repository Changes During Verification

1. Verification starts with one fingerprint.
2. Relevant repository state changes before it completes.
3. Completion fingerprint differs from the start fingerprint.
4. Canvas shows **Inconclusive · repository changed during run**, even if the command exited successfully.

### Edge Cases

- Verification configuration or selected-spec changes invalidate the previous result.
- Relevant untracked files participate in freshness.
- A fingerprint failure is inconclusive, never fresh.
- Node position remains unchanged while Git and verification projections refresh.
- Application restart does not restore verification jobs in PM-037; durable verification lineage is follow-up scope.

## Scenario 4.4: Plan Workbench Details And Quality

1. User selects a plan node and opens the **Info**, **Jira**, or **Quality** tab in the Workbench.
2. **Info** loads current item details and may save title, source, item identifier, status, owner, and tags through the
   existing item metadata authority.
3. **Jira** reuses the existing item Jira panel.
4. **Quality** can run smoke or critical verification, rerun the latest result, select automation specs, and run selected
   automation with its saved environment and display mode.
5. Suggested automation specs come from `automation-test` entries in the plan metadata; the Workbench also presents
   existing E2E runbook coverage.

### Acceptance Notes

- Item metadata, Jira data, verification jobs, automation selections, and E2E runbooks retain their existing owners.
- The Workbench refreshes the Canvas projection after an item metadata save or verification action.

## Scenario 5: Capability And Stale-Reference Recovery

### Goal

> Explain why an action cannot run without hard-coding behavior from deployment or access-mode labels.

### Flow 5.1: Capability States

1. Resolver composes provider support, availability, authorization, and branch context.
2. Canvas receives `available`, `unavailable`, `unsupported`, `forbidden`, or `conflicted` for each relevant action.
3. UI enables, disables, hides, or explains the action according to its state and disclosure policy.
4. Backend revalidates the action if the user invokes it.

### Flow 5.2: Stale Plan Reference

1. A referenced plan is renamed, moved, deleted, or no longer accessible.
2. Canvas retains its placement and returns a safe stale or forbidden projection.
3. A candidate replacement may be shown by branch and identifier.
4. Canvas does not silently rebind because current plan IDs are path-derived.
5. User removes the placement or explicitly confirms a replacement.

### Acceptance Notes

- A forbidden reference never exposes its former cached title.
- Datastore, deployment, and access-mode labels do not determine UI action behavior.
- Provider availability changes do not rearrange or delete nodes.
- Agentless Remote Snapshot behavior is not delivered by PM-037; future snapshot UI consumes the same capability states.

## Scenario 6: Keyboard And Narrow-Window Operation

### Goal

> Operate essential Canvas workflows without precise pointer input.

### Flow 6.1: Search And Move

1. User tabs to **Search nodes**.
2. User filters by plan identifier, title, branch, or session state.
3. Selecting a result focuses the node, opening the Workbench for a plan without changing session
   disclosure state.
4. Documented keyboard controls move the node by a bounded increment.
5. Save status is announced.
6. Closing the Workbench or inline terminal returns focus to the selected node or search result.

### Flow 6.2: Narrow Window

1. User opens Canvas in a narrow desktop window.
2. A plan or workspace Workbench opens as an overlay; the session's top-right action expands its terminal within the
   Canvas.
3. User closes the inspector or terminal through its explicit close action.
4. Search and **Fit content** recover nodes that are outside the current viewport.

### Acceptance Notes

- Every node exposes kind, title, branch, lifecycle/status, and blocked state in its accessible name.
- Git, verification freshness, branch mismatch, and session state use text/icon cues in addition to color.
- Reduced-motion preference removes smooth Canvas transitions.
- Essential navigation and launch actions remain available through normal focusable controls.
