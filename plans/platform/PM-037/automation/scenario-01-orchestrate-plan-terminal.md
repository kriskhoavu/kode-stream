# UI Playbook 1: Operate The Focused Terminal Canvas

## Goal

> Validate that a user can arrange and restore workspace, plan, and session nodes; launch one terminal on the correct
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

| #   | Agent Action                                                                                | Wait Condition                                             | Expected Result                                                                                                           | Write Class |
|-----|---------------------------------------------------------------------------------------------|------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------|-------------|
| 1   | Navigate to Kode Stream and select the supplied Local workspace and matching branch.        | Workspace and branch are visible in the application shell. | The workspace loads without an error.                                                                                     | Read-only   |
| 2   | Choose **Canvas** from Workspace navigation.                                                | Canvas loading finishes and **Fit content** is available.  | One workspace node and current branch plan nodes are visible; no group, note, artifact, or editable edge controls appear. | Safe write  |
| 3   | Record the positions of one workspace node and one plan node, then move the workspace node. | Save status changes from **Saving** to **Saved**.          | Only the workspace position changes; the plan position is unchanged.                                                      | Safe write  |
| 4   | Move the supplied plan node to a distinct position.                                         | Save status becomes **Saved**.                             | The plan moves without changing the workspace position or repository data.                                                | Safe write  |
| 5   | Reload the page and reopen Canvas for the same workspace and branch.                        | Entity resolution and placement loading finish.            | Workspace and plan positions are restored while labels, Git state, and capabilities reflect current state.                | Read-only   |
| 6   | If **Unplaced work** is visible, choose **Place new items**.                                | Placement save finishes.                                   | New items receive positions without moving previously saved nodes.                                                        | Safe write  |

## Section B: Launch One Branch-Safe Session

| #   | Agent Action                                                                            | Wait Condition                                                | Expected Result                                                                                                                                 | Write Class |
|-----|-----------------------------------------------------------------------------------------|---------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------|-------------|
| 1   | Select the supplied plan node.                                                          | The Plan Workbench is visible.                                | The plan identifier, matching branch, terminal action, verification action, and **Open full view** are visible as capabilities allow.           | Read-only   |
| 2   | Choose **Open session**, select the supplied provider, and submit one test-safe launch. | Session becomes running or a contextual launch error appears. | One durable session is created. A successful launch exposes one live terminal and one session node or unplaced session item linked to the plan. | Safe write  |
| 3   | While submission is pending, activate the launch control again if it remains enabled.   | The original request settles.                                 | At most one session and one process exist for the submission; no duplicate terminal appears.                                                    | Safe write  |
| 4   | Select the workspace node, then return to the session node.                             | Each Workbench state finishes loading.                        | The same process remains active and returning to the session does not relaunch it.                                                              | Read-only   |
| 5   | Move the session node and reload the browser page without restarting the backend.       | Canvas and session state reload.                              | Session placement is restored; the record remains visible and reconnect follows existing grant/lease behavior.                                  | Safe write  |

## Section C: Reject A Branch Mismatch

Run only when the supplied environment provides a reversible branch-mismatch procedure. Do not switch branches when
the repository is dirty or conflicted unless the supplied setup explicitly makes that safe.

| #   | Agent Action                                                                                                       | Wait Condition                              | Expected Result                                                                                                           | Write Class |
|-----|--------------------------------------------------------------------------------------------------------------------|---------------------------------------------|---------------------------------------------------------------------------------------------------------------------------|-------------|
| 1   | Use the supplied safe setup to make the plan branch differ from the current checkout, then refresh Canvas context. | Branch and capability state finish loading. | The plan shows expected and current branch context; launch is conflicted or the server is ready to reject stale UI state. | Safe write  |
| 2   | Attempt **Open session** only when the controlled fixture intentionally leaves the stale action available.         | A branch-mismatch result is visible.        | The launch is rejected with expected/current branch guidance and no new session or process is created.                    | Safe write  |
| 3   | Open the offered branch recovery control.                                                                          | Existing branch controls load.              | Canvas does not switch branches automatically and does not bypass dirty-tree safeguards.                                  | Read-only   |

## Section D: Git And Verification Freshness

| #   | Agent Action                                                                            | Wait Condition                                          | Expected Result                                                                                                        | Write Class |
|-----|-----------------------------------------------------------------------------------------|---------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------|-------------|
| 1   | Return to the matching branch and select the workspace node.                            | Git status finishes loading.                            | Current branch plus clean, dirty, or conflicted status and changed-file count are visible.                             | Read-only   |
| 2   | Select the supplied plan and run the supplied verification profile.                     | Verification reaches a final state.                     | A completed result shows command outcome separately from freshness and includes a safe abbreviated verified revision.  | Safe write  |
| 3   | If the result passed and is current, apply the supplied reversible repository mutation. | Git state refreshes and verification freshness updates. | Git becomes dirty or changes revision, and the previous result becomes **stale** without moving the plan node.         | Safe write  |
| 4   | Rerun verification after the mutation when permitted.                                   | Verification reaches a final state.                     | The new result is associated with the changed repository fingerprint; the previous result is not presented as current. | Safe write  |

## Section E: Removal, Keyboard, And Recovery

| #   | Agent Action                                                                              | Wait Condition                                     | Expected Result                                                                                                      | Write Class |
|-----|-------------------------------------------------------------------------------------------|----------------------------------------------------|----------------------------------------------------------------------------------------------------------------------|-------------|
| 1   | Use node search to focus the supplied plan without pointer selection.                     | The plan node and Workbench receive visible focus. | The accessible state includes kind, title, branch, status, and blocked state where applicable.                       | Read-only   |
| 2   | Use the documented keyboard move control on the plan.                                     | Save status is announced as **Saved**.             | The plan moves by a bounded increment and other node positions remain unchanged.                                     | Safe write  |
| 3   | Select the session node and choose **Remove from Canvas** without cancelling the process. | Removal confirmation and save finish.              | The placement disappears, while the session record/process remains discoverable through active or unplaced sessions. | Safe write  |
| 4   | Use **Reset layout**, inspect the preview, and cancel it.                                 | Preview closes.                                    | Saved positions remain unchanged.                                                                                    | Read-only   |
| 5   | Enable reduced motion and revisit Canvas.                                                 | Canvas finishes rendering.                         | Status remains understandable without smooth viewport transitions or animated connections.                           | Read-only   |

## Evidence

- Capture the initial Canvas and the independently moved workspace and plan positions.
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
