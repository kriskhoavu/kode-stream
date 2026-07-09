# Scenarios: PM-026 Overview

## Scenario List

| # | Title | Description |
|---|-------|-------------|
| 0 | Runtime contract setup | User configures runtime adapter and verification profiles per workspace |
| 1 | Fast verify on large compose app | Agent runs checkpoint verification without full rebuild loops |
| 2 | Procfile and Makefile compatibility | Non-compose projects run through the same verification lifecycle |
| 3 | Embedded and external terminal parity | Verify loop behaves the same for embedded and external terminals |
| 4 | Verification failure retry loop | Failure artifacts are returned to AI session for automatic next fix step |
| 5 | Missing or invalid runtime config | System blocks unsafe execution and provides clear guidance |

---

# Scenario 0: Runtime Contract Setup

## Starting State

- A workspace is already registered.
- The workspace has no runtime instructions yet.
- User wants AI-assisted implementation with automatic run-and-test checks.

## Execution Flow

```text
User opens Workspace Settings
  -> opens Runtime section
  -> chooses adapter type (docker-compose, procfile, makefile, custom)
  -> fills required commands and profile commands
  -> saves configuration
  -> backend validates and persists contract
  -> Workstream marks workspace as verify-ready
```

## Expected Result

- Runtime config is stored per workspace.
- Adapter-specific required fields are enforced.
- Invalid commands or paths are rejected with field-level errors.
- Existing workspaces without config continue to function normally.

---

# Scenario 1: Fast Verify On Large Compose App

## Goal

Run iterative implementation checkpoints on a multi-service docker-compose app without rebuilding every image each time.

## Execution Flow

```text
AI session applies implementation changes
  -> checkpoint triggers verify(smoke)
  -> runtime adapter starts compose services with no-build default
  -> changed-only rebuild policy checks impacted services
  -> only impacted services are rebuilt (if needed)
  -> health checks pass
  -> Playwright smoke tests run
  -> artifacts are stored and linked to run result
```

## Expected Result

- No full-compose rebuild on every checkpoint.
- Smoke profile starts only required subset of services.
- Verification is faster than full-stack startup.
- Pass/fail plus logs and Playwright artifacts are available immediately.

---

# Scenario 2: Procfile And Makefile Compatibility

## Goal

Ensure teams with Procfile or Makefile workflows can use the same run-and-verify loop.

## Execution Flow

```text
Workspace uses Procfile or Makefile adapter
  -> verify(profile) triggers adapter-specific up command
  -> health checks execute
  -> Playwright profile runs
  -> adapter down command executes on completion/failure
```

## Expected Result

- Runtime type is transparent to the verification pipeline.
- Result model and artifact locations stay consistent across adapters.
- Teams do not need to migrate to docker-compose to use PM-026 workflows.

---

# Scenario 3: Embedded And External Terminal Parity

## Goal

Ensure terminal choice does not change verification behavior.

## Execution Flow

```text
User starts AI session in embedded terminal
  -> checkpoint triggers verify(smoke)
  -> job runs and artifacts appear in Workstream
User starts AI session in external terminal (for example WezTerm)
  -> checkpoint triggers verify(smoke)
  -> same job lifecycle and artifacts appear in Workstream
```

## Expected Result

- Embedded and external terminal sessions create the same verification jobs.
- Provider and terminal mode are displayed as context only.
- Retry actions and artifact links are identical for both modes.

---

# Scenario 4: Verification Failure Retry Loop

## Goal

Convert failed run output into structured AI retry context.

## Execution Flow

```text
verify(profile) fails
  -> system classifies failure (boot, tests, infra)
  -> failure summary and key artifact links are generated
  -> active AI session receives retry payload
  -> AI proposes and applies fix
  -> next checkpoint triggers verify again
```

## Expected Result

- AI receives concise, actionable error context.
- User can open trace, video, and logs from Workstream.
- Retry loop continues until success or run budget is exhausted.

---

# Scenario 5: Missing Or Invalid Runtime Config

## Goal

Prevent unsafe or confusing execution when runtime instructions are absent or broken.

## Edge Cases

| Case | Expected Behavior |
|------|-------------------|
| Runtime config missing | Show setup-required state and disable auto-verify |
| Adapter path invalid | Return validation error with failing field |
| Required command missing | Reject save and show required command hints |
| Health endpoint never ready | Mark boot failure and skip Playwright |
| Verify command exits non-zero | Mark test failure and attach command output |
| Down command fails | Preserve failure result and include teardown warning |
