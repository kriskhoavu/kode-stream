# Backend Design: Source Items Proposals

## Overview

The backend extends the existing source-structure response with source-derived proposals and preview rows. Saving remains unchanged: the client still sends `SourceStructureSettings`, the backend validates it, writes `workspace-settings.yaml`, and refreshes the workspace scan.

## Data Model

### Entity: SourceStructureProposal

| Field        | Type                       | Purpose                                                  |
|--------------|----------------------------|----------------------------------------------------------|
| `id`         | string                     | Stable UI key such as `scope-feature-identifier`         |
| `label`      | string                     | Short user-facing option label                           |
| `summary`    | string                     | Explanation of what the proposal will create             |
| `confidence` | string                     | `high`, `medium`, or `low`                               |
| `card`       | `SourceStructureCard`      | Candidate rule that can be saved after user confirmation |
| `preview`    | []`SourceStructurePreview` | Sample rows produced by this proposal                    |

### Entity: SourceStructurePreview

| Field        | Type     | Purpose                                             |
|--------------|----------|-----------------------------------------------------|
| `path`       | string   | Source-relative matched card directory path         |
| `scope`      | string   | Resolved scope value                                |
| `identifier` | string   | Resolved identifier value                           |
| `title`      | string   | Resolved title, using README heading when available |
| `status`     | string   | Resolved default status                             |
| `tags`       | []string | Resolved tags after template substitution           |

### Entity: SourceStructureSegmentRole

| Field     | Type   | Purpose                                       |
|-----------|--------|-----------------------------------------------|
| `segment` | string | Segment text from a sample path               |
| `role`    | string | `scope`, `identifier`, `literal`, or `ignore` |

## API Contract

| Method | Endpoint                                           | Request                   | Response                    |
|--------|----------------------------------------------------|---------------------------|-----------------------------|
| `GET`  | `/api/workspaces/{id}/source-structure?directory=` | none                      | `SourceSettingsResult`      |
| `PUT`  | `/api/workspaces/{id}/source-structure?directory=` | `SourceStructureSettings` | `SourceStructureSaveResult` |

`SourceSettingsResult` adds:

| Field       | Type                        | Purpose                                  |
|-------------|-----------------------------|------------------------------------------|
| `proposals` | []`SourceStructureProposal` | Candidate rules from real source paths   |
| `preview`   | []`SourceStructurePreview`  | Preview rows for the current saved draft |

## Proposal Heuristics

| Heuristic                  | Candidate Pattern              | Confidence |
|----------------------------|--------------------------------|------------|
| `folder/feature/item` | `{folder}/feature/{item}` | high       |
| `source/item`         | `{folder}/{item}`         | high       |
| `item-only`          | `{item}`                 | medium     |
| `docs collection`          | source-level fallback card     | low        |

The first version samples directories, ignores hidden folders, limits preview rows, and reads README headings only when the matched card directory contains a README-like Markdown file.

## Design Decisions

| Decision                               | Rationale                                                                        |
|----------------------------------------|----------------------------------------------------------------------------------|
| Add fields to `SourceSettingsResult`   | Existing clients tolerate additive JSON fields and current API paths stay stable |
| Generate previews in `scanner` helpers | The scanner already owns pattern parsing, matching, and title extraction         |
| Keep save validation unchanged         | PM-014 should not weaken source settings safety rules                            |
| Cap preview rows                       | Large docs trees should not make opening the dialog expensive                    |
