# Scenario Overview: PM-028

## Scenario 1: Switch item detail branches without checkout

1. Open Workstream and select an item.
2. Open the item detail page.
3. Use the shared branch picker to select another branch.
4. If the branch contains the item, show that branch as a snapshot.
5. If the branch does not contain the item, keep the selected branch in the picker and show an empty snapshot state.

## Scenario 2: Return from snapshot to the current checkout branch

1. Open the branch picker while viewing a snapshot branch.
2. See `main`, `master`, and the current checkout branch pinned at the top of the list.
3. Select the current checkout branch.
4. Return to working-tree content without browsing the rest of the branch list.

## Scenario 3: Browse item files through the embedded Explorer shell

1. Open an item detail page on a working-tree item.
2. Use the embedded Explorer to switch between Plan files and Explorer modes.
3. Search, open, edit, create, rename, and inspect files inside the item workspace context.

## Scenario 4: Open arbitrary Knowledge files

1. Open Knowledge and read a structured page.
2. Use the existing file deep-link action.
3. The app opens the standalone Explorer route for that workspace path.
4. This flow remains a blocker for deleting the standalone Explorer route.
