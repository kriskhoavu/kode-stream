# Deployment and Operations

Operator procedures. Reference lives in [`docs/`](../docs/README.md) and the [wiki](../wiki/index.md).

| Area     | Procedure                                                                                        | Drives                                          |
|----------|--------------------------------------------------------------------------------------------------|-------------------------------------------------|
| Docker   | [Local mode](docker/local/local-mode/README.md), [Cloud mode](docker/local/cloud-mode/README.md) | `docker/local/<mode>/compose.yaml` and `run.sh` |
| Cloud    | [Cloud deployment](cloud-deployment.md)                                                          | A hosted VM or container                        |
| Homebrew | [Release](homebrew/release.md), [Tap bootstrap](homebrew/homebrew-tap-bootstrap.md)              | `cmd/scripts/distribution/`                     |

`docker/image/` holds one folder per image. `docker/local/` holds the one-machine Compose stacks. Every image builds
from the repository root. The image embeds the UI from `internal/server/frontend/`, so run `npm run build` before you
build an image.
