# Workspace Import API

The workspace import API separates read-only review from confirmed registry mutation. It accepts only absolute local
paths to current-schema `.yaml` or `.yml` registry files.

## Endpoints

| Method | Path                             | Purpose                                      |
|--------|----------------------------------|----------------------------------------------|
| POST   | `/api/system/select-file`        | Select a YAML file through the native dialog |
| GET    | `/api/system/config-paths`       | Return the effective `registryFile` path     |
| POST   | `/api/workspaces/import-preview` | Parse and validate candidates without writes |
| POST   | `/api/workspaces/import`         | Register and scan selected candidate keys    |

## Preview

Request fields:

| Field        | Type   | Requirement                      |
|--------------|--------|----------------------------------|
| `sourcePath` | string | Absolute readable YAML file path |

The response includes canonical `sourcePath`, backend-resolved `destinationPath`, a diagnostic source fingerprint,
ordered candidates, and valid/invalid/duplicate/already-registered counts. Each candidate includes its stable key,
one-based source position, normalized non-secret configuration, selection recommendation, status, and field issues.

File-level rejection applies to relative paths, non-YAML or non-regular files, files over 1 MiB, more than 500 entries,
unknown current-schema fields, YAML aliases, malformed YAML, and multiple documents. Candidate validation errors do not
reject the whole preview.

## Confirmed Import

Request fields:

| Field           | Type     | Requirement                              |
|-----------------|----------|------------------------------------------|
| `sourcePath`    | string   | Same absolute source selected for review |
| `candidateKeys` | string[] | Candidate keys explicitly selected       |

The backend rereads and revalidates the source. It never accepts workspace configuration from the browser. Still-valid
candidates are registered with one atomic effective-registry replacement, mode `0600`, destination-generated IDs and
timestamps, registration mode `existing_workspace`, and `clonePathManaged: false`.

Each selected key returns one outcome:

| Status        | Meaning                                                        |
|---------------|----------------------------------------------------------------|
| `indexed`     | Registration and initial scan succeeded                        |
| `scan_failed` | Registration succeeded; retry the normal Scan action           |
| `skipped`     | Candidate changed, became invalid/duplicate, or already exists |
| `failed`      | Registry persistence failed and registration was not committed |

Scan failures do not roll back valid registrations and do not stop scans for other imported workspaces. API and audit
messages exclude credential values and raw local failure details.
