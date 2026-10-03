# Scenario 1: Card Agent Journey

1. Open the Workstream board. Hover a card with no live session; the agent launcher appears.
2. Launch a Claude session from the launcher. The drawer opens on the Agents tab with the terminal in the main pane.
3. Type a follow-up prompt in the terminal; the agent answers.
4. Close the drawer. The session appears in the terminal dock and keeps running; the card shows its chip.
5. Open the Canvas: the session is drawn next to its plan. Open the Sessions page: it is listed.
6. Stop the session. The chip and the Canvas node disappear; the Sessions page still lists it.
7. In a separate terminal, run `claude` in the workspace and mention the plan's path. Reload the board: the card
   shows a hollow chip. Open it, choose **Continue here**, and keep prompting in the embedded terminal.
