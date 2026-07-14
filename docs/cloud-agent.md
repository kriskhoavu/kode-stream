# Cloud Agent

Cloud Agent is the local execution component for Cloud mode. It connects outbound to the Cloud public URL and keeps
repository files, Git credentials, terminal sessions, AI CLIs, runtime commands, and verification commands on the user's
machine.

## CLI

```bash
kode-stream agent start --connect "kodestream://connect?token=..." --cloud-url https://kode-stream.example.com
kode-stream agent status
kode-stream agent doctor --cloud-url https://kode-stream.example.com --repo /path/to/repo
```

`agent start` is the future long-running channel process. The current foundation validates CLI parsing and gives the
packaging path a stable command surface.

## macOS Packaging

Homebrew is the first supported packaging path:

```bash
brew update
brew tap kriskhoavu/homebrew-tap
brew install kode-stream
kode-stream agent doctor --cloud-url https://kode-stream.example.com --repo /path/to/repo
```

The formula should install the same `kode-stream` binary and expose `kode-stream agent ...`. Deep-link registration for
`kodestream://connect` should point to `kode-stream agent start --connect <url>`.

## Deep Link

Cloud UI launches:

```text
kodestream://connect?token=<short-lived-token>
```

The handler starts or wakes Cloud Agent. The agent exchanges the short-lived token for an authenticated outbound
WebSocket to `/api/agents/channel`.

## Planned Targets

Windows and Linux packages are planned. They must register the same `kodestream://connect` handler and preserve the
outbound-only network model.

## Smoke

```bash
kode-stream agent doctor --cloud-url https://kode-stream.example.com --repo /path/to/repo
open "kodestream://connect?token=test" # macOS deep-link registration smoke after packaging
```

The repo scan smoke should validate a Git root locally and publish only metadata to Cloud.
