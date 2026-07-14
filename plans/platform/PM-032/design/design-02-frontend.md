# Frontend Design: Cloud And Agent Modes

## Overview

The frontend becomes mode-aware through `/api/state`. Local mode keeps the existing app shell and workspace registration UI. Cloud mode adds signed-in user context, role-aware controls, Git URL-only registration, and clear unavailable states for local-only or admin-only features. Agent mode adds connection and offline states without requiring browser extensions.

## App State Model

| Field                   | Type   | Purpose                                                |
|-------------------------|--------|--------------------------------------------------------|
| `mode`                  | string | `local`, `cloud`, or `agent`.                          |
| `user`                  | object | Signed-in Cloud user, or synthetic local user context. |
| `role`                  | string | Effective role for frontend gating.                    |
| `capabilities`          | map    | Feature availability and reason strings.               |
| `workspace.location`    | string | `local_path`, `cloud_clone`, or `agent_local`.         |
| `workspace.agentStatus` | string | Agent availability for agent-backed workspaces.        |

Existing `useAppState` should remain the app-level owner of route, theme, active workspace, and refresh keys. PM-032 extends it with runtime context rather than creating a parallel global store.

## Capability UX

| Feature Area         | Local Mode                               | Cloud Mode                                             | Agent Mode                                             |
|----------------------|------------------------------------------|--------------------------------------------------------|--------------------------------------------------------|
| Workspace register   | Current local path, remote clone, import | Git URL only; local path and import hidden or disabled | Connect agent, then select local repo through agent    |
| System picker/reveal | Current native behavior                  | Disabled with Cloud explanation                        | Native picker/reveal happens inside agent flow         |
| Git status/diff      | Current behavior                         | Available by role and Cloud workspace credential state | Routed to live agent                                   |
| Git commit/push      | Current behavior                         | Editor/admin only                                      | Routed to live agent                                   |
| Terminal and AI CLI  | Current behavior                         | Admin-only or unavailable                              | Routed to live agent when connected and allowed        |
| Runtime verification | Current behavior                         | Admin-only start, read-only history for others         | Routed to live agent when connected and allowed        |
| Settings             | Current behavior                         | Role-gated Cloud settings and integration panels       | Agent-specific connection and local workspace settings |

Unavailable controls should explain the reason in place. Avoid dead buttons and generic errors.

## Cloud Registration Flow

| Step | UI State          | Behavior                                                          |
|------|-------------------|-------------------------------------------------------------------|
| 1    | Registration mode | Cloud shows only Git URL registration.                            |
| 2    | Repository input  | User enters remote URL, name, baseline branch, and source roots.  |
| 3    | Credential status | UI shows provider connection state or credential requirement.     |
| 4    | Clone progress    | Existing streaming registration log is reused for clone progress. |
| 5    | Scan result       | Workspace appears in list and board refreshes after indexing.     |

Cloud mode must not ask for or display clone destination paths.

## Agent Connection Flow

| Step | UI State                | Behavior                                                                 |
|------|-------------------------|--------------------------------------------------------------------------|
| 1    | Connect local workspace | User clicks a Cloud UI action.                                           |
| 2    | Install check           | UI explains Homebrew install on macOS first, with Windows/Linux planned. |
| 3    | Wake agent              | Browser opens `kodestream://connect` after install.                      |
| 4    | Agent status            | UI shows connecting, connected, offline, or unsupported platform.        |
| 5    | Local repo registration | Agent presents native folder picker and returns workspace metadata.      |
| 6    | Workspace use           | UI labels the workspace as local-agent backed and routes actions there.  |

No Chrome extension is part of the flow. Homebrew macOS install is the first supported path; package-manager or installer paths for Windows and Linux are planned follow-ups.

## User And Role UI

| Surface         | Change                                                            |
|-----------------|-------------------------------------------------------------------|
| App shell       | Show signed-in user, role, mode label, and logout in Cloud mode.  |
| Workspace list  | Show workspace location badges: Local, Cloud clone, Agent local.  |
| Workspaces page | Show Cloud and Agent registration options based on capabilities.  |
| Item workspace  | Disable Git/terminal/runtime actions by capability and role.      |
| Settings page   | Separate Cloud account/deployment info from local app settings.   |
| Error boundary  | Keep existing behavior but preserve authorization recovery hints. |

Mode labels should be short and functional, not marketing copy.

## Component Impact

| Component Or Area             | Required Change                                                           |
|-------------------------------|---------------------------------------------------------------------------|
| `web/src/app/useAppState.ts`  | Fetch and expose runtime mode, current user, role, and capabilities.      |
| `web/src/lib/types.ts`        | Add runtime, user, role, capability, workspace location, and agent types. |
| `web/src/shared/api/index.ts` | Add auth/logout and agent discovery/connect calls.                        |
| `WorkspacesPage`              | Render Cloud Git URL flow and Agent connect flow by capability.           |
| `ItemWorkspacePage`           | Gate Git, terminal, runtime, verification, and file actions.              |
| `AISessionLaunchControl`      | Render admin-only or agent-unavailable states in Cloud.                   |
| `EmbeddedTerminalDock`        | Support local, Cloud admin, and Agent-routed terminal sources.            |
| `ReliabilityPanels`           | Show Cloud and Agent capability warnings in health panels.                |

## State And Routing

- Browser routes remain the same for board, explorer, workspaces, and items.
- Cloud auth routes are server routes outside SPA handling.
- Frontend fetch handling must redirect or show login-required state on 401 in Cloud mode.
- A 403 should show role/access copy without logging the user out.
- A 503 capability error should show unavailable state and recovery hint.

## Empty And Failure States

| State                       | User-Facing Behavior                                               |
|-----------------------------|--------------------------------------------------------------------|
| Cloud unauthenticated       | Redirect to login or show sign-in action.                          |
| Viewer tries write action   | Disabled controls and 403 recovery hint if request is forced.      |
| Git credential missing      | Workspace registration explains provider connection requirement.   |
| Agent not installed         | Show macOS Homebrew setup first, Windows/Linux planned.            |
| Agent offline               | Show reconnect action and disable live local operations.           |
| Cloud mode local path input | Do not render local path field; API still rejects forced payloads. |

## Design Decisions

| Decision                                        | Rationale                                                                     |
|-------------------------------------------------|-------------------------------------------------------------------------------|
| Extend `useAppState`                            | Runtime context is global and already loaded during app startup.              |
| Gate controls from server-provided capabilities | Keeps UI aligned with backend policy and avoids hard-coded frontend guesses.  |
| Keep current routes                             | Mode changes behavior, not navigation structure.                              |
| Label workspace location visibly                | Users need to know where files and Git operations actually run.               |
| Use guided agent install, no extension          | Native install is required for Git/files/terminal; extension adds complexity. |
