# Performance Fix: Self-Hosted UI Runs List

## Problem

On Inngest v1.17.1, the self-hosted UI's function runs list page was unusable — page loads took **12-50+ seconds** on a production-scale database (~88K trace runs, ~1.2M spans). The root cause was a combination of issues across four layers of the stack.

## Environment

- **Database**: PostgreSQL (RDS) with ~1.2M rows in `spans` table, ~88K trace runs
- **Deployment**: EKS (Kubernetes), 2 replicas
- **Version**: Inngest v1.17.1 (self-hosted)

## Root Cause Analysis

### Layer 1: Missing Database Indexes

The `spans` table had no indexes optimized for the runs list query pattern. The main query performed a **sequential scan over 1.2M rows** (84+ seconds) instead of using an index scan.

**Fix**: Added three PostgreSQL indexes:

| Index | Purpose | Impact |
|-------|---------|--------|
| `idx_spans_executor_run_start` | Partial index on root spans (`name='executor.run'`) ordered by `start_time DESC` | Enables index scan with LIMIT pushdown — **84s → <10ms** |
| `idx_spans_run_dynamic_endtime_status` | Covering index with `INCLUDE(status)` for correlated status subquery | Index-only scans (0 heap fetches) for status lookups |
| `idx_spans_name_dynamic_span_id` | For name-based root span lookups | Supports inner subquery |

### Layer 2: Poorly Written Query (ORM-Generated)

The original `GetSpanRuns` query used a `GROUP BY` on 6 columns with a correlated subquery for status that executed once per group. With 88K runs, this meant 88K subqueries on every page load.

**Fix**: Rewrote to "root-spans-first" approach:
- Query root spans (`WHERE name='executor.run'`) directly with `LIMIT`
- Correlated subqueries only execute for the result set (100 rows), not the full table
- Eliminated the `GROUP BY` entirely for PostgreSQL path

Also added a PostgreSQL-optimized `GetTraceRunsCount` using `SELECT COUNT(*)` instead of fetching all rows and counting in Go.

### Layer 3: GQL Requesting Unnecessary Subfields

The UI's GraphQL query requests `app { externalID name }` and `function { name slug }` as sub-objects on each run node. These trigger separate field resolvers for each of the 100 rows (200 additional resolver calls per page).

**Impact**: While the fields are needed for display, the resolver pattern was the bottleneck (see Layer 4).

### Layer 4: N+1 Queries from Concurrent Field Resolution

gqlgen resolves fields **concurrently** — all 100 `App()` and 100 `Function()` field resolvers fire simultaneously. A simple mutex-based cache doesn't help because all goroutines start before the first DB call completes, resulting in 100 duplicate DB calls.

**Fix**: Implemented a **singleflight per-request cache** (`LookupCache`):
- Uses channels for deduplication: first goroutine executes the DB call, all others block on a channel
- Only **one DB call per unique key** regardless of concurrency
- Cache lives for the duration of a single HTTP request (no stale data)
- Injected via the existing DataLoader middleware

## Performance Results

Measured via `[perf]` structured timing logs on the deployed patched image:

### Before (v1.17.1 stock)

| Operation | Duration |
|-----------|----------|
| `GetTraceRuns` (main query) | **84,000+ ms** |
| App field resolver (×100) | **0-1,198 ms each** (N+1) |
| Function field resolver (×100) | **Similar** (N+1) |
| **Total page load** | **12-50+ seconds** |

### After (patched)

| Operation | Duration |
|-----------|----------|
| `GetTraceRuns` (main query) | **3-7 ms** |
| `GetEventsByInternalIDs` | **0 ms** |
| App field resolver (×100) | **0 ms each** (singleflight cached) |
| Function field resolver (×100) | **5-6 ms each** (singleflight cached) |
| Runs resolver total | **3-7 ms** |
| **Total page load** | **< 100 ms** |

### Improvement: **~1000x faster**

## Files Changed

| File | Change |
|------|--------|
| `pkg/cqrs/base_cqrs/cqrs.go` | Root-spans-first query rewrite, optimized count |
| `pkg/cqrs/base_cqrs/migrations/postgres/000019_*.sql` | New indexes (up + down) |
| `pkg/coreapi/graph/loaders/cache.go` | New singleflight LookupCache |
| `pkg/coreapi/graph/loaders/loader.go` | Cache injection in middleware |
| `pkg/coreapi/graph/resolvers/function_run_v2.resolver.go` | App/Function resolvers use cache |
| `pkg/coreapi/graph/resolvers/runs_v2.go` | Perf logging, count optimization |
| `Dockerfile` | Multi-stage build with embedded UI |
