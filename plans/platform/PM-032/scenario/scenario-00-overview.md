# Scenarios: PM-032 Overview

## Scenario List

| #   | Title                          | Description                                                                  |
|-----|--------------------------------|------------------------------------------------------------------------------|
| 0   | Local-only baseline            | Current trusted local app continues to work without login or remote hosting. |
| 1   | Cloud user registers Git URL   | Authenticated user clones a remote repository into Cloud-managed storage.    |
| 2   | Cloud role gates risky actions | UI and API block terminal/runtime actions unless the user has admin role.    |
| 3   | Agent user connects local repo | User connects installed agent and registers an existing local repository.    |
| 4   | Agent unavailable              | Cloud UI shows agent workspace as offline without exposing stale controls.   |
| 5   | Cloud deployment smoke         | Operator runs the VM/container deployment and verifies Cloud readiness.      |

## Scenario 0: Local-Only Baseline

### Goal

Existing local users keep the current Kode Stream behavior.

### Starting State

| #   | Title           | Summary                                                                 |
|-----|-----------------|-------------------------------------------------------------------------|
| 1   | Runtime         | `kode-stream serve` starts in local mode when no mode is configured.    |
| 2   | Network         | Server binds to loopback by default.                                    |
| 3   | Workspace input | Local path, remote clone, and existing workspace import remain visible. |
| 4   | Capabilities    | User is treated as trusted local admin.                                 |

### Execution Flow

User starts local server -> backend resolves local mode -> local paths and native system helpers are enabled -> frontend receives local capabilities -> current workspace, Git, terminal, AI, runtime, and verification UI remains available.

### Expected Result

No login prompt appears, no Cloud storage layout is used, and existing local tests continue to pass.

## Scenario 1: Cloud User Registers Git URL

### Goal

An authenticated Cloud user registers a workspace without uploading local paths or SSH keys to Kode Stream.

### Starting State

| #   | Title      | Summary                                                                          |
|-----|------------|----------------------------------------------------------------------------------|
| 1   | Runtime    | Server runs with `KODE_STREAM_MODE=cloud`.                                       |
| 2   | Auth       | User has a valid OIDC session and at least editor role.                          |
| 3   | Storage    | Cloud data root exists with per-user state and clone root directories available. |
| 4   | Git access | Cloud Git credential provider is configured for the selected Git provider.       |

### Execution Flow

User opens workspace registration -> UI shows Git URL registration only -> user submits name, Git URL, baseline branch, and sources -> API checks role and Cloud mode rules -> clone path is allocated under the user data root -> repository is cloned -> scanner indexes configured sources -> UI refreshes board and workspace list.

### Expected Result

The workspace path is server-managed, the registry belongs to the authenticated user scope, and no local filesystem path fields are accepted by Cloud routes.

## Scenario 2: Cloud Role Gates Risky Actions

### Goal

Cloud users only see and execute actions allowed by their role and runtime capabilities.

### Starting State

| #   | Role   | Expected Risky Actions                                               |
|-----|--------|----------------------------------------------------------------------|
| 1   | Viewer | Read-only board, explorer, item, knowledge, status, and audit views. |
| 2   | Editor | File edits, metadata updates, scans, and guarded Git operations.     |
| 3   | Admin  | Workspace management, integration settings, terminal, AI, runtime.   |

### Execution Flow

Frontend loads `/api/state` -> state includes mode, user, role, and capabilities -> controls render enabled, disabled, or hidden states -> API middleware enforces the same policy on every matching route -> denied requests return stable authorization errors.

### Expected Result

Terminal, AI CLI, runtime command execution, and verification start cannot be triggered by non-admin users in Cloud mode.

## Scenario 3: Agent User Connects Local Repo

### Goal

User registers an existing repository on their machine without Cloud storing Git credentials.

### Starting State

| #   | Title         | Summary                                                                             |
|-----|---------------|-------------------------------------------------------------------------------------|
| 1   | Cloud session | User is logged into the hosted app.                                                 |
| 2   | Agent install | User has installed `kode-stream agent`; macOS Homebrew is the first install target. |
| 3   | Local Git     | User machine already has working Git remotes, SSH agent, and credential helpers.    |

### Execution Flow

User selects connect local workspace -> browser opens `kodestream://connect` -> installed agent wakes and connects outbound to Cloud -> user selects a local repository with native picker -> agent validates and scans the repo -> Cloud shows the Agent workspace and routes file, Git, terminal, AI, runtime, and verification actions to the live agent.

### Expected Result

Cloud stores no SSH key, Git token, credential-helper output, terminal transcript, or full repository copy for the Agent workspace.

## Scenario 4: Agent Unavailable

### Goal

Agent workspaces fail clearly when the user machine is offline.

### Starting State

| #   | Title          | Summary                                                   |
|-----|----------------|-----------------------------------------------------------|
| 1   | Workspace      | Cloud knows an Agent workspace exists for the user.       |
| 2   | Agent status   | No active agent connection exists.                        |
| 3   | Published data | Optional published summaries may exist from a prior scan. |

### Execution Flow

User opens Agent workspace -> Cloud checks connection registry -> no live agent is found -> UI shows offline state -> read-only published summaries may render when available -> live file, Git, terminal, AI, runtime, and verification actions are disabled.

### Expected Result

No Cloud-side fallback tries to access local paths or prompts for Git credentials.

## Scenario 5: Cloud Deployment Smoke

### Goal

Operator can deploy Cloud mode on a VM/container and verify it is ready.

### Starting State

| #   | Title   | Summary                                                                  |
|-----|---------|--------------------------------------------------------------------------|
| 1   | Image   | Container image includes built frontend and compiled Go binary.          |
| 2   | Volumes | Data and clone root volumes are mounted.                                 |
| 3   | Proxy   | TLS and public routing are handled by the reverse proxy.                 |
| 4   | OIDC    | Issuer, client ID, client secret, redirect URL, and cookie secret exist. |

### Execution Flow

Operator starts container -> healthcheck passes -> browser reaches login -> user authenticates -> `/api/state` reports Cloud mode -> user registers Git URL workspace -> scan completes -> board loads indexed items.

### Expected Result

Deployment is usable without local path features and can be backed up by preserving the configured data volume.
