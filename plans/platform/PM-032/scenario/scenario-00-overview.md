# Scenarios: PM-032 Overview

## Scenario List

| #   | Title                          | Description                                                                    |
|-----|--------------------------------|--------------------------------------------------------------------------------|
| 0   | Local mode                     | Trusted user-machine app continues to support local workspace operations.      |
| 1   | Cloud user connects agent      | Authenticated user connects a Cloud Agent from the hosted UI.                  |
| 2   | Cloud user registers workspace | User selects a local repository through Cloud Agent and Cloud stores metadata. |
| 3   | Cloud routes terminal command  | AI Session Terminal runs on the user machine through Cloud Agent.              |
| 4   | Cloud role gates actions       | UI and API block actions outside the user's role and agent capability.         |
| 5   | Agent unavailable              | Cloud UI shows workspace offline and disables command-capable controls.        |
| 6   | Cloud deployment smoke         | Operator deploys hosted Cloud and verifies agent-backed readiness.             |

## Scenario 0: Local Mode

### Goal

Local users run the app directly on their machine.

### Starting State

| #   | Title           | Summary                                                              |
|-----|-----------------|----------------------------------------------------------------------|
| 1   | Runtime         | `kode-stream serve` starts in local mode when no mode is configured. |
| 2   | Network         | Server binds to loopback by default.                                 |
| 3   | Workspace input | Local path, remote clone, and workspace import remain visible.       |
| 4   | Capabilities    | User is treated as trusted local admin.                              |

### Execution Flow

User starts local server -> backend resolves local mode -> local paths and native system helpers are enabled -> frontend
receives local capabilities -> workspace, Git, terminal, AI, runtime, and verification UI remain available.

### Expected Result

No login prompt appears, no Cloud agent connection is required, and local workflows run on the user machine.

## Scenario 1: Cloud User Connects Agent

### Goal

Authenticated Cloud user connects a local agent so Cloud can route workspace operations to the user machine.

### Starting State

| #   | Title         | Summary                                                                             |
|-----|---------------|-------------------------------------------------------------------------------------|
| 1   | Runtime       | Server runs with `KODE_STREAM_MODE=cloud`.                                          |
| 2   | Auth          | User has authenticated through OAuth2Proxy and Keycloak.                            |
| 3   | Agent install | User has installed `kode-stream agent`; macOS Homebrew is the first install target. |

### Execution Flow

User selects connect local workspace -> Cloud creates short-lived connect token -> browser opens
`kodestream://connect` -> installed agent wakes -> agent opens outbound HTTPS WebSocket to Cloud -> Cloud marks the
agent connected.

### Expected Result

Cloud knows the user has a connected agent and does not receive SSH keys, Git tokens, or local credential helper output.
No port forwarding or inbound user-machine firewall rule is required.

## Scenario 2: Cloud User Registers Workspace

### Goal

User registers a local repository through Cloud Agent while Cloud stores only metadata.

### Starting State

| #   | Title     | Summary                                                       |
|-----|-----------|---------------------------------------------------------------|
| 1   | Session   | User is logged into Cloud.                                    |
| 2   | Agent     | User has a connected Cloud Agent.                             |
| 3   | Local Git | User machine has a Git repository and local credential setup. |

### Execution Flow

User starts workspace registration -> Cloud sends registration command to agent -> agent presents native folder
picker -> user chooses local repository -> agent validates Git root and source paths -> agent scans locally -> agent
publishes workspace metadata and optional summaries -> Cloud stores metadata and refreshes the board.

### Expected Result

Cloud stores workspace identity, agent ownership, redacted path label, remote URL when available, scan summary, and
board metadata. Cloud does not store a repository clone or executable workspace path.

## Scenario 3: Cloud Routes Terminal Command

### Goal

AI Session Terminal runs on the user's machine from the hosted Cloud UI.

### Starting State

| #   | Title     | Summary                                                  |
|-----|-----------|----------------------------------------------------------|
| 1   | Workspace | Cloud workspace is backed by the user's connected agent. |
| 2   | Role      | User role permits terminal or AI CLI access.             |
| 3   | Agent     | Agent is connected and permits command execution.        |

### Execution Flow

User opens AI Session Terminal -> frontend checks capability map -> Cloud validates role and workspace ownership ->
Cloud sends command envelope to owner agent -> agent starts terminal process in the local workspace -> streams output
through Cloud -> frontend renders the terminal session.

### Expected Result

Terminal process runs on the user machine. Cloud transports authenticated control and stream messages only.

## Scenario 4: Cloud Role Gates Actions

### Goal

Cloud users only see and execute actions allowed by their role and agent capabilities.

### Starting State

| #   | Role   | Expected Actions                                                       |
|-----|--------|------------------------------------------------------------------------|
| 1   | Viewer | Read-only board, explorer, item, knowledge, status, and audit views.   |
| 2   | Editor | Workspace registration, file edits, scans, Git, terminal, AI, runtime. |
| 3   | Admin  | User settings, deployment settings, role policy, integration panels.   |

### Execution Flow

Frontend loads `/api/state` -> state includes mode, user, role, capabilities, and agent availability -> controls render
enabled, disabled, or hidden states -> API middleware enforces the same policy on matching routes -> command routes
require connected owner agent.

### Expected Result

Denied requests return stable authorization or unavailable errors. Cloud does not execute blocked commands on the hosted
VM.

## Scenario 5: Agent Unavailable

### Goal

Cloud workspaces fail clearly when the user machine is offline.

### Starting State

| #   | Title          | Summary                                                   |
|-----|----------------|-----------------------------------------------------------|
| 1   | Workspace      | Cloud knows a workspace exists for the user.              |
| 2   | Agent status   | No active owner agent connection exists.                  |
| 3   | Published data | Optional published summaries may exist from a prior scan. |

### Execution Flow

User opens workspace -> Cloud checks agent registry -> no live owner agent is found -> UI shows offline state ->
read-only published summaries may render when available -> live file, Git, terminal, AI, runtime, and verification
actions are disabled.

### Expected Result

No Cloud-side fallback accesses local paths, asks for Git credentials, clones the repository, or starts a hosted
terminal.

## Scenario 6: Cloud Deployment Smoke

### Goal

Operator can deploy Cloud mode and verify agent-backed readiness.

### Starting State

| #   | Title  | Summary                                                                           |
|-----|--------|-----------------------------------------------------------------------------------|
| 1   | Image  | Container image includes built frontend and compiled Go binary.                   |
| 2   | Volume | Metadata data volume is mounted.                                                  |
| 3   | Proxy  | OAuth2Proxy is exposed publicly and Kode Stream app port is private.              |
| 4   | OIDC   | Keycloak issuer, client ID, client secret, redirect URL, and cookie secret exist. |

### Execution Flow

Operator starts Kode Stream and OAuth2Proxy -> healthcheck passes through the proxy -> browser reaches OAuth2Proxy login
-> OAuth2Proxy redirects to Keycloak -> OAuth2Proxy forwards identity headers to Kode Stream -> `/api/state` reports
Cloud mode -> user connects Cloud Agent -> user registers local workspace through agent -> board loads published
metadata.

### Expected Result

Deployment is usable without hosted repository clones and can be backed up by preserving the configured metadata volume.
