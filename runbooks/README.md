# Runbooks

Runbooks are step-by-step operational procedures. They link to durable reference material in `docs/` but do not replace
architecture, domain, API, or storage documentation.

| Area     | Runbooks                                                                                      |
|----------|-----------------------------------------------------------------------------------------------|
| Docker   | [Cloud mode](docker/cloud-mode/README.md), [Local mode](docker/local-mode/README.md)          |
| Homebrew | [Release procedure](homebrew/release.md), [Tap bootstrap](homebrew/homebrew-tap-bootstrap.md) |
| Showcase | [Chrome extension showcase](chrome-extension-showcase.md)                                     |

Docker Compose files and launchers are colocated with their Docker runbooks; release automation scripts remain under `cmd/scripts/`.
