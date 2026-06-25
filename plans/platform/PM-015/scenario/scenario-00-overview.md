# Scenarios: PM-015 Overview

## Scenario List

| #   | Title                         | Description                                                                  |
|-----|-------------------------------|------------------------------------------------------------------------------|
| 0   | Current Implementation Review | Capture current backend and frontend behavior before refactoring             |
| 1   | Faster App State Polling      | `/api/state` returns the same contract without hashing every indexed item    |
| 2   | Smoother Rich Preview         | Large Markdown/source files do not block navigation, typing, or tree actions |
| 3   | Feature Controller Extraction | Pages keep the same UI while data loading and actions move into hooks        |
| 4   | Refresh Policy Extension      | New workspace mutations can choose refresh behavior without duplicating code |

---

# Scenario 0: Current Implementation Review

## Starting State

| #   | Area     | Summary                                                                    |
|-----|----------|----------------------------------------------------------------------------|
| 1   | Backend  | API, workspace, scanner, item, file, search, and index tests pass          |
| 2   | Frontend | Kanban, item workspace, workspace explorer, and source settings tests pass |
| 3   | Storage  | Workspaces and item index remain YAML-backed in the user config folder     |
| 4   | Routing  | Browser routes and API endpoints remain unchanged                          |

## Visual State Before

```text
React pages with broad orchestration
  -> API resource handlers in one package/file
  -> application services
  -> scanner, item index, workspace files, Git
  -> YAML state and workspace files
```

## Execution Flows

### Flow 0.1: Review And Baseline

```text
Developer runs focused tests and builds
  -> record large files and hot paths
  -> document current performance risks
  -> add characterization tests where gaps exist
  -> implement behavior-preserving phases
```

## Visual State After

```text
React feature controllers and renderer adapters
  -> stable API facade
  -> resource-specific handlers
  -> scan pipeline and refresh policy
  -> item index state snapshot
```

## Edge Cases

| Case                         | Expected Result                                              |
|------------------------------|--------------------------------------------------------------|
| Empty workspace registry     | State and UI still return empty collections quickly          |
| Large Markdown file          | Rich preview pauses or defers; source mode remains available |
| Snapshot branch item         | PM-013 materialization rules stay unchanged                  |
| Source setting save/reset    | PM-014 proposal and preview behavior stays unchanged         |
| Mutation outside source path | Existing safety and audit behavior stays unchanged           |
