# Cloud Agent

Cloud Agent is the local execution component for Cloud mode. It connects outbound to the Cloud public URL and keeps
repository files, Git credentials, terminal sessions, AI CLIs, runtime commands, and verification commands on the user's
machine.

## CLI

```bash
kode-stream agent start --connect "kodestream://connect?token=..." --cloud-url https://kode-stream.example.com --repo /path/to/repo
kode-stream agent status
kode-stream agent doctor --cloud-url https://kode-stream.example.com --repo /path/to/repo
```

`agent start` is the foreground channel process. It parses raw tokens or `kodestream://connect` links, connects outbound
to `/api/agents/channel`, sends heartbeat frames, reads acknowledgements, and can publish local Git workspace metadata
when `--repo` is provided. Full command dispatch is planned after this connection and metadata slice.

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

The handler starts or wakes Cloud Agent. The agent uses the short-lived token for an authenticated outbound WebSocket to
`/api/agents/channel`. Durable reconnect credentials are planned after first-pairing smoke is complete.

## Planned Targets

Windows and Linux packages are planned. They must register the same `kodestream://connect` handler and preserve the
outbound-only network model.

## Smoke

```bash
kode-stream agent doctor --cloud-url https://kode-stream.example.com --repo /path/to/repo
kode-stream agent start --connect "kodestream://connect?token=..." --cloud-url https://kode-stream.example.com --repo /path/to/repo
open "kodestream://connect?token=test" # macOS deep-link registration smoke after packaging
```

The repo smoke validates a Git root locally and publishes only metadata to Cloud. Command-capable file, Git, terminal,
AI, runtime, and verification operations must route through the agent command bridge once that phase is implemented.
