# Implementation Plan: PM-021 - Guarded Jira Editing

## Overview

Add controlled Jira field, transition, and attachment writes using PM-019 contracts.

## Prerequisites

- PM-019 Jira connections, normalized reads, cache invalidation, and attachment controls are complete.

## Phases Summary

| Phase | Name                             | Status |
|-------|----------------------------------|--------|
| B1    | Jira Field And Transition Writes | Draft  |
| F1    | Jira Edit View                   | Draft  |
| B2    | Jira Attachment Mutations        | Draft  |
| F2    | Attachment Management            | Draft  |
| V1    | Integrated Verification          | Draft  |

## Phase B1: Jira Field And Transition Writes

**Deliverables:**

- [ ] Extend deployment adapters with edit metadata, field update, and transition methods.
- [ ] Add allowlisted field policy and issue-version conflict checks.
- [ ] Add edit metadata, update, and transition endpoints.
- [ ] Invalidate cache, refetch normalized issue, and write redacted audit events.

**Verification:** `go test ./internal/jira ./internal/application/jira ./internal/api`

**Commit:** `PM-021: Add guarded Jira issue mutations`

## Phase F1: Jira Edit View

**Deliverables:**

- [ ] Add route, shared mutation types, and edit hook.
- [ ] Render supported field controls and valid transition selection.
- [ ] Add change confirmation, conflict comparison, refresh, and recovery states.
- [ ] Test permissions, stale versions, failed transitions, and successful refresh.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/jira-edit web/src/app/router.test.ts`

**Commit:** `PM-021: Add dedicated Jira edit workflow`

## Phase B2: Jira Attachment Mutations

**Deliverables:**

- [ ] Add deployment-adapter upload and deletion methods.
- [ ] Add bounded multipart parsing and shared PM-019 attachment policy checks.
- [ ] Verify membership and confirmation before deletion.
- [ ] Return per-file results, refresh issue metadata, and add redacted audit events.

**Verification:** `go test ./internal/jira ./internal/application/jira ./internal/api`

**Commit:** `PM-021: Add guarded Jira attachment mutations`

## Phase F2: Attachment Management

**Deliverables:**

- [ ] Add file queue, client validation, upload progress, and per-file result handling.
- [ ] Add attachment deletion confirmation and conflict recovery.
- [ ] Keep successful partial results and refresh metadata after mutations.
- [ ] Test limits, partial failure, cancellation, deletion, and inaccessible files.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/jira-edit`

**Commit:** `PM-021: Add Jira attachment management interface`

## Phase V1: Integrated Verification

**Deliverables:**

- [ ] Test Jira mutations against Cloud and Server fixture implementations.
- [ ] Confirm Jira values and attachment contents never enter logs or audit payloads.
- [ ] Update architecture, requirements baseline, security guidance, and user documentation.

**Verification:** `go test ./... && npm run typecheck && npm test -- --run && npm run build && go build ./cmd/kode-stream`

**Commit:** `PM-021: Verify guarded Jira editing`
