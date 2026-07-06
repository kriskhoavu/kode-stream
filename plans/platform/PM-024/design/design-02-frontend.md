# Frontend Design: Existing Workspaces Review

## Overview

The current Add Workspace dialog gains a third mode, **Existing Workspaces**. This mode has file selection, review, and result states. It does not reuse the Local Path/Jira steps because imported candidates already contain their setup and must be reviewed as a group.

## User Flow

```text
Add Workspace
  -> Existing Workspaces
  -> choose or enter workspaces.yaml
  -> Preview
  -> review destination and every candidate
  -> select valid candidates
  -> Import selected
  -> inspect indexed, scan-failed, and skipped outcomes
  -> Done refreshes application workspace state
```

## Components

| Component                     | Responsibility                                                         |
|-------------------------------|------------------------------------------------------------------------|
| `WorkspaceRegistrationDialog` | Owns registration-mode routing and close confirmation                  |
| `ExistingWorkspacesFileStep`  | File picker, path input, supported-format help, and preview request    |
| `WorkspaceImportReview`       | Destination, summary, selection controls, and candidate list           |
| `WorkspaceImportCandidateRow` | Configuration disclosure, validation status, and details               |
| `WorkspaceImportResults`      | Registration/index status and scan retry guidance                      |
| Shared API types              | Preview, candidate, issue, import request, and import result contracts |

## State Model

| State        | Meaning                                  | Primary Action          |
|--------------|------------------------------------------|-------------------------|
| `selecting`  | No preview loaded                        | Preview file            |
| `previewing` | Preview request in progress              | None                    |
| `reviewing`  | Candidates loaded and registry unchanged | Import selected         |
| `importing`  | Confirmed import and scans in progress   | None                    |
| `complete`   | Per-candidate outcomes available         | Done                    |
| `error`      | File-level preview or import failure     | Retry or choose another |

Preview data is server-owned. The client stores only the source path, preview response, selected candidate-key set, and result response. Changing the path clears stale preview and selection state.

## Review Presentation

The review header shows:

- Canonical source path.
- Effective destination `workspaces.yaml` path.
- Counts for valid, invalid, duplicate, and already registered entries.
- A statement that the source file will not be changed.

Each candidate row shows name, path, baseline branch, sources, registration origin, Jira connection metadata, Knowledge settings, and validation issues. Configuration details are expanded by default for invalid entries and collapsible for valid entries.

Valid new candidates are selected by default. Select All affects only selectable candidates. Invalid, duplicate, and already registered candidates remain visible but disabled. Import is disabled when nothing is selected.

## Result Presentation

| Result        | Visual Treatment | Follow-Up                                         |
|---------------|------------------|---------------------------------------------------|
| Indexed       | Success          | Workspace is immediately available                |
| Scan failed   | Warning          | Keep workspace and link to its normal Scan action |
| Skipped       | Neutral          | Explain changed source or new duplicate           |
| Import failed | Error            | Keep dialog open and retain safe retry context    |

Closing after any successful registration refreshes workspaces and application state. The selected active workspace does not change automatically when multiple entries are imported.

## Accessibility and Interaction

- Mode choices remain a keyboard-accessible radio group.
- Candidate selection uses labeled checkboxes and a fieldset.
- Status is expressed by text and icon, never color alone.
- Async state uses `aria-busy`; file and import errors use an alert region.
- Focus moves to the review heading after preview and result heading after import.
- Escape does not discard an in-progress request; closing a loaded review requires confirmation.
- Long paths wrap and provide a copy action without horizontal page scrolling.

## Responsive Layout

Desktop uses a summary header and dense candidate rows. Narrow screens stack configuration fields and keep the confirmation actions in normal document flow. The dialog must not require a fixed minimum width and the candidate list must remain independently readable at 320 CSS pixels.

## Tests

| Area          | Coverage                                                                 |
|---------------|--------------------------------------------------------------------------|
| Helpers       | Default selection, status counts, disabled candidates, result summaries  |
| API client    | Preview and import payloads, normalization, error propagation            |
| Dialog        | Third mode, picker cancellation, path changes, close behavior            |
| Review        | Full setup display, selection, inaccessible candidates, destination path |
| Results       | Mixed indexed/failed/skipped outcomes and app refresh                    |
| Accessibility | Radio, checkbox, focus, busy, alert, and keyboard behavior               |
