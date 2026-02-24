-- Drop indexes added for the optimized GetSpanRuns query.
DROP INDEX IF EXISTS idx_spans_executor_run_start;
DROP INDEX IF EXISTS idx_spans_run_dynamic_endtime_status;
DROP INDEX IF EXISTS idx_spans_name_dynamic_span_id;
