# PM-030 Performance Scorecard

## Routes

| Route               | Baseline Source    | Migrated Source                | Notes                                                       |
|---------------------|--------------------|--------------------------------|-------------------------------------------------------------|
| `/api/health`       | `HealthController` | `BenchmarkGinHealthRoute`      | Gin route is direct and has parity coverage.                |
| `/api/audit-events` | `AuditController`  | `BenchmarkGinAuditEventsRoute` | Gin route uses cached audit reader with write invalidation. |

## Baseline

Measured before Gin migration on Apple M1 Pro, darwin arm64:

| Benchmark                           | Time/op          | Bytes/op | Allocs/op |
|-------------------------------------|------------------|----------|-----------|
| `BenchmarkHealthController-10`      | 795.3 ns/op      | 1424     | 14        |
| `BenchmarkAuditControllerEvents-10` | 1348138458 ns/op | 331344   | 2263      |

## Migrated Measurement Command

`rtk go test -bench 'BenchmarkGin(HealthRoute|AuditEventsRoute)' -benchmem ./internal/server/api`

## Migrated Result

Measured after Gin migration on Apple M1 Pro, darwin arm64:

| Benchmark                         | Time/op          | Bytes/op | Allocs/op |
|-----------------------------------|------------------|----------|-----------|
| `BenchmarkGinHealthRoute-10`      | 1358 ns/op       | 2066     | 22        |
| `BenchmarkGinAuditEventsRoute-10` | 1543495125 ns/op | 684344   | 4425      |

## Regression Notes

| Route               | Observation                                                                                                                | Follow-Up                                                                   |
|---------------------|----------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------|
| `/api/health`       | Gin transport adds small fixed overhead versus direct `ServeMux` controller dispatch.                                      | Accept for middleware consistency; no optimization needed.                  |
| `/api/audit-events` | Benchmark remains dominated by audit repository file reads and fixture setup; cache benefits require warm-hit measurement. | Add a dedicated warm-cache benchmark before expanding cache to other reads. |

## Cleanup Status

| Route               | Gin Parity | Legacy API Mux Registration | Cleanup Status |
|---------------------|------------|-----------------------------|----------------|
| `/api/health`       | yes        | removed                     | complete       |
| `/api/audit-events` | yes        | removed                     | complete       |
