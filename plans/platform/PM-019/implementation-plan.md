# Implementation Plan: PM-019 - Jira Integration and Workspace Settings Redesign

## Overview

Preserve the completed read-only Jira integration and redesign workspace management around a compact workspace list, tabbed workspace details, a focused registration flow, and separate global storage settings.

## Phases Summary

| Phase | Name                     | Status  |
|-------|--------------------------|---------|
| B1    | Connection Configuration | Done    |
| B2    | Issue Adapters And Cache | Done    |
| B3    | Attachment Proxy         | Done    |
| F1    | Workspace Jira Settings  | Done    |
| F2    | Item Jira Side Panel     | Done    |
| V1    | Integrated Verification  | Done    |
| F3    | Workspace Manager Shell  | Done    |
| F4    | Workspace Settings Tabs  | Done    |
| F5    | Registration And Storage | Done    |
| V2    | Redesign Verification    | Pending |

## Phase B1: Connection Configuration

**Deliverables:**

- [x] Extend workspace models, validation, normalization, and registry compatibility.
- [x] Resolve tokens only through configured environment-variable names.
- [x] Add Cloud and Server connection test clients and endpoint.
- [x] Add redaction, URL, deployment-specific, and API tests.

**Verification:** `go test ./internal/registry ./internal/application/jira ./internal/api`

**Commit:** `PM-019: Add workspace Jira connection configuration`

## Phase B2: Issue Adapters And Cache

**Deliverables:**

- [x] Add normalized issue, description, person, and attachment DTOs.
- [x] Implement Cloud and Server/Data Center issue readers.
- [x] Add exact identifier matching, typed states, five-minute cache, and refresh.
- [x] Test ADF, Server variants, 404, authentication, authorization, timeout, and malformed responses.

**Verification:** `go test ./internal/jira ./internal/application/jira ./internal/api`

**Commit:** `PM-019: Add normalized Jira issue reads`

## Phase B3: Attachment Proxy

**Deliverables:**

- [x] Validate issue and attachment ownership before remote access.
- [x] Add bounded streaming, redirect, filename, media-type, and response-header controls.
- [x] Add safe inline allowlist and forced-download behavior.
- [x] Test oversized, spoofed, redirected, timed-out, and missing attachments.

**Verification:** `go test ./internal/jira ./internal/application/jira ./internal/api`

**Commit:** `PM-019: Add safe Jira attachment access`

## Phase F1: Workspace Jira Settings

**Deliverables:**

- [x] Add shared Jira connection types and API method.
- [x] Extend workspace create/edit UI with optional deployment-specific settings.
- [x] Add explicit connection test and state-specific recovery guidance.
- [x] Test validation, token-reference handling, settings persistence, and failures.

**Verification:** `npm run typecheck && npm test -- --run web/src/pages/WorkspacesPage.test.ts web/src/features/jira-settings`

**Commit:** `PM-019: Add workspace Jira settings`

## Phase F2: Item Jira Side Panel

**Deliverables:**

- [x] Add independent issue-loading and refresh hook.
- [x] Render normalized fields and all typed issue states.
- [x] Add safe description view and metadata-first attachment list.
- [x] Test remote failure isolation, long content, refresh, and attachment errors.

**Verification:** `npm run typecheck && npm test -- --run web/src/pages/ItemWorkspacePage.test.ts web/src/features/jira`

**Commit:** `PM-019: Add Jira item side panel`

## Phase V1: Integrated Verification

**Deliverables:**

- [x] Exercise Cloud and Server fixture servers with representative responses.
- [x] Confirm secrets and Jira content do not enter Git files, indexes, logs, or audit payloads.
- [x] Update architecture, requirements baseline, and configuration documentation.
- [x] Run full backend, frontend, and production build checks.

**Verification:** `go test ./... && npm run typecheck && npm test -- --run && npm run build && go build ./cmd/plan-manager`

**Commit:** `PM-019: Verify read-only Jira integration`

## Phase F3: Workspace Manager Shell

**Deliverables:**

- [x] Extract workspace list behavior into a focused feature component while retaining page-level operation coordination.
- [x] Add the master-detail page shell, compact searchable workspace list, stable selection, and responsive list-to-detail navigation.
- [x] Add page-level `Add workspace` and `Scan all` actions without rendering creation or edit forms in the list.
- [x] Add explicit bulk-selection mode and preserve current confirmed removal behavior.
- [x] Replace the global busy flag with operation-scoped pending state so navigation and unrelated actions remain available.
- [x] Add list selection, filtering, empty state, bulk mode, and operation isolation coverage.

**Verification:** `npm run typecheck && npm test -- --run web/src/pages/WorkspacesPage.test.ts web/src/features/workspaces`

**Commit:** `PM-019: Add workspace manager shell`

## Phase F4: Workspace Settings Tabs

**Deliverables:**

- [x] Add Overview, Sources, Integrations, and Health tabs for the selected workspace.
- [x] Move editable identity, path, baseline branch, registration metadata, and destructive removal into Overview.
- [x] Replace source chips with source rows and retain the existing source-structure dialog through labeled Configure actions.
- [x] Move Jira fields and connection testing into Integrations without changing existing API or secret-handling behavior.
- [x] Move detailed health checks and scanning into Health while retaining a compact scan summary in the list.
- [x] Add domain-scoped drafts, save behavior, navigation guards, keyboard tab behavior, and focused component tests.

**Verification:** `npm run typecheck && npm test -- --run web/src/pages/WorkspacesPage.test.ts web/src/features/workspaces web/src/features/jira-settings web/src/components/ReliabilityPanels.test.tsx`

**Commit:** `PM-019: Organize workspace settings by domain`

## Phase F5: Registration And Storage

**Deliverables:**

- [x] Add a focused Add Workspace dialog with local folder and remote Git URL modes.
- [x] Infer workspace name and present branch and source defaults as reviewable fields.
- [x] Keep optional Jira configuration under Advanced settings and available after registration.
- [x] Keep remote clone progress, logs, failure details, retry, and successful selection inside the focused flow.
- [x] Move Data Directory configuration to application Settings under Storage with browse, reveal, restart guidance, and existing API behavior.
- [x] Add registration, storage, accessibility, and unsaved-close tests while retaining existing payload and clone coverage.

**Verification:** `npm run typecheck && npm test -- --run web/src/pages/WorkspacesPage.test.ts web/src/features/workspaces web/src/features/settings`

**Commit:** `PM-019: Add focused workspace registration and storage settings`

## Phase V2: Redesign Verification

**Deliverables:**

- [ ] Verify local registration, remote cloning, editing, scanning, source configuration, Jira testing, health inspection, and removal end to end.
- [ ] Verify keyboard navigation, focus restoration, narrow-screen navigation, long paths, empty states, errors, and unsaved-change guards.
- [ ] Confirm existing workspace and Jira API contracts, persisted YAML, token redaction, and attachment behavior remain unchanged.
- [ ] Update architecture, README screenshots or usage documentation, and planning documents to match final component names.
- [ ] Run full backend, frontend, production build, and browser visual checks.

**Verification:** `go test ./... && npm run typecheck && npm test -- --run && npm run build && go build ./cmd/plan-manager`

**Commit:** `PM-019: Verify workspace settings redesign`
