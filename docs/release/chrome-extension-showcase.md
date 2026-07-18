# Chrome Extension Local Showcase

Kode Stream can be built as an unpacked Chrome extension for Local mode showcases. The extension bundles the React UI
and calls the local Kode Stream server for workspace files, Git operations, system dialogs, and guarded writes.

## Build

From the repository root:

```bash
npm run build:extension
```

The unpacked extension is emitted to `dist/chrome-extension`. The directory is ignored by Git and should be rebuilt
locally when needed.

## Load In Chrome

1. Start the local server:

```bash
kode-stream serve -port 4317
```

2. Open `chrome://extensions`.
3. Enable Developer Mode.
4. Select Load unpacked.
5. Choose `dist/chrome-extension`.
6. Click the Kode Stream extension action to open the bundled UI in a Chrome tab.

The extension defaults to `http://127.0.0.1:4317` for API calls. To test another port, open the extension page console,
set `localStorage.kodeStreamApiOrigin` to the desired origin, and reload the extension page.

## Acceptance Scenarios

| Scenario       | Steps                                                               | Expected Result                                                            |
|----------------|---------------------------------------------------------------------|----------------------------------------------------------------------------|
| Server health  | Open the extension while `kode-stream serve -port 4317` is running. | The app loads workspaces from the local API.                               |
| Server stopped | Stop the local server and reload the extension page.                | The local server unavailable state is shown with a Retry action.           |
| Browse files   | Open a registered workspace and use Workstream Explorer.            | The tree, Markdown preview, content search, and file load calls work.      |
| Edit files     | Edit and save a Markdown file in a workspace source.                | Existing stale-content and guarded write behavior is preserved.            |
| Git status     | Refresh Git status after a save.                                    | Dirty state reflects the modified file.                                    |
| Branches       | Open the branch selector and switch to a clean branch.              | Branch list and clean branch switch work through the existing Git adapter. |

## V1 Limits

| Limit                | Reason                                                                                          |
|----------------------|-------------------------------------------------------------------------------------------------|
| No file URL access   | Workspace files are accessed through the local Kode Stream API, not Chrome `file://` access.    |
| No downloads access  | The showcase does not create downloads from the extension.                                      |
| No native messaging  | The user starts `kode-stream serve` explicitly for v1.                                          |
| No embedded terminal | Embedded AI terminal uses same-origin WebSocket assumptions and needs a separate security pass. |
| No store publishing  | PM-034 targets manual local import only.                                                        |

## Troubleshooting

| Problem                  | Check                                                                                            |
|--------------------------|--------------------------------------------------------------------------------------------------|
| Extension shows offline  | Confirm `kode-stream serve -port 4317` is running and `/api/health` responds in a browser tab.   |
| Wrong local port         | Set `localStorage.kodeStreamApiOrigin` in the extension page and reload.                         |
| API requests blocked     | Confirm the server is a PM-034 build with local-mode Chrome extension CORS support.              |
| Blank page after rebuild | Reload the extension from `chrome://extensions` after running `npm run build:extension`.         |
| Terminal controls absent | This is expected in extension mode for v1. Use the normal localhost UI for embedded AI terminal. |
