# Documentation Map

Use the folder that matches the document’s purpose. Filenames are lowercase kebab-case and state the subject, not the
document type already expressed by the folder.

| Folder          | Contains                                             | Examples                                          |
|-----------------|------------------------------------------------------|---------------------------------------------------|
| `architecture/` | System overview and mode/distribution diagrams       | `ARCHITECTURE.md`, `local-mode/`, `cloud-mode/`   |
| `cloud/`        | Durable Cloud operational guidance                   | `cloud-modes.md`, `remote-snapshot-operations.md` |
| `storage/`      | Storage option operation and recovery guidance       | `storage-architecture.md`                         |
| `terminal/`     | Terminal session ownership and lifecycle guidance    | `canvas-sessions.md`                              |
| `verification/` | Verification result and freshness semantics          | `canvas-freshness.md`                             |
| `workspace/`    | Workspace-facing API and configuration documentation | `workspace-import-api.md`                         |
| `specs/`        | Product and technical requirements                   | `requirement.md`                                  |

Start at [Architecture](architecture/README.md) for system and mode diagrams. Runbooks live at the repository root
under [`runbooks/`](../runbooks/README.md). Do not put runtime procedures in `docs/`; link to durable reference
documentation from a runbook when needed.
