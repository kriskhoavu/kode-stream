# Frontend Design: Runtime-Adaptive Verification Harness

## Overview

Extend workspace configuration and Workstream run surfaces so users can define per-app runtime instructions and observe automatic checkpoint verification results. The UI supports heterogeneous runtime styles while preserving one consistent run experience.

## Routes And Navigation

| Route | Behavior |
|-------|----------|
| `/workspaces` | Workspace settings include Runtime configuration |
| `/workstream` | Board and intake surface; verification controls are not hosted here |
| `/items/:id` | Primary verification panel with run controls, trigger badges, timeline, and artifacts |

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
| Verify run state | Item details page | Polls latest job and updates timeline incrementally |
| Artifact availability | Verification section | Enables preview/open actions only when artifacts exist |
| Retry controls | Verification section | Allows rerun with selected profile and handles busy state |
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

- In item details `Info` tab, add run status timeline with states:
  - `Preparing`
  - `Coding`
  - `Verifying`
  - `Fixing`
  - `Passed`
  - `Failed`
- Show current verification profile and elapsed time.
- Show provider and terminal mode badges (`Embedded`, `External`) as context only.
- Show latest verification status, trigger source badge, and failure category.
- Artifacts panel lists logs, report, screenshot, video, and trace.
- Add in-app Preview dialog for text artifacts and external Open action.
- Actions:
  - `Re-run Smoke`
  - `Re-run Profile`
  - `Open Trace` (when available)
  - `Open Report`

## Implemented Visualization Flow

```text
verification job starts
  -> item details page polls verification job endpoint
  -> verification section shows profile + status + trigger source badge
  -> timeline section renders step results (prepare/up/health/verify/down)
  -> artifact section renders indexed files with Preview/Open actions
  -> rerun action creates a new job and refreshes the same section
```

Trigger source badge behavior:

- `manual` for direct Run verify action.
- `rerun` for rerun requests.
- `checkpoint:* (embedded)` for embedded session completion.
- `checkpoint:* (external)` for external terminal completion wrappers.

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
