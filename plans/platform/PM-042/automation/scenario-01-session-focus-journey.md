# UI Playbook 1: Focus On Running Sessions And Read Their Changes

## Goal

> Validate that a user can see which plans have a running agent, list the branch's sessions, read what a session
> changed since launch, and stop a session from the cross-workspace page.

## Preconditions

- Two isolated Local workspaces are registered, each with one indexed plan on its checkout branch.
- A test-safe provider is configured that edits one known file and exits after a short delay.

## Steps

| #  | Action                                                            | Expected                                                                                       |
|----|-------------------------------------------------------------------|------------------------------------------------------------------------------------------------|
| 1  | Open Workbench for workspace A; launch an AI session from plan A. | Session node appears beside plan A in state running.                                           |
| 2  | Toggle Focus.                                                     | Plan A and its session stay bright; other plans dim; label reads "Focus · 1 plan · 1 session". |
| 3  | Select the workspace node; open Sessions.                         | Table lists the session with state running and a changes count.                                |
| 4  | Click Changes on that row.                                        | Session node selected; Changes tab shows the edited file with `M` and a diff.                  |
| 5  | Wait for the provider to exit.                                    | Row shows exited with exit code; Focus dims plan A; label reads "Focus · nothing running".     |
| 6  | Launch a session from plan B in workspace B.                      | Running in workspace B.                                                                        |
| 7  | Open Sessions in the left nav.                                    | Active scope lists the workspace B session; workspace A's exited session is absent.            |
| 8  | Switch scope to All.                                              | Both sessions listed with workspace names.                                                     |
| 9  | Click Stop on the running row; confirm.                           | Row shows cancelled; no process remains for it.                                                |
| 10 | Click Open on that row.                                           | Workbench for workspace B opens with the session node selected and expanded.                   |

## Cleanup

- Reset both workspaces to their starting commits.
