# Frontend Design: Embedded AI Terminal

## Overview

Extend the PM-018 launch dialog with external and embedded surfaces. Add an embedded terminal workspace driven by typed WebSocket events.

## State Management

| State              | Owner                   | Behavior                                             |
|--------------------|-------------------------|------------------------------------------------------|
| Launch selection   | PM-018 launch dialog    | Adds `external` or `embedded` surface                |
| Session metadata   | App-level terminal dock | Tracks sessions across item and workspace navigation |
| Terminal transport | Terminal component      | Sends input/resize and renders output frames         |
| Navigation guard   | Item workspace          | Warns before leaving a running session               |

The terminal component uses a maintained terminal emulator library with fit and resize support. Raw provider output is rendered as terminal data, never HTML. The WebSocket URL derives from the current loopback origin and the grant remains in memory only.

## User Experience

- Keep external terminal as the saved default until the user selects embedded mode.
- Show connecting, running, reconnecting, exited, cancelled, and failed states.
- Provide explicit cancel and close actions; closing an active view does not silently leave a process running.
- Preserve bounded scrollback supplied by the terminal component and backend reconnect buffer.
- Restore focus predictably and expose lifecycle status outside the terminal canvas.
- Keep multiple workspace sessions connected and identify each by workspace, item, and provider.
- Support a compact bottom-right minimized session chip plus normal and maximized modes; keep transports mounted and refit the terminal after restoration.
- Use provider-focused window titles, workspace/card subtitles, conventional window controls, and close confirmation instead of a separate cancel action.

## Accessibility

- Terminal controls remain keyboard reachable without stealing provider input shortcuts.
- Provide a keyboard-accessible escape route from terminal focus.
- Announce lifecycle state changes outside the terminal canvas.

## Design Decisions

| Decision                       | Rationale                                                |
|--------------------------------|----------------------------------------------------------|
| Separate terminal lifecycle UI | Terminal output alone cannot communicate app-owned state |
| Keep grants in memory          | Prevents session credentials entering persistent storage |
| App-level multi-session dock   | Prevents navigation from destroying unrelated sessions   |
