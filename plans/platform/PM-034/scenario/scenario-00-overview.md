# Scenarios: PM-034 Overview

## Scenario List

| #   | Title                      | Description                                                         |
|-----|----------------------------|---------------------------------------------------------------------|
| 0   | Current localhost-only app | User runs Kode Stream and opens the UI from `http://localhost`.     |
| 1   | Extension opens local app  | User loads the unpacked extension and reaches the bundled React UI. |
| 2   | Files and Git showcase     | User edits workspace files and runs branch-oriented Git actions.    |
| 3   | Local server unavailable   | Extension explains that `kode-stream serve` must be running.        |

---

# Scenario 0: Current Localhost-Only App

## Starting State

| #   | Title           | Summary                                                                           |
|-----|-----------------|-----------------------------------------------------------------------------------|
| 1   | Local server    | `kode-stream serve -port 4317` serves embedded React assets and `/api/*` routes.  |
| 2   | Browser origin  | React runs from `http://127.0.0.1:4317` or `http://localhost:4317`.               |
| 3   | API calls       | Frontend calls relative `/api/*` paths.                                           |
| 4   | Privileged work | The Go server handles filesystem, Git, terminal, AI CLI, and verification access. |

## Visual State

```text
Chrome tab on localhost
    -> Kode Stream React UI
    -> relative /api calls
    -> Go local server
    -> registered workspace files and Git
```

---

# Scenario 1: Extension Opens Local App

## Goal

Let a user load Kode Stream from an unpacked Chrome extension while the local server supplies API behavior.

## Starting State

| #   | Title              | Summary                                                                 |
|-----|--------------------|-------------------------------------------------------------------------|
| 1   | Extension artifact | `dist/chrome-extension` contains MV3 manifest and built React assets.   |
| 2   | Local server       | User has started `kode-stream serve -port 4317`.                        |
| 3   | Registered repo    | At least one local Git workspace is already registered or can be added. |

## Execution Flow

```text
User loads unpacked extension
    -> Chrome opens the extension page
    -> React detects extension surface
    -> API origin adapter calls http://127.0.0.1:4317/api/health
    -> UI loads state, workspaces, and files from the local API
```

## Expected Result

The extension-hosted UI shows the same Kode Stream workspace experience as the localhost UI for supported Files + Git
workflows.

---

# Scenario 2: Files And Git Showcase

## Goal

Prove that existing cloned folders still work when the UI is bundled as a Chrome extension.

## Execution Flow

```text
User selects a workspace
    -> extension UI calls local API for workspace tree
    -> user opens a Markdown file
    -> extension UI calls local API for content and preview data
    -> user edits and saves the file
    -> Go server writes through existing guarded file writer
    -> user checks Git status and branch list
    -> Go server shells out to allowed Git adapter operations
    -> user switches to a clean branch
    -> UI refreshes board, file tree, and Git state
```

## Acceptance Notes

| Capability    | Expected Behavior                                                                     |
|---------------|---------------------------------------------------------------------------------------|
| Browse files  | Workspace tree and content search work through the existing local API.                |
| Edit file     | Markdown save uses current write guards and stale-content handling.                   |
| Git status    | Dirty state reflects the saved local file.                                            |
| Branch list   | Branch selector loads current and available branches from the existing Git endpoint.  |
| Branch switch | Clean branch switch succeeds; dirty-tree guard keeps the existing confirmation rules. |

---

# Scenario 3: Local Server Unavailable

## Goal

Avoid a blank extension page when the local API is not running.

## Execution Flow

```text
User opens extension while kode-stream is stopped
    -> extension UI calls /api/health on configured local API origin
    -> request fails
    -> UI shows local server unavailable state
    -> user starts kode-stream serve -port 4317
    -> user retries
    -> extension loads normal app state
```

## Edge Cases

| Case             | Expected Behavior                                                              |
|------------------|--------------------------------------------------------------------------------|
| Wrong port       | User can set `localStorage.kodeStreamApiOrigin` and reload the extension page. |
| API unhealthy    | UI reports the local server problem instead of hiding workspace actions.       |
| Unsupported flow | Embedded terminal/AI streaming controls are unavailable in extension mode.     |
