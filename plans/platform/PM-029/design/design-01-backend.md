# Backend Design: PM-029

## Overview

The backend extends the existing runtime verification model with optional automation runner settings, card-level selected specs, discovery of candidate specs, and a verification job mode that runs selected specs in an external automation repository. No database schema is required because workspace settings live in `workspaces.yaml` and item metadata lives in each item's `plan.yaml`.

## Data Model

### Workspace Automation Config

| Field                | Type     | Purpose                                                                |
|----------------------|----------|------------------------------------------------------------------------|
| `enabled`            | boolean  | Controls whether automation test features are active for the workspace |
| `repositoryPath`     | string   | Absolute path to the external automation repository                    |
| `runner`             | string   | Runner type: `cypress`, `playwright`, or `custom`                      |
| `defaultEnvironment` | string   | Default test environment, initially `local`                            |
| `commandTemplate`    | string   | Shell command template rendered for selected specs                     |
| `artifactPaths`      | string[] | Paths inside the automation repo to collect after a run                |

### Item Verification Metadata

| Field           | Type     | Purpose                                                      |
|-----------------|----------|--------------------------------------------------------------|
| `selectedSpecs` | string[] | Workspace-independent spec paths relative to automation repo |
| `environment`   | string   | Optional item-level default environment override             |
| `updatedAt`     | string   | Timestamp used for display and conflict clarity              |

The item metadata writer stores this under the existing item `plan.yaml`. Plan Manager should keep the metadata sparse and only write the verification section when selected specs or an environment override exists.

### Verification Job Extensions

| Field                | Type     | Purpose                                                              |
|----------------------|----------|----------------------------------------------------------------------|
| `mode`               | string   | `runtime` for existing profiles, `automation` for selected spec runs |
| `environment`        | string   | Effective automation environment                                     |
| `selectedSpecs`      | string[] | Specs used by the job                                                |
| `automationRepoPath` | string   | Repository path used by the job                                      |
| `renderedCommand`    | string   | Final command stored for logs and auditability                       |

Existing `profile`, `trigger`, `provider`, `sessionId`, and `terminalMode` remain backward compatible.

## API Contract

| Method | Endpoint                                                   | Request                                 | Response                              |
|--------|------------------------------------------------------------|-----------------------------------------|---------------------------------------|
| `GET`  | `/api/workspaces/{id}/runtime`                             | none                                    | Runtime config with automation fields |
| `PUT`  | `/api/workspaces/{id}/runtime`                             | Runtime config with automation fields   | Saved runtime config                  |
| `GET`  | `/api/items/{itemId}/verification-tests`                   | none                                    | Selected specs and discovered specs   |
| `PUT`  | `/api/items/{itemId}/verification-tests`                   | Selected specs and optional environment | Saved selected specs                  |
| `POST` | `/api/workspaces/{id}/verification-jobs`                   | Runtime profile or automation run input | Verification job                      |
| `GET`  | `/api/workspaces/{id}/verification-jobs/{jobId}`           | none                                    | Verification job                      |
| `GET`  | `/api/workspaces/{id}/verification-jobs/{jobId}/artifacts` | none                                    | Verification artifacts                |

## Command Rendering

| Template Variable | Source                                                   |
|-------------------|----------------------------------------------------------|
| `{env}`           | Request environment, item override, or workspace default |
| `{specs}`         | Selected spec paths joined for the runner                |
| `{workspacePath}` | Main app workspace path                                  |
| `{itemId}`        | Current item identifier                                  |
| `{itemPath}`      | Current item path inside workspace                       |

The default Cypress template runs `npx cypress run --spec` with selected specs and the effective environment. Playwright can later use the same field with a different runner default.

## Execution Flow

```text
Start verification job
  -> resolve workspace runtime config
  -> if mode is runtime, run existing profile command
  -> if mode is automation, validate automation config and selected specs
  -> create artifact root under main workspace
  -> run prepare, up, health in main workspace
  -> render automation command
  -> run command in automation repository
  -> collect runtime and automation artifacts
  -> run teardown
  -> mark passed or failed
```

## Validation Rules

| Rule                                             | Error Behavior                                |
|--------------------------------------------------|-----------------------------------------------|
| Automation repository must exist and be a folder | Reject save or run when automation is enabled |
| Selected specs must be relative paths            | Reject absolute paths                         |
| Selected specs must stay inside automation repo  | Reject path traversal                         |
| Automation command template must be non-empty    | Reject automation run                         |
| Runtime smoke command remains required           | Preserve existing runtime validation behavior |
| Unknown runner values are invalid                | Reject save                                   |

## Design Decisions

| Decision                                           | Rationale                                                              |
|----------------------------------------------------|------------------------------------------------------------------------|
| Store automation config in workspace runtime       | The automation runner is part of how a workspace verifies itself       |
| Store selected specs in item metadata              | Test links belong to the card and should move with the planning item   |
| Keep existing runtime mode as the default          | Existing API callers that post only `profile` must continue to work    |
| Add a separate automation mode                     | Automation runs need selected specs, environment, and repo context     |
| Run automation in the external repo working dir    | Test dependencies, Cypress config, and reports live in that repository |
| Collect artifacts from both runtime and test repos | Users should inspect runtime logs and test runner output in one job    |
