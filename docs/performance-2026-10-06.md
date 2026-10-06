# Discovery query measurement — 2026-10-06

An isolated PostgreSQL 16 database was seeded with 20,000 events and 20,000
RSVPs. The opt-in Go test captures actual repository SQL and runs
`EXPLAIN (ANALYZE, BUFFERS)`. These single-run synthetic measurements are
comparative evidence, not production latency guarantees.

| Query | Before 0017 | After 0017 |
|---|---:|---:|
| Public discovery | 7.994 ms | 0.130 ms |
| City filter | 1.555 ms | 1.496 ms |
| Broad search | 8.356 ms | 0.113 ms |
| Cursor continuation | 7.967 ms | 0.114 ms |
| Trending | 16.359 ms | 16.235 ms |

Migration 0017 adds `(starts_at, id)` restricted to published public events.
Discovery and cursor continuation previously scanned approximately 14,000
public rows and sorted for the first page; the new index returns the requested
21 rows in order. City filtering and trending retain their existing plans.
Search selectivity and real data distribution can change these results.

Reproduce only against a disposable, migrated database:

```bash
cd backend
BENCHMARK_DATABASE_URL=postgres://meetus:meetus@localhost:55432/meetus?sslmode=disable \
  go test -count=1 -run TestDiscoveryQueryPlans -v ./internal/event
```

The test seeds and removes its own fixture rows, and runs ANALYZE. Do not point
it at production. Keep `-count=1` to prevent cached test output. Compare separate
disposable databases migrated through 0016 and 0017; never roll back a live
migration solely for benchmarking.
