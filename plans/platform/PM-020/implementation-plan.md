# Implementation Plan: PM-020 - Embedded AI Terminal

## Overview

Add a bounded embedded AI terminal using PM-018 contracts while retaining external launch as a fallback.

## Prerequisites

- PM-018 external launch settings, providers, eligibility, and context generation are complete.

## Phases Summary

| Phase | Name                    | Status   |
|-------|-------------------------|----------|
| B1    | PTY Session Lifecycle   | Complete |
| F1    | Embedded Terminal       | Complete |
| V1    | Integrated Verification | Complete |
| F2    | Multi-Session Dock      | Complete |
| F3    | Floating Minimized Mode | Complete |

## Phase B1: PTY Session Lifecycle

**Deliverables:**

- [x] Select a maintained cross-platform Go PTY dependency and record its platform behavior.
- [x] Add session manager, cryptographic IDs/grants, bounded buffers, leases, and process-group cleanup.
- [x] Add create, status, cancel, and typed WebSocket channel endpoints.
- [x] Test input/output, resize, reconnect, limits, cancellation, expiry, and shutdown.

**Verification:** `go test ./internal/application/aisession ./internal/ptysession ./internal/api`

**Commit:** `PM-020: Add embedded AI session lifecycle`

## Phase F1: Embedded Terminal

**Deliverables:**

- [x] Add launch-surface selection to PM-018 workflow.
- [x] Add terminal emulator, typed channel client, resize, reconnect, and lifecycle controls.
- [x] Add navigation guard, accessible focus escape, cancellation, and exit presentation.
- [x] Test connection states, frame handling, reconnect deadline, and cleanup actions.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/ai-session`

**Commit:** `PM-020: Add embedded AI terminal interface`

## Phase V1: Integrated Verification

**Deliverables:**

- [x] Test PTY lifecycle on supported development platforms and external-launch fallback.
- [x] Confirm session grants and terminal content never enter logs or audit payloads.
- [x] Update architecture, requirements baseline, security guidance, and user documentation.

**Verification:** `go test ./... && npm run typecheck && npm test -- --run && npm run build && go build ./cmd/plan-manager`

**Commit:** `PM-020: Verify embedded AI terminal`

## Phase F2: Multi-Session Dock

**Deliverables:**

- [x] Move embedded session ownership from the item header to an app-level dock.
- [x] Keep multiple sessions connected across item and workspace navigation.
- [x] Add session switching plus minimized, normal, and maximized modes.
- [x] Refit the terminal after every presentation change to prevent TUI overlap and clipping.
- [x] Test multiple sessions, workspace labels, mode changes, and cleanup.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/ai-session web/src/App.test.tsx`

**Commit:** `PM-020: Add multi-session terminal dock`

## Phase F3: Floating Minimized Mode

**Deliverables:**

- [x] Keep the active terminal visible in a compact bottom-right window when minimized.
- [x] Allow interaction with the application outside the floating terminal.
- [x] Preserve session switching, input, output, resize, restore, and close controls.
- [x] Test non-modal minimized presentation and restoration.

**Verification:** `npm run typecheck && npm test -- --run web/src/features/ai-session`

**Commit:** `PM-020: Add floating minimized terminal`
