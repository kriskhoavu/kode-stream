# Scenario 0: Behavior-Preserving Refactor

## Goal

Refactor architecture while the user sees the same app, routes, data, files, Git behavior, and UI.

## Starting State

| #   | Area      | Summary                                                                 |
|-----|-----------|-------------------------------------------------------------------------|
| 1   | Backend   | API handlers coordinate registry, scanner, index, file, writer, and Git |
| 2   | Frontend  | Large pages own rendering, state, side effects, and helper logic        |
| 3   | Storage   | Workspace registry and item index are YAML files in the user config dir |
| 4   | Workflows | Workspace scan, item editing, source settings, and Git operations work  |

## Execution Flows

### Flow 0.1: Backend Endpoint Extraction

```text
User action
  -> existing frontend API call
  -> existing HTTP route
  -> thin HTTP handler decodes request
  -> application service coordinates domain, storage, scanner, files, and Git
  -> same response payload
  -> same frontend state update
```

### Flow 0.2: Frontend Page Extraction

```text
User action
  -> same route
  -> page component calls feature hook
  -> hook loads the same API data
  -> extracted components render the same markup classes
  -> same keyboard, mouse, autosave, and confirmation behavior
```

### Flow 0.3: Scanner Extraction

```text
Scan request
  -> same API endpoint
  -> application service calls scanner.Scanner.Scan
  -> scanner facade delegates traversal, settings, metadata, and assembly
  -> same item index replacement
  -> same warnings and item summaries
```

## Visual State

```text
Before PM-003:
  api.go and large pages own many responsibilities directly

After PM-003:
  existing routes and pages remain
  responsibilities move behind stable facades
  tests protect API responses, file writes, scanning, and UI flows
```

## Invariants

| Invariant       | Requirement                                                        |
|-----------------|--------------------------------------------------------------------|
| API routes      | Paths, methods, request bodies, and response bodies stay the same  |
| UI behavior     | Layout, labels, classes, workflows, and controls stay the same     |
| Storage         | Existing YAML files keep the same shape                            |
| Git behavior    | Guards, selected paths, confirmations, and errors stay the same    |
| Source scanning | Structured, configured, and freestyle docs behavior stays the same |
| Stale state     | `/api/state` behavior and cross-tab notice behavior stay the same  |

