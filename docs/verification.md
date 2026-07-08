# PM-001 Verification And Release Notes

## Local Build Commands

- Backend tests: `go test ./...`
- Frontend typecheck: `npm run typecheck`
- Frontend tests: `npm test`
- Frontend production build: `npm run build`
- Local binary build: `go build ./cmd/kode-stream`
- Run app: `./kode-stream serve -port 4317`

## Embedded AI Terminal Checklist

- Start workspace-only and selected-card sessions with a disposable provider command.
- Verify input, output, resize, cancellation, reconnect, exit status, and external-launch fallback.
- Confirm disconnect grace and `Ctrl+C` server shutdown terminate provider process groups.
- Inspect console and `audit-log.jsonl`; neither the channel grant nor terminal content may appear.
- Run the lifecycle suite on macOS or another supported Unix development host.

## Playwright Acceptance Checklist

- Register this repository with `plans` as the Plan Directory.
- Run Scan and confirm PM-001 appears in the Workstream board view.
- Filter by repository, branch, status, and text.
- Open PM-001 and confirm file tree, raw Markdown, preview, metadata, and diff tabs.
- Capture desktop screenshot at 1536 x 1024.
- Capture mobile screenshot at 390 x 844.
- Confirm no save, commit, push, pull, branch switch, or file write action is enabled.

## Repository Safety Check

After scan, run:

```bash
git status --short
```

Managed repositories must not show new changes caused by Kode Stream scanning.

## Existing Workspace Import Checklist

- Preview a current-schema `workspaces.yaml` and verify the effective destination path is backend-resolved.
- Confirm invalid, duplicate, and already-registered candidates remain visible but cannot be selected.
- Change the source path and verify stale candidates disappear.
- Cancel the native file picker and verify the current manual path remains unchanged.
- Confirm an import and verify indexed, scan-failed, skipped, and failed results are distinguishable.
- Verify preview writes no registry or index data and imported workspace deletion leaves its directory intact.
- Repeat the dialog pass at desktop width and at 390 px width using keyboard-only selection and confirmation.

## Release Notes For Future Homebrew Packaging

- Package the Go binary after `npm run build`.
- The binary embeds the frontend assets.
- The app stores registry and cache files in the OS user config directory.
- The default local URL is `http://127.0.0.1:4317`.
