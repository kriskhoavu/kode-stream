# UI Playbook 1: Orchestrate A Plan Terminal

## Goal

> Validate that a user can create or restore a workspace Canvas, launch a plan-linked terminal Workbench when execution
> is supported, and receive accurate capability guidance when it is not.

## Preconditions

- A PM-037 feature build is deployed to the supplied environment.
- The account reference and roles are supplied at run time.
- A writable isolated Local workspace contains at least one indexed plan.
- An enabled and authenticated AI provider is available if the terminal launch section will run.
- Optional Cloud data identifies an Agent-Backed workspace and a Remote Snapshot workspace.
- Do not use production workspaces or credentials.

## Runtime Inputs

| Input                     | Required Value                                                      |
|---------------------------|---------------------------------------------------------------------|
| Base URL                  | Supplied at run time                                                |
| Authentication            | Supplied at run time                                                |
| Local workspace           | Name of isolated writable workspace with an indexed plan            |
| Plan                      | Visible plan title or identifier                                    |
| AI provider               | Test-safe authenticated provider; required only for terminal launch |
| Agent-Backed workspace    | Optional workspace and owner Agent controls                         |
| Remote Snapshot workspace | Optional authorized workspace with visible ref and resolved commit  |

## Section A: Create And Restore A Local Canvas

| #   | Agent Action                                                                            | Wait Condition                                          | Expected Result                                                                                                  | Write Class |
|-----|-----------------------------------------------------------------------------------------|---------------------------------------------------------|------------------------------------------------------------------------------------------------------------------|-------------|
| 1   | Navigate to Kode Stream and select the supplied Local workspace.                        | Workspace name is visible in the application shell.     | Workstream or current workspace view loads without an error.                                                     | Read-only   |
| 2   | Choose **Canvas** from Workspace navigation.                                            | Canvas loading state finishes.                          | A workspace Canvas opens; first use creates or resolves the default app-owned Canvas.                            | Safe write  |
| 3   | Inspect the initial spatial view.                                                       | Nodes finish positioning and the viewport fits content. | One workspace node and the indexed plan node are visible; an empty workspace instead shows scan/source guidance. | Read-only   |
| 4   | Select the supplied plan through node search.                                           | The plan node receives visible focus.                   | Node status is announced and the plan Workbench opens with current plan context.                                 | Read-only   |
| 5   | Move the plan node to a distinct position and wait until save status becomes **Saved**. | Save status changes from **Saving** to **Saved**.       | The Canvas remains interactive and reports a successful app-state save.                                          | Safe write  |
| 6   | Reload the page and return to the same Canvas.                                          | Canvas loading and entity resolution finish.            | The moved position and viewport are restored while plan labels/status reflect current indexed state.             | Read-only   |

## Section B: Launch And Operate A Plan Terminal

| #   | Agent Action                                                                            | Wait Condition                                       | Expected Result                                                                                                     | Write Class |
|-----|-----------------------------------------------------------------------------------------|------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------|-------------|
| 1   | Select the supplied plan node and choose **Open AI session**.                           | AI launch dialog is visible.                         | Dialog shows current plan/workspace context and enabled providers.                                                  | Read-only   |
| 2   | Select the supplied provider, embedded surface, and a test-safe prompt; confirm launch. | Session reaches **running** or shows a launch error. | A successful launch opens Terminal Workbench and adds a session node connected to the plan; an error is contextual. | Safe write  |
| 3   | Select the plan node, then return to the session node.                                  | Each selected node updates the Workbench.            | Terminal transport stays active while the Workbench switches; returning does not launch a second process.           | Read-only   |
| 4   | Use the visible terminal controls to change Workbench presentation and then restore it. | Terminal fit and status stabilize after each change. | The same running session remains attached and focus can return to the node.                                         | Read-only   |
| 5   | Close the active session and accept the cancellation confirmation.                      | Session lifecycle changes to cancelled or exited.    | Session node shows a final state; removing its reference does not remove the plan or repository content.            | Safe write  |

## Section C: Capability Boundaries

Run only the subsections for which suitable Cloud runtime inputs were supplied.

| #   | Agent Action                                                                                | Wait Condition                                   | Expected Result                                                                                                                   | Write Class |
|-----|---------------------------------------------------------------------------------------------|--------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------|-------------|
| 1   | Open the supplied Agent-Backed workspace Canvas while its owner Agent is connected.         | Canvas and Agent status finish loading.          | Workspace is labeled Agent-Backed and role-authorized terminal actions are available.                                             | Read-only   |
| 2   | Disconnect or stop the isolated owner Agent using the supplied test control.                | UI reports the owner Agent as offline.           | Existing nodes remain visible; terminal, Git, file mutation, AI, runtime, and verification actions are unavailable with guidance. | Safe write  |
| 3   | Reconnect the owner Agent.                                                                  | UI reports the Agent as connected.               | Supported actions return without recreating or rearranging the Canvas.                                                            | Safe write  |
| 4   | Open the supplied Remote Snapshot workspace Canvas.                                         | Commit-pinned content and Canvas finish loading. | Workspace shows **Remote Snapshot**, selected ref, resolved commit, and read-only plan state.                                     | Read-only   |
| 5   | Inspect the plan node's available actions.                                                  | Capability state is visible.                     | No terminal/AI, file mutation, Git mutation, runtime, or verification action is enabled; Agent/local handoff is visible.          | Read-only   |
| 6   | Move a Remote Snapshot plan node and wait for **Saved**, if the account can edit app state. | Save status settles or role denial appears.      | Layout saves independently of repository read-only state, or role-specific denial explains why it cannot.                         | Safe write  |

## Section D: Keyboard And Recovery

| #   | Agent Action                                                                                      | Wait Condition                           | Expected Result                                                                                     | Write Class |
|-----|---------------------------------------------------------------------------------------------------|------------------------------------------|-----------------------------------------------------------------------------------------------------|-------------|
| 1   | Use only visible keyboard controls to open node search and select a plan.                         | Selected node and Workbench are visible. | Focus order is predictable and the selected kind, title, and state are announced.                   | Read-only   |
| 2   | Enable the browser or OS reduced-motion preference and revisit the Canvas.                        | Canvas finishes rendering.               | Running and relationship states remain understandable without animated edges or smooth transitions. | Read-only   |
| 3   | If a controlled version-conflict fixture is supplied, edit from two views and save the older one. | Conflict message is visible.             | Automatic saving pauses and offers **Reload latest** or **Keep my layout as a copy**.               | Safe write  |

## Evidence

- Capture the initial Canvas after auto-layout.
- Capture the plan-linked running session node and Terminal Workbench without recording sensitive terminal content.
- Capture Agent offline guidance when the optional Agent-Backed section runs.
- Capture Remote Snapshot labels and disabled execution guidance when the optional Agentless section runs.
- Capture version-conflict recovery when the controlled fixture is available.

## Cleanup

- Cancel any terminal session created by this playbook.
- Remove only the test Canvas/session references when safe; do not delete the workspace, plan, branch, or repository.
- Restore the isolated owner Agent if the Agent-Backed section changed its connection state.
- Record skipped optional sections and missing runtime inputs in `results/latest.md`.
