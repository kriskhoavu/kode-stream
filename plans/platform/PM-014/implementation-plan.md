# Implementation Plan: PM-014 - Visual Source Structure Proposals

## Overview

Add source-derived proposals and visual previews to the Source Structure configuration flow while preserving the existing `workspace-settings.yaml` save contract.

## Phases Summary

| Phase | Name                     | Status |
|-------|--------------------------|--------|
| B1    | Proposal And Preview API | Done   |
| F1    | Visual Source Dialog     | Done   |

## Terminology Lock

All code, fields, API params, and TS types must use:

- `SourceStructureProposal`
- `SourceStructurePreview`
- `SourceStructureSegmentRole`
- `workspace-settings.yaml`

## Backend Phases

### Phase B1: Proposal And Preview API

**Deliverables:**

- [x] Extend `internal/models.SourceSettingsResult` with `proposals` and `preview`.
- [x] Add scanner helpers that sample source paths, generate candidate `SourceStructureCard` rules, and preview matched rows.
- [x] Include proposals and preview in `workspace.Service.SourceStructure`.
- [x] Add backend tests for proposal generation and preview rows.

**Verification:** `rtk go test ./internal/scanner ./internal/application/workspace ./internal/api`

**Commit:** `PM-014: Add source structure proposal API`

---

## Frontend Phases

### Phase F1: Visual Source Dialog

**Deliverables:**

- [x] Extend TypeScript API types for proposals and previews.
- [x] Add proposal cards to the Source Structure dialog.
- [x] Add clickable path segment role controls for common pattern edits.
- [x] Add a preview table showing path, scope, identifier, title, status, and tags.
- [x] Keep advanced path pattern and field inputs available.
- [x] Add frontend tests for applying proposals and preview rendering.

**Verification:** `rtk npm test -- --run web/src/pages/WorkspacesPage.test.ts web/src/features/workspaces/sourceSettings.test.ts && rtk npm run build`

**Commit:** `PM-014: Add visual source structure dialog`

---

## Post-Implementation Checklist

- [x] Confirm existing saved `workspace-settings.yaml` files still load and save.
- [x] Confirm freestyle docs roots still fall back to one unsorted docs card when no settings are saved.
- [x] Confirm proposal preview uses real README headings when present.
- [x] Run Markdown formatting on `plans/platform/PM-014/**/*.md`.
