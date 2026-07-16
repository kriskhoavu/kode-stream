# Frontend Design: Cloud Mode With Local Agent Execution

## Overview

The frontend becomes mode-aware through `/api/state`. Local mode keeps the trusted user-machine app shell. Cloud mode
adds signed-in user context, role-aware controls, Cloud Agent connection, workspace status, and unavailable states when
the owner agent is offline. The UI presents Cloud as a hosted control panel for a user-machine agent.

## App State Model

| Field                   | Type   | Purpose                                              |
|-------------------------|--------|------------------------------------------------------|
| `mode`                  | string | `local` or `cloud`.                                  |
| `user`                  | object | Signed-in Cloud user, or trusted local user context. |
| `role`                  | string | Effective role for frontend gating.                  |
| `capabilities`          | map    | Feature availability and reason strings.             |
| `workspace.location`    | string | `local_path` or `cloud_agent`.                       |
| `workspace.agentStatus` | string | Agent availability for Cloud workspaces.             |
| `workspace.agentId`     | string | Agent that owns workspace command execution.         |

`useAppState` remains the app-level owner of route, theme, active workspace, and refresh keys. PM-032 extends it with
runtime context and agent state.

## Capability UX

| Feature Area         | Local Mode                       | Cloud Mode                                        |
|----------------------|----------------------------------|---------------------------------------------------|
| Workspace register   | Local path, remote clone, import | Connect Cloud Agent, then choose local repository |
| System picker/reveal | Native behavior                  | Routed through connected Cloud Agent              |
| Git status/diff      | Native behavior                  | Routed through owner Cloud Agent                  |
| Git commit/push      | Native behavior                  | Editor/admin through owner Cloud Agent            |
| Terminal and AI CLI  | Native behavior                  | Routed through owner Cloud Agent                  |
| Runtime verification | Native behavior                  | Routed through owner Cloud Agent                  |
| Settings             | Local app settings               | Account, role, deployment, and agent settings     |

Unavailable controls explain the reason in place. Cloud mode must show Cloud Agent registration as the workspace
registration path.

## Cloud Workspace Registration Flow

| Step | UI State                | Behavior                                                                 |
|------|-------------------------|--------------------------------------------------------------------------|
| 1    | Connect local workspace | User starts Cloud Agent connection from Cloud UI.                        |
| 2    | Install guidance        | UI shows native agent installation path when no agent is available.      |
| 3    | Wake agent              | Browser opens `kodestream://connect` with a short-lived token.           |
| 4    | Agent status            | UI shows connecting while the agent opens outbound WebSocket to Cloud.   |
| 5    | Local repo registration | Agent presents native folder picker and validates the selected Git root. |
| 6    | Workspace use           | UI labels the workspace as Cloud Agent backed and routes actions there.  |

No Chrome extension is part of the flow. Homebrew macOS install is the first supported path; package-manager or installer paths for Windows and Linux are planned follow-ups.

The UI should not ask the user to configure port forwarding. Network recovery copy should mention Cloud reachability,
WebSocket proxy policy, optional VPN policy, and agent reconnect.

## User And Role UI

| Surface         | Change                                                                         |
|-----------------|--------------------------------------------------------------------------------|
| App shell       | Show signed-in user, role, mode label, agent status, and logout in Cloud mode. |
| Workspace list  | Show workspace agent status and owner device label.                            |
| Workspaces page | Show Cloud Agent connection and local repository registration.                 |
| Item workspace  | Gate Git, terminal, runtime, verification, and file actions by agent status.   |
| Settings page   | Separate Cloud account/deployment info from local agent settings.              |
| Error boundary  | Preserve authorization and agent recovery hints.                               |

Mode labels should be short and functional.

## Component Impact

| Component Or Area             | Required Change                                                         |
|-------------------------------|-------------------------------------------------------------------------|
| `web/src/app/useAppState.ts`  | Fetch and expose runtime mode, current user, role, capabilities, agent. |
| `web/src/lib/types.ts`        | Add runtime, user, role, capability, workspace, and agent types.        |
| `web/src/shared/api/index.ts` | Add auth/logout and agent discovery/connect calls.                      |
| `WorkspacesPage`              | Render Cloud Agent connection and local repo registration flow.         |
| `ItemWorkspacePage`           | Gate Git, terminal, runtime, verification, and file actions.            |
| `AISessionLaunchControl`      | Render connected-agent, offline-agent, and unavailable states.          |
| `EmbeddedTerminalDock`        | Stream terminal sessions from the owner Cloud Agent.                    |
| `ReliabilityPanels`           | Show Cloud Agent health and capability warnings.                        |

## State And Routing

- Browser routes remain the same for board, explorer, workspaces, and items.
- Cloud auth routes are server routes outside SPA handling.
- Frontend fetch handling redirects or shows login-required state on 401 in Cloud mode.
- A 403 shows role/access copy without logging the user out.
- A 503 capability error shows agent unavailable state and recovery hint.

## Empty And Failure States

| State                     | User-Facing Behavior                                                |
|---------------------------|---------------------------------------------------------------------|
| Cloud unauthenticated     | Redirect to login or show sign-in action.                           |
| Viewer tries write action | Disabled controls and 403 recovery hint if request is forced.       |
| Agent not installed       | Show native install guidance, with macOS Homebrew first.            |
| Agent connecting          | Show progress and keep command-capable controls disabled.           |
| Agent offline             | Show reconnect action and disable live local operations.            |
| Agent rejects command     | Show command denial and keep workspace state intact.                |
| Browser path submission   | Do not render direct local path field; API rejects forced payloads. |

## Design Decisions

| Decision                                        | Rationale                                                                    |
|-------------------------------------------------|------------------------------------------------------------------------------|
| Extend `useAppState`                            | Runtime and agent context are global startup concerns.                       |
| Gate controls from server-provided capabilities | UI stays aligned with backend role and agent policy.                         |
| Keep current routes                             | Mode changes behavior, not navigation structure.                             |
| Label agent-backed workspaces                   | Users need to know which device owns command execution.                      |
| Use guided agent install, no extension          | Native install is required for Git, files, terminal, and AI CLI integration. |
