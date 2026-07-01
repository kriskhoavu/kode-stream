# Implementation Plan: PM-019 - Read-Only Jira Integration

## Overview

Deliver per-workspace Jira configuration, Cloud and Server/Data Center read adapters, normalized issue display, and safe attachment access.

## Phases Summary

| Phase | Name                     | Status |
|-------|--------------------------|--------|
| B1    | Connection Configuration | Done   |
| B2    | Issue Adapters And Cache | Done   |
| B3    | Attachment Proxy         | Done   |
| F1    | Workspace Jira Settings  | Draft  |
| F2    | Item Jira Side Panel     | Draft  |
| V1    | Integrated Verification  | Draft  |

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

- [ ] Add shared Jira connection types and API method.
- [ ] Extend workspace create/edit UI with optional deployment-specific settings.
- [ ] Add explicit connection test and state-specific recovery guidance.
- [ ] Test validation, token-reference handling, settings persistence, and failures.

**Verification:** `npm run typecheck && npm test -- --run web/src/pages/WorkspacesPage.test.ts web/src/features/jira-settings`

**Commit:** `PM-019: Add workspace Jira settings`

## Phase F2: Item Jira Side Panel

**Deliverables:**

- [ ] Add independent issue-loading and refresh hook.
- [ ] Render normalized fields and all typed issue states.
- [ ] Add safe description view and metadata-first attachment list.
- [ ] Test remote failure isolation, long content, refresh, and attachment errors.

**Verification:** `npm run typecheck && npm test -- --run web/src/pages/ItemWorkspacePage.test.ts web/src/features/jira`

**Commit:** `PM-019: Add Jira item side panel`

## Phase V1: Integrated Verification

**Deliverables:**

- [ ] Exercise Cloud and Server fixture servers with representative responses.
- [ ] Confirm secrets and Jira content do not enter Git files, indexes, logs, or audit payloads.
- [ ] Update architecture, requirements baseline, and configuration documentation.
- [ ] Run full backend, frontend, and production build checks.

**Verification:** `go test ./... && npm run typecheck && npm test -- --run && npm run build && go build ./cmd/plan-manager`

**Commit:** `PM-019: Verify read-only Jira integration`
