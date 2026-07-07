# Scenarios: PM-024 Overview

## Scenario List

| #   | Title                        | Expected Result                                                  |
|-----|------------------------------|------------------------------------------------------------------|
| 1   | Preview a valid file         | All candidates and settings appear without changing the registry |
| 2   | Import selected workspaces   | Selected registrations are saved, scanned, and indexed           |
| 3   | Mixed valid and invalid      | Invalid entries remain visible and cannot be selected            |
| 4   | Existing workspace path      | Duplicate is marked already registered and is not overwritten    |
| 5   | Source changes after preview | Confirmation revalidates and skips changed or invalid candidates |
| 6   | Scan fails after import      | Registration remains and the UI exposes retryable scan failure   |
| 7   | Alternate data directory     | Imports are written to the backend-resolved effective registry   |
| 8   | Cancel review                | No application state changes                                     |

## Starting State

- Plan Manager already has zero or more registered workspaces.
- The user has a predefined current-schema `workspaces.yaml` outside the effective data directory.
- Referenced repositories already exist on the machine.
- The Add Workspace dialog offers Local Path, Remote Git URL, and Existing Workspaces.

## Flow 1: Preview Before Save

```text
Open Add Workspace
  -> choose Existing Workspaces
  -> select workspaces.yaml
  -> backend parses and validates the file
  -> review every candidate and destination path
  -> registry and indexes remain unchanged
```

The review displays name, resolved path, baseline branch, sources, original registration details, Jira metadata without tokens, Knowledge settings, and validation messages. Valid new candidates are selected by default. Invalid and already registered candidates are disabled.

## Flow 2: Confirm Import

```text
Select valid candidates
  -> confirm import
  -> backend rereads and revalidates source
  -> atomically append valid candidates to effective workspaces.yaml
  -> scan and index each imported workspace
  -> refresh workspace list and show per-entry outcomes
```

The imported records use destination-generated IDs and timestamps. Their registration mode is `existing_workspace`; `clonePathManaged` is false.

## Flow 3: Mixed Results

| Condition                         | Preview State         | Import Behavior                                 |
|-----------------------------------|-----------------------|-------------------------------------------------|
| YAML syntax error                 | File-level error      | Nothing selectable or writable                  |
| Unknown or old schema field       | File-level error      | Reject strict parsing; explain current schema   |
| Missing name                      | Invalid candidate     | Keep visible; disable selection                 |
| Path is not a Git root            | Invalid candidate     | Keep visible; disable selection                 |
| Baseline branch is absent         | Invalid candidate     | Keep visible; disable selection                 |
| Source is absent or escapes root  | Invalid candidate     | Keep visible; disable selection                 |
| Same path appears twice in source | Duplicate candidate   | Allow only the first valid occurrence           |
| Path exists in effective registry | Already registered    | Skip without changing destination configuration |
| Jira metadata is invalid          | Invalid candidate     | Keep visible; disable selection                 |
| Scan fails after registry save    | Imported, scan failed | Keep registration and expose Scan retry         |

## Flow 4: Changed State Between Requests

Confirmation sends the source path and selected candidate keys. The backend rereads the file and recalculates keys. Missing keys, changed candidates, new duplicates, and newly invalid files are skipped with explicit outcomes. Unselected entries are ignored. No client-submitted workspace configuration is persisted.

## Flow 5: OS-Specific Configuration

The frontend gets `registryFile` from `/api/system/config-paths` and shows it in the review. The backend registry remains the only writer. The browser never guesses macOS, Linux, Windows, environment override, or bootstrap paths.

## Acceptance Criteria

- Selecting a file and previewing it does not write registry or index data.
- The preview includes all entries, including invalid and duplicate entries.
- Only selected, still-valid entries are registered.
- Registry persistence is one atomic batch operation with mode `0600`.
- Imported repositories are never marked as Plan Manager-managed clones.
- Every new registration receives an automatic scan attempt.
- One scan failure does not block scans for other imported workspaces.
- The final response distinguishes registration success from indexing success.
- The effective registry path shown by the UI matches the backend destination.
- Cancelling at any point before confirmation leaves app state unchanged.
