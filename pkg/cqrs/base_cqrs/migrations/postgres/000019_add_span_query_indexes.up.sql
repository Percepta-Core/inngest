-- Indexes to support the optimized GetSpanRuns query.
--
-- For root span lookup with ORDER BY start_time DESC, run_id.
-- Partial index covers the common filters, enabling index scan with LIMIT pushdown.
-- This is the critical index: turns a 84s sequential scan into a <1ms index scan.
-- Note: the filter matches newSpanRunsQueryBuilder which excludes 'Skipped' (not 'Cancelled').
CREATE INDEX IF NOT EXISTS idx_spans_executor_run_start
    ON spans(start_time DESC, run_id)
    WHERE name = 'executor.run' AND debug_run_id IS NULL AND (status IS NULL OR status <> 'Skipped');

-- For correlated subqueries: MAX(end_time) and latest status per (run_id, dynamic_span_id).
-- The INCLUDE(status) enables index-only scans (0 heap fetches) for both lookups.
CREATE INDEX IF NOT EXISTS idx_spans_run_dynamic_endtime_status
    ON spans(run_id, dynamic_span_id, end_time DESC NULLS LAST) INCLUDE (status);

-- For the name-based lookup of root spans (used by SQLite path and as fallback).
CREATE INDEX IF NOT EXISTS idx_spans_name_dynamic_span_id
    ON spans(name, dynamic_span_id);
