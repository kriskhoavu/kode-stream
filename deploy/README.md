# Deployment and Operations

Each entry is an operator procedure. Procedures link to reference in [`docs/`](../docs/README.md) and the
[wiki](../wiki/index.md) instead of copying it.

| Area     | Procedure                                                                                        | Drives                                          |
|----------|--------------------------------------------------------------------------------------------------|-------------------------------------------------|
| Docker   | [Local mode](docker/local/local-mode/README.md), [Cloud mode](docker/local/cloud-mode/README.md) | `docker/local/<mode>/compose.yaml` and `run.sh` |
| Cloud    | [Cloud deployment](cloud-deployment.md)                                                          | A hosted VM or container                        |
| Homebrew | [Release](homebrew/release.md), [Tap bootstrap](homebrew/homebrew-tap-bootstrap.md)              | `cmd/scripts/distribution/`                     |
| Showcase | [Chrome extension](chrome-extension.md)                                                          | The unpacked extension build                    |

`docker/image/` holds one folder per image. `docker/local/` holds the one-machine Compose stacks: `local-mode` is the
single-user server, and `cloud-mode` mirrors the VM deployment. Every image builds from the repository root, so each
recipe names its source by repository-relative path. CI runs the release automation, so it stays in `cmd/scripts/`.
