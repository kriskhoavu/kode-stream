# PM-003 Baseline

Captured before implementation refactors.

| Check               | Command                 | Result                         |
|---------------------|-------------------------|--------------------------------|
| Backend tests       | `rtk go test ./...`     | 38 tests passed in 13 packages |
| TypeScript          | `rtk npm run typecheck` | Passed                         |
| Frontend tests      | `rtk npm test -- --run` | 16 tests passed in 4 files     |
| Frontend production | `rtk npm run build`     | Passed                         |
