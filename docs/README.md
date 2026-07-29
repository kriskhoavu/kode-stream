# Documentation Map

Use the folder that matches the document’s purpose. Filenames are lowercase kebab-case and state the subject, not the
document type already expressed by the folder.

| Folder       | Contains                                                          | Examples                                          |
|--------------|-------------------------------------------------------------------|---------------------------------------------------|
| `cloud/`     | Durable Cloud architecture, access-mode, and operational guidance | `cloud-modes.md`, `remote-snapshot-operations.md` |
| `storage/`   | Storage architecture, operation, and diagrams                     | `storage-architecture.md`                         |
| `workspace/` | Workspace-facing API and configuration documentation              | `workspace-import-api.md`                         |
| `specs/`     | Product and technical requirements                                | `requirement.md`                                  |

Runbooks live at the repository root under [`runbooks/`](../runbooks/README.md). Do not put feature explanations or
runtime procedures in `docs/`; link to durable reference documentation from a runbook when needed.
