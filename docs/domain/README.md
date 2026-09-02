# Domain

`docs/domain/` holds Kode Stream's domain knowledge: what each part of the product is and how it behaves. A folder
here answers what a concept means and what guarantees it makes — not how the code is arranged, which belongs to
[`architecture/`](../architecture/README.md), and not the steps to operate it, which belong to
[`deploy/`](../../deploy/README.md).

| Folder                             | Contains                                                       |
|------------------------------------|----------------------------------------------------------------|
| [`cloud/`](cloud/cloud-modes.md)   | What Cloud is, its two access modes, and the Cloud Agent       |
| [`storage/`](storage/storage-architecture.md) | App-owned versus repository-owned state, and the storage options |
| [`terminal/`](terminal/canvas-sessions.md)    | Terminal session ownership and lifecycle                       |
| [`verification/`](verification/canvas-freshness.md) | How command outcome and repository freshness are reported separately |
| [`workspace/`](workspace/workspace-import-api.md)   | The workspace-facing import API and its configuration          |

Filenames are lowercase kebab-case and state the subject, not the document type already expressed by the folder.
