# UI Playbook 1: Review And Activate A Branch

## Goal

> Review committed plans without changing the checkout, explicitly import one plan, then activate the reviewed branch
> through guarded checkout and observe all operational pages synchronize.

## Preconditions

- Feature build is deployed to a non-production Local environment.
- A disposable registered workspace is checked out on branch A.
- Branch A has a known uncommitted marker and branch B has a unique committed structured plan.
- Import and checkout writes are approved for the disposable workspace.

## Runtime Inputs

| Input                | Required Value                           |
|----------------------|------------------------------------------|
| Base URL             | Supplied at run time                     |
| Authentication       | Supplied at run time                     |
| Workspace            | Disposable registered Local workspace    |
| Checkout / review    | Branch A / branch B                      |
| Unique reviewed plan | Structured plan present only on branch B |

## Steps

| #   | Agent Action                                                                                                 | Wait Condition                                                      | Expected Result                                                                                  | Write Class |
|-----|--------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------|--------------------------------------------------------------------------------------------------|-------------|
| 1   | Open Workstream for the workspace.                                                                           | Board loading finishes.                                             | Branch A is labeled as the checkout and its working-tree plans are visible.                      | Read-only   |
| 2   | Choose **Review branch**, then branch B.                                                                     | **Branch Review** and **Read-only committed snapshot** are visible. | Branch B and its pinned commit are shown while checkout remains branch A.                        | Read-only   |
| 3   | Open the unique reviewed plan and one committed file.                                                        | File content finishes loading.                                      | Snapshot content is visible with no editing, terminal, verification, or status-mutation control. | Read-only   |
| 4   | Open Canvas and Knowledge in turn, then return to Review.                                                    | Each page finishes loading.                                         | Operational pages still show branch A content; review remains route-local.                       | Read-only   |
| 5   | Choose **Import plan into checkout** and confirm the preview.                                                | Import completes and the operational item opens.                    | The plan is copied into branch A without changing checkout or the uncommitted marker.            | Safe write  |
| 6   | Return to Review and choose **Switch workspace to this branch**. Confirm dirty-tree switching when prompted. | Checkout refresh completes.                                         | Git reports branch B and Workstream reloads branch B.                                            | Safe write  |
| 7   | Open Canvas, Knowledge, and the imported item route.                                                         | Each page finishes loading.                                         | All operational pages use branch B; no page silently shows branch A or a selected snapshot.      | Read-only   |

## Evidence

- Capture Workstream checkout state, the Branch Review banner, import preview/result, and synchronized post-checkout
  Workstream and Canvas states.
- Do not capture terminal content, credentials, or unrelated repository files.

## Cleanup

- Stop any process started outside this playbook.
- Remove the disposable registered workspace through the normal test-environment cleanup procedure.
