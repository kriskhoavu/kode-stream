# PM-031 Performance Scorecard

Measured on Apple M1 Pro, darwin arm64, with `rtk go test -bench 'BenchmarkGin(HealthRoute|AuditEventsRoute|StateRoute|SearchRoute|WorkspaceListRoute|ItemListRoute|ItemDetailRoute)' -benchmem ./internal/server/api`.

| Benchmark                         | Time/op       | Bytes/op | Allocs/op |
|-----------------------------------|---------------|----------|-----------|
| `BenchmarkGinHealthRoute-10`      | 1383 ns/op    | 2066     | 22        |
| `BenchmarkGinAuditEventsRoute-10` | 1268573417 ns | 620024   | 4328      |
| `BenchmarkGinStateRoute-10`       | 3679 ns/op    | 2702     | 25        |
| `BenchmarkGinSearchRoute-10`      | 1279 ns/op    | 1642     | 16        |
| `BenchmarkGinWorkspaceListRoute`  | 2454 ns/op    | 2220     | 19        |
| `BenchmarkGinItemListRoute-10`    | 2575 ns/op    | 2678     | 22        |
| `BenchmarkGinItemDetailRoute-10`  | 5133 ns/op    | 3064     | 23        |

## Notes

- Audit benchmark cost remains dominated by fixture-backed event storage and filtering, not route matching.
- State, search, workspace list, and item read routes are all below 6 us/op in the measured fixture.
- No accepted API contract regressions were identified by `rtk go test ./...` and `rtk npm run typecheck`.
