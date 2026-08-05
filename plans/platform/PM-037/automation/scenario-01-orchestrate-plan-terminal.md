# UI Playbook 1: Operate The Focused Terminal Canvas

## Goal

> Validate that a user can arrange and restore plan and session nodes within workspace context; launch one terminal on the correct
> branch; inspect Git state; and see verification become stale after a repository change.

## Preconditions

- A PM-037 feature build is deployed to the supplied non-production environment.
- The supplied account can read the workspace, edit app-owned layout, launch the selected provider, and run
  verification.
- A writable isolated Local Git workspace contains at least one indexed plan on the current checkout branch.
- A second safe branch or a controlled branch-mismatch setup is available.
- The repository can be returned to its starting state after a reversible test file change.
- Do not use production workspaces, credentials, branches, or provider prompts.

## Runtime Inputs

| Input                   | Required Value                                                  |
|-------------------------|-----------------------------------------------------------------|
| Base URL                | Supplied at run time                                            |
| Authentication          | Supplied at run time                                            |
| Local workspace         | Isolated writable workspace name                                |
| Plan                    | Visible plan identifier or title on the current branch          |
| Matching branch         | Current checkout branch containing the plan                     |
| Mismatch branch         | Safe alternate branch or controlled mismatch procedure          |
| AI or terminal provider | Test-safe authenticated provider                                |
| Verification profile    | Deterministic test-safe profile                                 |
| Repository mutation     | Reversible file change relevant to the verification fingerprint |

## Section A: Arrange And Restore The Canvas

| #   | Agent Action                                                                             | Wait Condition                                                   | Expected Result                                                                                                                   | Write Class |
|-----|------------------------------------------------------------------------------------------|------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------|-------------|
| 1   | Navigate to Kode Stream and select the supplied Local workspace and matching branch.     | Workspace and branch are visible in the application shell.       | The workspace loads without an error.                                                                                             | Read-only   |
| 2   | Choose **Workbench** from Workspace navigation.                                          | Canvas loading finishes and **Fit Canvas to view** is available. | Current-branch plan nodes are visible inside workspace and service sections; no note, artifact, or editable-edge controls appear. | Safe write  |
| 3   | Record one plan position, then move the plan to a distinct position.                     | Save status changes from **Saving** to **Saved**.                | The plan moves without changing repository data or the surrounding workspace context.                                             | Safe write  |
| 4   | Drag the workspace section header.                                                       | Save status becomes **Saved**.                                   | The section moves its member placements together without changing repository data.                                                | Safe write  |
| 5   | Reload the page and reopen Workbench for the same workspace and branch.                  | Entity resolution and placement loading finish.                  | Plan and session positions are restored while labels, Git state, and capabilities reflect current state.                          | Read-only   |
| 6   | Trigger the supplied safe plan scan and refresh Canvas.                                  | The new plan node becomes visible.                               | Canvas places it silently without moving saved nodes or showing an action, count, or notification.                                | Safe write  |
| 7   | Cmd/Ctrl-click or box-select two nodes, create a named section, then move and remove it. | Each placement save finishes.                                    | Moving the section moves its member nodes; removing it leaves those node positions unchanged.                                     | Safe write  |

## Section B: Launch One Branch-Safe Session

| #   | Agent Action                                                                                  | Wait Condition                                          | Expected Result                                                                                                                     | Write Class |
|-----|-----------------------------------------------------------------------------------------------|---------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------|-------------|
| 1   | Select the supplied plan node.                                                                | The Plan Workbench is visible.                          | **Info**, **Jira**, and **Quality** tabs plus **View details** are visible; terminal and verification controls follow capabilities. | Read-only   |
| 2   | Choose **Launch terminal** using the default provider configured in Settings.                 | **Launching…** clears and the new session node expands. | One durable session is created, only that session is placed, and its live terminal appears inside the Canvas node.                  | Safe write  |
| 3   | While submission is pending, activate the launch control again if it remains enabled.         | The original request settles.                           | At most one session and one process exist for the submission; no duplicate terminal appears.                                        | Safe write  |
| 4   | Select another plan and the session node, then use the session's top-right disclosure action. | Selection and disclosure updates finish.                | Selection alone does not change the terminal; the action collapses or expands the same process without relaunching it.              | Safe write  |
| 5   | Move the session node and reload the browser page without restarting the backend.             | Canvas and session state reload.                        | Session placement is restored; the record remains visible and reconnect follows existing grant/lease behavior.                      | Safe write  |

## Section C: Reject A Branch Mismatch

