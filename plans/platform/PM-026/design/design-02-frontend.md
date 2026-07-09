# Frontend Design: Runtime-Adaptive Verification Harness

## Overview

Extend workspace configuration and Workstream run surfaces so users can define per-app runtime instructions and observe automatic checkpoint verification results. The UI supports heterogeneous runtime styles while preserving one consistent run experience.

## Routes And Navigation

| Route | Behavior |
|-------|----------|
| `/workspaces` | Workspace settings include Runtime configuration |
| `/workstream` | Shows run timeline, latest verify result, and artifacts during AI task execution |
| `/items/:id` | Existing item view remains unchanged and links to latest verification state where available |

## Data Model

| Type | Fields | Owner |
|------|--------|-------|
| `WorkspaceRuntimeConfig` | runtimeType, configPath, command set, rebuildPolicy, healthChecks, artifacts | Workspace settings state |
| `RuntimeAdapterType` | docker-compose, procfile, makefile, custom | Runtime settings form |
| `VerificationProfile` | smoke, critical, full | Verify trigger controls |
| `VerificationJobView` | id, status, profile, failureType, step timeline | Workstream run panel |
| `RunArtifactView` | kind, label, path/url, available | Artifact list and viewers |
| `SessionRunContext` | provider, sessionId, terminalMode, lastCheckpointAt | AI session attribution for run events |

## State Management

| State | Owner | Behavior |
|-------|-------|----------|
| Runtime form state | Workspace settings feature | Tracks adapter-specific fields and validation |
| Verify run state | Workstream page | Polls latest job and updates timeline incrementally |
| Artifact availability | Run panel | Enables links only when artifacts exist |
| Retry controls | Run panel | Allows rerun with selected profile and handles busy state |
| Failure summary state | AI integration UI bridge | Displays normalized failure explanation and retry context status |
| Session context state | Workstream page | Shows provider name and terminal mode without changing verify behavior |

## Runtime Settings UX

- Add a `Runtime` section in workspace settings.
- Adapter selector changes required fields dynamically:
  - Docker Compose: compose file path and project options.
  - Procfile: Procfile path and process manager command.
  - Makefile: make targets for up, down, and verify.
  - Custom: explicit command fields.
- Profile commands are grouped under `Smoke`, `Critical`, and `Full`.
- Rebuild policy selector includes help text:
  - `Never`
  - `Changed Only` (recommended)
  - `Always`
- Health checks support URL and command types.
- Save action validates before submission and shows field-level errors.

## Workstream Run UX

- Add run status timeline with states:
  - `Preparing`
  - `Coding`
  - `Verifying`
  - `Fixing`
  - `Passed`
  - `Failed`
- Show current verification profile and elapsed time.
- Show provider and terminal mode badges (`Embedded`, `External`) as context only.
- Show latest app preview link when health checks pass.
- Artifacts panel lists logs, report, screenshot, video, and trace.
- Actions:
  - `Re-run Smoke`
  - `Re-run Profile`
  - `Open Trace` (when available)
  - `Open Report`

## Failure Experience

- Show concise failure reason by category:
  - App failed to boot.
  - Tests failed.
  - Runtime infrastructure error.
- Provide quick artifact links near the failure message.
- Keep previous successful run visible until replaced so users do not lose context.
- Preserve failure and artifact visibility when session continues in an external terminal.

## Provider And Terminal Behavior

- The Workstream verification view is provider-agnostic.
- Claude, Codex, OpenCode, and future providers map to the same checkpoint timeline.
- Terminal mode controls where the user interacts with AI, not how verification is executed.
- External terminals (for example WezTerm) use the same backend run updates and artifact views.

## Accessibility And Responsive Behavior

- Timeline and artifact updates use accessible live regions.
- Keyboard navigation supports adapter form sections and run actions.
- On smaller screens, run timeline stacks above artifacts and preview links.
- Long paths and command lines wrap safely without overflow.

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Keep runtime config in workspace settings | Users already configure app-specific behavior there |
| Show one normalized run timeline across adapters | Reduces cognitive load regardless of runtime type |
| Prefer profile-first actions over raw commands | Keeps AI and user workflows safe and consistent |
| Keep artifact panel always visible in failed states | Speeds triage and retry loops |
| Treat missing runtime config as explicit setup state | Avoids silent fallback to unsafe assumptions |
| Keep run view provider-agnostic | Avoids UI branching and behavior drift between AI vendors |
| Treat terminal mode as context metadata | Keeps verification behavior consistent between embedded and external sessions |
