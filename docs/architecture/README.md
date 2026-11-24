# Architecture

Start with the overview, then read the page for your runtime.

| Page                                                  | Covers                                                                     |
|-------------------------------------------------------|----------------------------------------------------------------------------|
| [System overview](overview.md)                        | Components, packages, data ownership and how the modes fit together        |
| [Local mode](local-mode/README.md)                    | The single-user local server and its `datadir` or SQLite storage           |
| [Cloud mode](cloud-mode/README.md)                    | The hosted control plane, Agent-Backed and Remote Snapshot workspaces      |
| [Chrome extension](chrome-extension/README.md)        | The unpacked extension and its local-server boundary                       |
| [Deployment adapters](deployment-adapters.md)         | Runtime policy, storage providers, Cloud workspace access, extension origin |
| [Engineering harness](../../wiki/platform/harness.md) | Scouts, the ranked queue, coding CLIs in worktrees, approvals              |

Each README renders its main Mermaid diagram inline. The `.mmd` file beside it is the source.