Run only when the supplied environment provides a reversible branch-mismatch procedure. Do not switch branches when
the repository is dirty or conflicted unless the supplied setup explicitly makes that safe.

| #   | Agent Action                                                                                                       | Wait Condition                              | Expected Result                                                                                                           | Write Class |
|-----|--------------------------------------------------------------------------------------------------------------------|---------------------------------------------|---------------------------------------------------------------------------------------------------------------------------|-------------|
| 1   | Use the supplied safe setup to make the plan branch differ from the current checkout, then refresh Canvas context. | Branch and capability state finish loading. | The plan shows expected and current branch context; launch is conflicted or the server is ready to reject stale UI state. | Safe write  |
| 2   | Attempt **Launch terminal** only when the controlled fixture intentionally leaves the stale action available.      | **Checkout changed** is visible.            | The launch is rejected with expected/current branch guidance and no new session or process is created.                    | Safe write  |
| 3   | Choose **Refresh Canvas and Git status**.                                                                          | Canvas context finishes refreshing.         | Canvas does not switch branches automatically and does not bypass dirty-tree safeguards.                                  | Read-only   |

## Section D: Git And Verification Freshness

| #   | Agent Action                                                                                | Wait Condition                                          | Expected Result                                                                                                                       | Write Class |
|-----|---------------------------------------------------------------------------------------------|---------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------|-------------|
| 1   | Return to the matching branch and inspect the Workbench checkout context.                   | Git status finishes loading.                            | Current branch plus available Git and verification context are visible.                                                               | Read-only   |
| 2   | Select the supplied plan and run the supplied verification profile.                         | Verification reaches a final state.                     | A completed result shows command outcome separately from freshness and includes a safe abbreviated verified revision.                 | Safe write  |
| 3   | If the result passed and is current, apply the supplied reversible repository mutation.     | Git state refreshes and verification freshness updates. | Git becomes dirty or changes revision, and the previous result becomes **stale** without moving the plan node.                        | Safe write  |
| 4   | Rerun verification after the mutation when permitted.                                       | Verification reaches a final state.                     | The new result is associated with the changed repository fingerprint; the previous result is not presented as current.                | Safe write  |
| 5   | Open **Quality**, select a suggested or supplied automation spec, and inspect the run mode. | Quality settings load.                                  | Suggested specs identify their `automation-test` source; selected specs, environment, and silent/visible mode controls are available. | Safe write  |

## Section E: Removal, Keyboard, And Recovery

| #   | Agent Action                                                                              | Wait Condition                                     | Expected Result                                                                                              | Write Class |
|-----|-------------------------------------------------------------------------------------------|----------------------------------------------------|--------------------------------------------------------------------------------------------------------------|-------------|
| 1   | Use **Search Canvas nodes** to focus the supplied plan without pointer selection.         | The plan node and Workbench receive visible focus. | The accessible state includes kind, title, branch, status, and blocked state where applicable.               | Read-only   |
| 2   | Press an arrow key on the focused plan; use Shift plus an arrow for a one-pixel move.     | Save status is announced as **Saved**.             | The plan moves by 12 pixels, or one pixel with Shift, and other node positions remain unchanged.             | Safe write  |
| 3   | Select the session node and choose **Remove from Canvas** without cancelling the process. | Removal confirmation and save finish.              | The placement disappears and later refreshes do not recreate it; the session record/process remains durable. | Safe write  |
| 4   | Use **Reset layout**, inspect the preview, and cancel it.                                 | Preview closes.                                    | Saved positions remain unchanged.                                                                            | Read-only   |
| 5   | Enable reduced motion and revisit Canvas.                                                 | Canvas finishes rendering.                         | Status remains understandable without smooth viewport transitions or animated connections.                   | Read-only   |

## Evidence

- Capture the initial Canvas, a moved plan, and the workspace section after its member placements move.
- Capture the restored layout after reload.
- Capture one plan-linked session without recording terminal content, prompts, arguments, or credentials.
- Capture branch-mismatch guidance when the controlled setup is available.
- Capture a completed current verification result and the same result after it becomes stale.
- Capture keyboard focus and non-color status cues.

## Cleanup

- Cancel any process created by this playbook through the normal terminal control.
- Revert the isolated repository mutation through the supplied safe procedure.
- Restore the original test branch through existing guarded controls.
- Keep or remove test placements according to the supplied environment policy; never delete the workspace, plan,
  branch, or repository.
- Record skipped conditional steps and missing runtime inputs in `results/latest.md`.
