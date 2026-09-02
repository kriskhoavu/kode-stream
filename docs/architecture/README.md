# Architecture Documentation

Kode Stream separates its system architecture from operational guidance. Start with the overview, then choose the
runtime or distribution that matches the way Kode Stream is being used.

| Architecture                                   | Use it to understand                                                                      | Primary Mermaid diagram                                   |
|------------------------------------------------|-------------------------------------------------------------------------------------------|-----------------------------------------------------------|
| [System overview](ARCHITECTURE.md)             | Shared components, ownership, and how the modes fit together                              | [Deployment model](ARCHITECTURE.md#deployment-model)      |
| [Local mode](local-mode/README.md)             | A single-user local server, local repositories, and the `datadir` or SQLite option        | [Local mode](local-mode/local-mode.mmd)                   |
| [Cloud mode](cloud-mode/README.md)             | Hosted control plane, Agent-Backed workspaces, and Agentless Remote Snapshot workspaces   | [Cloud mode](cloud-mode/cloud-mode.mmd)                   |
| [Chrome extension](chrome-extension/README.md) | The unpacked extension distribution and its local-server boundary                         | [Chrome extension](chrome-extension/chrome-extension.mmd) |
| [Deployment adapters](deployment-adapters.md)  | Runtime policy, storage providers, Cloud workspace access, and extension API-origin seams | [Class diagrams](deployment-adapters.md)                  |

Each README renders its primary Mermaid diagram inline; the adjacent `.mmd` file is the single source for every diagram.
Detailed storage operation and deployment procedures stay in [Storage](../domain/storage/storage-architecture.md), [Cloud](../domain/cloud/cloud-modes.md), and
[deployment and operations](../../deploy/README.md).
