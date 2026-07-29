# Documentation Map

Use the folder that matches the document’s purpose. Filenames are lowercase kebab-case and state the subject, not the
document type already expressed by the folder.

| Folder       | Contains                                                                                       | Examples                                          |
|--------------|------------------------------------------------------------------------------------------------|---------------------------------------------------|
| `cloud/`     | Durable Cloud architecture, access-mode, and operational guidance                              | `cloud-modes.md`, `remote-snapshot-operations.md` |
| `release/`   | Procedures for packaging and publishing a Kode Stream release                                  | `release.md`, `homebrew-tap-bootstrap.md`         |
| `runbooks/`  | Repeatable setup, showcase, validation, or operator procedures that are not release publishing | `chrome-extension-showcase.md`                    |
| `storage/`   | Storage architecture, operation, and diagrams                                                  | `storage-architecture.md`                         |
| `workspace/` | Workspace-facing API and configuration documentation                                           | `workspace-import-api.md`                         |
| `specs/`     | Product and technical requirements                                                             | `requirement.md`                                  |

Do not put feature explanations or runtime operations in `release/`. Link to a runbook or durable domain document from a
release procedure when a release requires it.
