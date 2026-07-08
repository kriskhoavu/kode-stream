# Backend Design: Workspace Import

## Overview

Workspace owns parsing, candidate validation, batch registration, scan orchestration, and import HTTP contracts. System owns native file selection and effective application paths. The source YAML is read-only; successful candidates are converted to destination-owned registrations and written through the existing registry.

## API Contract

| Method | Endpoint                         | Request                        | Response                          |
|--------|----------------------------------|--------------------------------|-----------------------------------|
| POST   | `/api/system/select-file`        | YAML extension filter          | Selected absolute path            |
| POST   | `/api/workspaces/import-preview` | Source path                    | Import preview                    |
| POST   | `/api/workspaces/import`         | Source path and candidate keys | Per-candidate import results      |
| GET    | `/api/system/config-paths`       | None                           | Includes effective `registryFile` |

The file picker cancellation returns an empty path as a normal response. Preview and import reject relative paths, directories, unsupported extensions, unreadable files, files over 1 MiB, more than 500 entries, YAML aliases, multiple YAML documents, and fields outside the current schema.

## Data Model

### Workspace Import Preview

| Field               | Type                   | Purpose                                          |
|---------------------|------------------------|--------------------------------------------------|
| `sourcePath`        | string                 | Canonical selected file path                     |
| `destinationPath`   | string                 | Effective `Paths.RegistryFile`                   |
| `sourceFingerprint` | string                 | Diagnostic content digest                        |
| `candidates`        | import candidate array | Ordered entries including invalid candidates     |
| `summary`           | counts                 | Valid, invalid, duplicate, and registered totals |

### Workspace Import Candidate

| Field          | Type                | Purpose                                                    |
|----------------|---------------------|------------------------------------------------------------|
| `candidateKey` | string              | Digest of normalized candidate content and source position |
| `position`     | integer             | One-based position for review and errors                   |
| `workspace`    | safe workspace view | Current-schema configuration without destination identity  |
| `status`       | enum                | `valid`, `invalid`, `duplicate`, or `already_registered`   |
| `issues`       | issue array         | Field, code, and actionable message                        |
| `selected`     | boolean             | Default selection recommendation                           |

### Workspace Import Result

| Field          | Type        | Purpose                                          |
|----------------|-------------|--------------------------------------------------|
| `candidateKey` | string      | Correlates result with preview                   |
| `workspace`    | workspace   | Destination registration when created            |
| `status`       | enum        | `indexed`, `scan_failed`, `skipped`, or `failed` |
| `scan`         | scan result | Item and warning counts when indexing succeeds   |
| `message`      | string      | Safe failure or skip explanation                 |

## Parsing and Validation

Use `yaml.Decoder.KnownFields(true)` against a dedicated import DTO. The DTO accepts current `WorkspaceConfig` fields so a normal Kode Stream registry can be reused, but ignores source identity fields only after successful strict decoding. `id`, `createdAt`, `lastScannedAt`, `lastSelectedBranch`, and `clonePathManaged` never control destination state.

For each entry:

1. Normalize strings and paths without modifying the source file.
2. Require name, path, baseline branch, and at least one source.
3. Resolve the canonical Git root and validate the baseline branch.
4. Validate every source as an existing relative directory inside the root.
5. Reuse Jira and Knowledge validation.
6. Detect repeated paths inside the source and paths already registered.
7. Build a destination `WorkspaceInput` with mode `existing_workspace`.

The preview returns candidate-level validation errors together. A file-level structural or safety failure rejects the request.

## Registration Semantics

Add `existing_workspace` to `WorkspaceRegistrationMode`. It behaves like a local path during validation, records optional origin metadata only when safe, and always sets `clonePathManaged` to false. Deleting an imported registration removes app data and index entries but never deletes the workspace directory.

The registry receives one batch-create operation:

- Lock after validation and recheck destination path conflicts under the lock.
- Generate destination IDs and `createdAt` values.
- Append all accepted registrations in source order.
- Write a temporary file in the destination directory with mode `0600`.
- Flush, close, and atomically rename it over `workspaces.yaml`.
- Do not mutate in-memory records if persistence fails.

No registry entry is written when every selection is skipped or invalid.

## Scan and Index Orchestration

After the registry batch succeeds, call the existing workspace scan service once per imported ID. Continue after failures. Scans update `item-index.yaml`, timestamps, state version, and existing audit behavior. The response reports each workspace independently so the frontend can offer the normal Scan action for failures.

Registration is not rolled back after a scan failure because the source configuration passed validation and a later scan can recover. Registry persistence failure prevents every scan.

## Concurrency and Integrity

- Preview is read-only and does not reserve candidates.
- Import never trusts candidate configuration returned by the browser.
- Import rereads the same absolute path and recalculates candidate keys.
- Candidate keys bind selection to normalized content and source position.
- Destination duplicate checks run again while the registry is locked.
- Source-file changes produce skipped results instead of importing changed entries.
- The batch write uses atomic replacement; scans remain independent derived-data writes.

## Security and Privacy

- Accept only explicitly selected or entered local paths.
- Apply size and entry limits before expensive Git validation.
- Never follow a YAML-provided path for writing.
- Never expose environment-variable values or other credentials.
- Return Jira token variable names because they are configuration, not token values.
- Sanitize filesystem and Git errors for API responses and audit details.
- Do not execute Git network operations or clone commands during preview or import.

## Tests

| Area       | Coverage                                                                  |
|------------|---------------------------------------------------------------------------|
| Parser     | Strict schema, malformed YAML, aliases, documents, size and entry limits  |
| Validation | Git root, branch, sources, Jira, Knowledge, duplicates, current registry  |
| Registry   | Atomic batch, write failure, concurrent duplicate, destination identities |
| Service    | Revalidation, changed keys, partial scans, no-selection behavior          |
| API        | Picker cancel, preview errors, result status, effective registry path     |
| Safety     | Imported deletion never removes workspace directory                       |
