# Documentation Map

`docs/` holds durable reference: what a thing is and why it is that way. Use the folder that matches the subject.
Filenames are lowercase kebab-case and state the subject, not the document type already expressed by the folder.

| Folder          | Contains                                                 | Examples                                        |
|-----------------|----------------------------------------------------------|-------------------------------------------------|
| `domain/`       | Domain knowledge: what the product is and how it behaves | `cloud/`, `storage/`, `terminal/`, `verification/`, `workspace/` |
| `architecture/` | System overview and mode/distribution diagrams           | `overview.md`, `local-mode/`, `cloud-mode/`     |
| `specs/`        | Product and technical requirements                       | `requirement.md`                                |

Start at [Domain](domain/README.md) for what a concept means and how it behaves, or
[Architecture](architecture/README.md) for system and mode diagrams. Operator procedures live under
[`deploy/`](../deploy/README.md), beside the Compose files and launchers they drive. Do not put a procedure in
`docs/`; link to reference from a procedure instead of copying it.

One exception is deliberate: [`development.md`](development.md) sits at this level. It is the build-and-test guide
for contributors, not a runtime or deployment procedure, so it belongs to neither tree's rule.
