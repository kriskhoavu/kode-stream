# Backend Design: PM-029

## Overview

The backend supports automation verification as a separate job mode. It stores workspace automation settings, persists item-selected specs, discovers candidate specs from structured plan metadata, renders runner commands, executes them in the automation repository, and returns unified artifacts.

No database schema is required. Workspace settings live in workspace config, item run selections live in item `plan.yaml`, and planned automation paths live in feature/test `plan.yaml` files.

## Data Model

### Workspace Automation Config

| Field                | Type     | Purpose                                                |
|----------------------|----------|--------------------------------------------------------|
| `enabled`            | boolean  | Enables automation controls and runs                   |
| `repositoryPath`     | string   | Absolute path to the automation repository             |
| `runner`             | string   | `cypress` or `playwright`                              |
| `defaultEnvironment` | string   | Default card environment, usually `local` or `nightly` |
| `commandTemplate`    | string   | Shell command template rendered at run time            |
| `artifactPaths`      | string[] | Automation repo paths to collect after a run           |

### Item Verification Metadata

| Field           | Type     | Purpose                                    |
|-----------------|----------|--------------------------------------------|
| `selectedSpecs` | string[] | Spec paths relative to the automation repo |
| `environment`   | string   | Item-level environment override            |
| `displayMode`   | string   | `silent` or `visible` browser mode         |
| `updatedAt`     | string   | Timestamp for display and conflict clarity |

### Planned Automation Metadata

| Field                          | Type    | Purpose                                            |
|--------------------------------|---------|----------------------------------------------------|
| `automation-test[].path`       | string  | Planned Cypress or Playwright spec path suggestion |
| `plan.wiki_enriched`           | boolean | Wiki enrichment state stored in `plan.yaml`        |

Empty `automation-test[].path` entries are valid placeholders and must be ignored by discovery.

## API Contract

| Method | Endpoint                                 | Request                               | Response                              |
|--------|------------------------------------------|---------------------------------------|---------------------------------------|
| `GET`  | `/api/workspaces/{id}/runtime`           | none                                  | Runtime config with automation        |
| `PUT`  | `/api/workspaces/{id}/runtime`           | Runtime config                        | Saved runtime config                  |
| `GET`  | `/api/items/{itemId}/verification-tests` | none                                  | Selection plus discovered specs       |
| `PUT`  | `/api/items/{itemId}/verification-tests` | Specs, environment, display mode      | Saved selection plus discovered specs |
| `POST` | `/api/workspaces/{id}/verification-jobs` | Runtime profile or automation payload | Verification job                      |

Automation job payload includes `mode: automation`, selected specs, environment, and display mode.

## Discovery Strategy

Spec discovery must be fast and metadata-first:

1. Look for likely `plan.yaml` files in the automation repo using item identifier, scope, and item ID.
2. Return non-empty `automation-test[].path` entries as discovered specs.

This avoids slow full-repo Markdown scans during Quality panel load.

## Command Rendering

| Placeholder  | Source / Meaning                                   |
|--------------|----------------------------------------------------|
| `{env}`      | Item environment or workspace default              |
| `{specs}`    | Selected spec paths joined for the runner          |
| `{modeArgs}` | Runner-specific args for visible mode              |
| `{headed}`   | Headed-only flag when visible mode is selected     |
| `{browser}`  | Browser/project flag when visible mode is selected |

If visible mode is selected and the template has no mode placeholder, the backend appends runner defaults:

| Runner     | Visible mode args             |
|------------|-------------------------------|
| Cypress    | `--headed --browser chrome`   |
| Playwright | `--headed --project=chromium` |

## Execution Flow

```text
Create automation job
  -> validate runtime automation config
  -> validate selected specs are relative and inside automation repo
  -> create artifact root
  -> run prepare, up, and health in app workspace
  -> run rendered command in automation repository
  -> collect runtime and automation artifacts
  -> run teardown
  -> expose status, steps, logs, and artifacts
```

## Knowledge Flag Risk

The Knowledge page implementation does not depend on `wiki_enriched`. It scans `docs/**` wiki pages and parses wiki frontmatter. Moving `wiki_enriched` to `plan.yaml` affects wiki enrichment skill behavior and migration scripts, not Knowledge page rendering.
