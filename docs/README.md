# Documentation Map

Use the folder that matches the document’s purpose. Filenames are lowercase kebab-case and state the subject, not the
document type already expressed by the folder.

| Folder          | Contains                                               | Examples                                        |
|-----------------|--------------------------------------------------------|-------------------------------------------------|
| `domain/`       | Domain knowledge: what the product is and how it behaves | `cloud/`, `storage/`, `terminal/`, `verification/`, `workspace/` |
| `architecture/` | System overview and mode/distribution diagrams         | `ARCHITECTURE.md`, `local-mode/`, `cloud-mode/` |
| `specs/`        | Product and technical requirements                     | `requirement.md`                                |

Start at [Domain](domain/README.md) for what a concept means and how it behaves, or
[Architecture](architecture/README.md) for system and mode diagrams. Runbooks live at the repository root
under [`deploy/`](../deploy/README.md). Do not put runtime procedures in `docs/`; link to durable reference
documentation from a runbook when needed.
