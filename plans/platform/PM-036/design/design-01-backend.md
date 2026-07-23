# Backend Design: PM-036 E2E Quality Panels

## Overview

Add a read-only E2E runbook service beside the existing item and Knowledge services. It resolves only safe workspace-relative Markdown paths, uses plan metadata and Knowledge `sourceRef` data for coverage, and parses the latest result without creating verification jobs. Extend AI launch with workspace-context targets so a wiki runbook can be executed without an item.

## Read Model

| Field          | Type                    | Purpose                                                                     |
|----------------|-------------------------|-----------------------------------------------------------------------------|
| `title`        | string                  | Runbook heading or indexed Knowledge title.                                 |
| `path`         | string                  | Workspace-relative Markdown runbook path.                                   |
| `source`       | `plan` or `wiki`        | Identifies ticket-local versus canonical coverage.                          |
| `resultPath`   | string                  | Safe result path for the runbook owner.                                     |
| `latestResult` | optional result summary | Parsed status, provider, environment, failed step, and evidence references. |
| `diagnostic`   | optional string         | Explains no coverage, unreadable result, or unavailable launch capability.  |

No database schema or persisted run history is added.

## Discovery and Result Rules

1. Read a plan’s `plan.yaml`; only discover ticket-local runbooks when `plan.e2e-runbook` is true.
2. Include the automation hub and scenario Markdown files, excluding `results/` and `artifacts/`.
3. Read indexed Knowledge pages under `e2e-testing/`; include canonical pages only when a `sourceRef` references the selected plan’s automation path.
4. Preserve the order: selected plan local runbooks, then canonical journeys sorted by title.
5. Resolve `automation/results/latest.md` inside the owning runbook directory only. A missing or malformed file produces `not run`, never an API failure.
6. Reject absolute paths, traversal, unsupported extensions, and files outside the workspace.

## API Contract

| Method | Endpoint                                                             | Request                                                        | Response                                             |
|--------|----------------------------------------------------------------------|----------------------------------------------------------------|------------------------------------------------------|
| GET    | `/api/items/{itemId}/e2e-runbooks`                                   | none                                                           | Ordered E2E runbook summaries for the selected plan. |
| GET    | `/api/knowledge/wikis/{workspaceId}/{root}/pages/{slug}/e2e-runbook` | none                                                           | E2E summary for the selected canonical wiki page.    |
| POST   | `/api/workspaces/{workspaceId}/ai-sessions`                          | Existing launch input plus validated E2E context path          | Workspace-context session launch result.             |
| POST   | `/api/workspaces/{workspaceId}/ai-sessions/embedded`                 | Existing embedded launch input plus validated E2E context path | Embedded workspace-context session result.           |

Existing item AI session routes remain unchanged.

## Design Decisions

| Decision                                    | Rationale                                                                |
|---------------------------------------------|--------------------------------------------------------------------------|
| New read model instead of `VerificationJob` | E2E is agent-executed Markdown coverage, not a local runner process.     |
| Source-reference matching                   | Canonical pages already express durable ticket-to-journey provenance.    |
| Workspace AI launch route                   | Knowledge execution needs a workspace path but does not have an item ID. |
| Result parser is tolerant                   | A result may not exist until the first successful or failed execution.   |

## Verification

- Unit-test plan discovery, source-reference matching, result parsing, ordering, and path rejection.
- Add API route tests for plan and Knowledge reads plus workspace-context launch validation.
- Preserve existing item launch and verification route tests.
