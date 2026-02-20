-- Test get_top_queries tool (resources mode)
-- Run: psql -U demo -d northwind -f test_get_top_queries.sql
--
-- Wraps the tool code in BEGIN/COMMIT to preserve the
-- transaction-local set_config result for retrieval.
-- Requires: pg_stat_statements extension

BEGIN;

DO $mcp_custom_tool$
<<mcp_block>>
DECLARE
    args jsonb := '{"sort_by":"resources","limit":5,"min_calls":1}'::jsonb;
    result jsonb;
    sort_mode text;
    query_limit integer;
    min_call_count integer;
    pg_ver integer;
    has_ext boolean;
    total_col text;
    mean_col text;
    wal_expr text;
    wal_frac_expr text;
    q text;
    rec RECORD;
BEGIN
    sort_mode := coalesce(args->>'sort_by', 'resources');
    query_limit := coalesce((args->>'limit')::integer, 10);
    min_call_count := coalesce((args->>'min_calls')::integer, 5);
    result := '[]'::jsonb;

    SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements') INTO has_ext;

    IF NOT has_ext THEN
        result := jsonb_build_object('error', 'pg_stat_statements is not installed',
            'install', 'CREATE EXTENSION pg_stat_statements;',
            'note', 'Tracks execution statistics for all SQL statements.');
        PERFORM set_config('mcp.tool_result', result::text, true);
        RETURN;
    END IF;

    pg_ver := current_setting('server_version_num')::integer / 10000;
    IF pg_ver >= 13 THEN
        total_col := 'total_exec_time'; mean_col := 'mean_exec_time';
        wal_expr := 'wal_bytes';
        wal_frac_expr := 'wal_bytes / NULLIF(SUM(wal_bytes) OVER (), 0)';
    ELSE
        total_col := 'total_time'; mean_col := 'mean_time';
        wal_expr := '0'; wal_frac_expr := '0';
    END IF;

    q := 'WITH fracs AS ('
        || 'SELECT query, calls, rows, '
        || total_col || ' AS total_ms, '
        || mean_col || ' AS mean_ms, '
        || 'shared_blks_hit, shared_blks_read, shared_blks_dirtied, '
        || wal_expr || ' AS wal, '
        || total_col || ' / NULLIF(SUM(' || total_col || ') OVER (), 0) AS time_frac, '
        || '(shared_blks_hit + shared_blks_read) / NULLIF(SUM(shared_blks_hit + shared_blks_read) OVER (), 0) AS blks_frac, '
        || 'shared_blks_read / NULLIF(SUM(shared_blks_read) OVER (), 0) AS read_frac, '
        || 'shared_blks_dirtied / NULLIF(SUM(shared_blks_dirtied) OVER (), 0) AS dirty_frac, '
        || wal_frac_expr || ' AS wal_frac '
        || 'FROM pg_stat_statements WHERE calls >= ' || min_call_count::text
        || ') SELECT left(query, 200) AS snippet, calls, rows, total_ms, mean_ms, '
        || 'round(time_frac::numeric, 4) AS time_frac, round(blks_frac::numeric, 4) AS blks_frac, '
        || 'round(read_frac::numeric, 4) AS read_frac, round(dirty_frac::numeric, 4) AS dirty_frac, '
        || 'round(wal_frac::numeric, 4) AS wal_frac '
        || 'FROM fracs WHERE time_frac > 0.05 OR blks_frac > 0.05 OR read_frac > 0.05 '
        || 'OR dirty_frac > 0.05 OR wal_frac > 0.05 '
        || 'ORDER BY total_ms DESC LIMIT ' || query_limit::text;

    FOR rec IN EXECUTE q LOOP
        result := result || jsonb_build_object(
            'query', rec.snippet, 'calls', rec.calls, 'rows', rec.rows,
            'total_ms', round(rec.total_ms::numeric, 2), 'mean_ms', round(rec.mean_ms::numeric, 2),
            'time_frac', rec.time_frac, 'blks_frac', rec.blks_frac,
            'read_frac', rec.read_frac, 'dirty_frac', rec.dirty_frac, 'wal_frac', rec.wal_frac);
    END LOOP;

    result := jsonb_build_object('sort_by', sort_mode, 'pg_version', pg_ver, 'min_calls', min_call_count, 'queries', result);
    PERFORM set_config('mcp.tool_result', result::text, true);
END mcp_block;
$mcp_custom_tool$ LANGUAGE plpgsql;

SELECT current_setting('mcp.tool_result', true);

COMMIT;
