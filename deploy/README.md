# Deployment and Operations

Each entry is an ordered operator procedure. Procedures link to durable reference material in
[`docs/`](../docs/README.md) but do not replace domain, architecture, API, or storage documentation.

| Area     | Procedure                                                                                         | Drives                                       |
|----------|---------------------------------------------------------------------------------------------------|----------------------------------------------|
| Docker   | [Cloud mode](docker/local/cloud-mode/README.md), [Local mode](docker/local/local-mode/README.md)  | `docker/local/<mode>/compose.yaml` and `run.sh` |
| Homebrew | [Release procedure](homebrew/release.md), [Tap bootstrap](homebrew/homebrew-tap-bootstrap.md)     | `cmd/scripts/distribution/`                  |
| Showcase | [Chrome extension showcase](chrome-extension.md)                                                  | The unpacked extension build                 |

`docker/` splits the two things Docker is used for. `docker/image/` holds one folder per image — its Dockerfile plus
whatever builds or verifies it — and `docker/local/` holds the Compose topologies that run Kode Stream on one
machine. Both modes are one-machine topologies: `local-mode` is the single-user local server, and `cloud-mode` is a
local stack that mirrors the VM deployment shape.

Application source stays in `cmd/`, `internal/`, and `web/`. Every image uses the repository root as its build
context, so a recipe under `docker/image/` refers to its source by repository-relative path rather than sitting
beside it. Release automation is invoked by CI rather than by an operator in this tree, so it stays under
`cmd/scripts/`.
