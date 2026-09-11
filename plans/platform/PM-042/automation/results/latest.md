# UI Automation Result: PM-042

Status: Passed with notes · 2026-09-12 · build `feature/PM-042-session-focus` at the commit after `aafe057`

Run by the orchestrator in the app's own browser pane against a throwaway Local workspace (`journey-pm042`, one indexed
plan) with the real `claude` provider. The lane-era steps do not apply to this shape; the playbook below is the
Session Focus journey.

| #  | Step                                                      | Result | Note                                                                                                                                           |
|----|-----------------------------------------------------------|--------|------------------------------------------------------------------------------------------------------------------------------------------------|
| 1  | Launch an AI session from plan PM-900 (embedded, claude). | Passed | Session node appeared running; Workbench opened on it with Terminal and Changes tabs.                                                          |
| 2  | Toggle Focus.                                             | Passed | Board reduced to the plan and its expanded session with live terminal; label "Focus · 1 plan · 1 session"; sections hidden; layout hint shown. |
| 3  | Session ends; Focus still on.                             | Passed | Label "Focus · nothing running · 2 hidden" with Turn off; Turn off restored positions and viewport.                                            |
| 4  | Changes tab on a session.                                 | Passed | Real data: launch commit, HEAD moved 0, two untracked files Claude's startup wrote (`.serena/`), grouped as untracked.                         |
| 5  | Sessions page, Active scope, nothing running.             | Passed | Empty state with "Go to Workbench".                                                                                                            |
| 6  | Sessions page, All scope.                                 | Passed | Card browser grouped by workspace and plan, 30 records, detail pane with Terminal and Changes.                                                 |
| 7  | Live session on the Sessions page.                        | Passed | Card marked Running; terminal streamed live output in the detail pane.                                                                         |
| 8  | Stop from the Sessions page.                              | Passed | Confirm dialog (Cancel default); after confirm the card read Cancelled and Live-only turned itself off, keeping the selection.                 |
| 9  | Workbench Sessions tab from the board.                    | Fixed  | Unreachable at first because the board never renders the workspace node; a toolbar Sessions button was added and the tab then opened.          |
| 10 | Restart with old app-state files.                         | Fixed  | Two loaders refused files the abandoned lane build had written; both now tolerate the stale field or placement.                                |

Notes: automated keyboard input did not reach the embedded xterm from the browser pane, so the folder-trust prompt was
answered over the session channel; that is a limitation of the test harness, not the app. The throwaway workspace was
removed afterwards.
