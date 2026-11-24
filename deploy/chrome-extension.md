# Chrome Extension Showcase

The unpacked Chrome extension bundles the React UI and calls a local Kode Stream server for files, Git, dialogs and
guarded writes. Architecture: [Chrome extension](../docs/architecture/chrome-extension/README.md).

## Build And Load

1. Run `npm run build:extension`. The output is `dist/chrome-extension`, which Git ignores.
2. Start the server: `kode-stream serve -port 4317`.
3. Open `chrome://extensions` and enable Developer Mode.
4. Choose **Load unpacked** and select `dist/chrome-extension`.
5. Click the extension action to open the UI in a tab.

The extension calls `http://127.0.0.1:4317`. For another port, set `localStorage.kodeStreamApiOrigin` in the extension
page's console and reload.

## Acceptance

| Scenario       | Steps                                                  | Expected result                                       |
|----------------|--------------------------------------------------------|-------------------------------------------------------|
| Server running | Open the extension while the server runs               | Workspaces load from the local API                    |
| Server stopped | Stop the server and reload                             | The server-unavailable state shows with **Retry**     |
| Browse files   | Open a workspace and use the explorer                  | Tree, preview, content search and file loads work     |
| Edit files     | Edit and save a Markdown file in a source              | Stale-content and guarded-write checks still apply    |
| Git status     | Refresh Git status after a save                        | The file shows as modified                            |
| Branches       | Switch to a clean branch                               | The branch list and switch work                       |

## Limits

No `file://` access, no downloads, no native messaging (the user starts the server), no embedded terminal (it needs a
separate security review of its WebSocket assumptions), and no Web Store distribution.

## Troubleshooting

| Problem                  | Check                                                                                  |
|--------------------------|----------------------------------------------------------------------------------------|
| Extension shows offline  | The server runs on the expected port and `http://127.0.0.1:4317/` loads in a tab       |
| API requests blocked     | The server runs in Local mode. Only Local mode allows `chrome-extension://` origins     |
| Blank page after rebuild | Reload the extension in `chrome://extensions`                                          |
| Terminal controls absent | Expected. Use the normal localhost UI for the embedded terminal                        |
