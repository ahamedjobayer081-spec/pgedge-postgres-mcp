-- Test analyze_db_health tool (all 7 check categories)
-- Run: psql -U demo -d northwind -f test_analyze_db_health.sql
--
-- Wraps the tool code in BEGIN/COMMIT to preserve the
-- transaction-local set_config result for retrieval.

BEGIN;

DO $mcp_custom_tool$
<<mcp_block>>
DECLARE
    args jsonb := '{"check_type":"all"}'::jsonb;
    result jsonb;
    checks text[];
    check_item text;
    rec RECORD;
    findings jsonb;
    pass_count integer := 0;
    warn_count integer := 0;
    crit_count integer := 0;
    max_conn integer;
    conn_pct numeric;
    frozen_max bigint := 2100000000;
    seq_max_int bigint := 2147483647;
    seq_max_bigint bigint := 9223372036854775807;
    idx_hit numeric;
    tbl_hit numeric;
    is_replica boolean;
BEGIN
    result := '{}'::jsonb;
    checks := ARRAY['index', 'connection', 'vacuum',
        'sequence', 'replication', 'buffer', 'constraint'];

    FOREACH check_item IN ARRAY checks LOOP
        check_item := trim(check_item);
        findings := '[]'::jsonb;

        -- ========== INDEX HEALTH ==========
        IF check_item = 'index' THEN
            FOR rec IN
                SELECT schemaname || '.' || indexrelname AS index_name,
                    relname AS table_name
                FROM pg_stat_user_indexes sui
                JOIN pg_index i ON i.indexrelid = sui.indexrelid
                WHERE NOT i.indisvalid
            LOOP
                findings := findings || jsonb_build_object(
                    'check', 'invalid_index', 'severity', 'critical',
                    'index', rec.index_name, 'table', rec.table_name);
                crit_count := crit_count + 1;
            END LOOP;

            FOR rec IN
                WITH idx_cols AS (
                    SELECT n.nspname AS schemaname, ct.relname AS table_name,
                        ci.relname AS index_name, i.indexrelid, i.indrelid,
                        array_agg(a.attname ORDER BY array_position(i.indkey, a.attnum)) AS columns,
                        pg_relation_size(ci.oid) AS index_bytes
                    FROM pg_index i
                    JOIN pg_class ci ON ci.oid = i.indexrelid
                    JOIN pg_class ct ON ct.oid = i.indrelid
                    JOIN pg_namespace n ON n.oid = ci.relnamespace
                    JOIN pg_attribute a ON a.attrelid = ct.oid AND a.attnum = ANY(i.indkey)
                    WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
                    GROUP BY n.nspname, ct.relname, ci.relname, i.indexrelid, i.indrelid, ci.oid
                )
                SELECT a.schemaname || '.' || a.table_name AS full_table,
                    a.index_name AS redundant_index, b.index_name AS covering_index,
                    pg_size_pretty(a.index_bytes) AS wasted_size
                FROM idx_cols a JOIN idx_cols b
                    ON a.indrelid = b.indrelid AND a.indexrelid <> b.indexrelid
                    AND array_length(a.columns, 1) <= array_length(b.columns, 1)
                    AND a.columns = b.columns[1:array_length(a.columns, 1)]
            LOOP
                findings := findings || jsonb_build_object(
                    'check', 'duplicate_index', 'severity', 'warning',
                    'table', rec.full_table,
                    'redundant_index', rec.redundant_index,
                    'covering_index', rec.covering_index,
                    'wasted_size', rec.wasted_size);
                warn_count := warn_count + 1;
            END LOOP;

            BEGIN
                FOR rec IN
                    WITH bloat_est AS (
                        SELECT n.nspname || '.' || ci.relname AS index_name,
                            ct.relname AS table_name,
                            ci.relpages AS actual_pages,
                            pg_relation_size(ci.oid) AS index_bytes,
                            GREATEST(CEIL(ci.reltuples * (8 + COALESCE((
                                SELECT AVG(COALESCE(s.avg_width, 8))
                                FROM pg_attribute a
                                LEFT JOIN pg_stats s
                                    ON s.tablename = ct.relname
                                    AND s.attname = a.attname
                                    AND s.schemaname = n.nspname
                                WHERE a.attrelid = i.indrelid
                                  AND a.attnum = ANY(i.indkey)
                                  AND a.attnum > 0
                            ), 8)) / (current_setting('block_size')::integer - 24)), 1)
                                AS estimated_pages
                        FROM pg_index i
                        JOIN pg_class ci ON ci.oid = i.indexrelid
                        JOIN pg_class ct ON ct.oid = i.indrelid
                        JOIN pg_namespace n ON n.oid = ci.relnamespace
                        WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
                          AND ci.relkind = 'i'
                          AND pg_relation_size(ci.oid) > 104857600
                          AND ci.relpages > 0
                    )
                    SELECT index_name, table_name,
                        pg_size_pretty(index_bytes) AS index_size,
                        round(actual_pages::numeric / estimated_pages::numeric, 1) AS bloat_ratio
                    FROM bloat_est WHERE actual_pages > estimated_pages * 2
                LOOP
                    findings := findings || jsonb_build_object(
                        'check', 'index_bloat', 'severity', 'warning',
                        'index', rec.index_name,
                        'bloat_ratio', rec.bloat_ratio::text || 'x');
                    warn_count := warn_count + 1;
                END LOOP;
            EXCEPTION WHEN OTHERS THEN
                findings := findings || jsonb_build_object(
                    'check', 'index_bloat', 'severity', 'info',
                    'message', 'Could not estimate: ' || SQLERRM);
            END;

            FOR rec IN
                SELECT schemaname || '.' || indexrelname AS index_name,
                    relname AS table_name,
                    pg_size_pretty(pg_relation_size(indexrelid)) AS index_size
                FROM pg_stat_user_indexes
                WHERE idx_scan = 0
                  AND pg_relation_size(indexrelid) > 1048576
                ORDER BY pg_relation_size(indexrelid) DESC LIMIT 20
            LOOP
                findings := findings || jsonb_build_object(
                    'check', 'unused_index', 'severity', 'warning',
                    'index', rec.index_name, 'index_size', rec.index_size);
                warn_count := warn_count + 1;
            END LOOP;
            IF findings = '[]'::jsonb THEN pass_count := pass_count + 1; END IF;

        -- ========== CONNECTION HEALTH ==========
        ELSIF check_item = 'connection' THEN
            SELECT current_setting('max_connections')::integer INTO max_conn;
            FOR rec IN
                SELECT count(*) FILTER (WHERE state = 'active') AS active,
                    count(*) FILTER (WHERE state = 'idle') AS idle,
                    count(*) FILTER (WHERE state = 'idle in transaction') AS idle_in_txn,
                    count(*) AS total
                FROM pg_stat_activity WHERE backend_type = 'client backend'
            LOOP
                conn_pct := round(100.0 * rec.total / max_conn, 1);
                findings := findings || jsonb_build_object(
                    'check', 'connection_count',
                    'severity', CASE WHEN conn_pct > 90 THEN 'critical'
                        WHEN conn_pct > 75 THEN 'warning' ELSE 'ok' END,
                    'active', rec.active, 'idle', rec.idle,
                    'idle_in_transaction', rec.idle_in_txn,
                    'total', rec.total, 'max_connections', max_conn,
                    'utilization_pct', conn_pct);
                IF conn_pct > 75 THEN warn_count := warn_count + 1;
                ELSE pass_count := pass_count + 1; END IF;
            END LOOP;
            FOR rec IN
                SELECT pid, usename, left(query, 200) AS query_snippet,
                    extract(epoch FROM (now() - query_start))::integer AS duration_sec
                FROM pg_stat_activity
                WHERE state = 'active'
                  AND query_start < now() - interval '5 minutes'
                  AND backend_type = 'client backend'
                ORDER BY query_start LIMIT 10
            LOOP
                findings := findings || jsonb_build_object(
                    'check', 'long_running_query', 'severity', 'warning',
                    'pid', rec.pid, 'duration_seconds', rec.duration_sec);
                warn_count := warn_count + 1;
            END LOOP;

        -- ========== VACUUM HEALTH ==========
        ELSIF check_item = 'vacuum' THEN
            FOR rec IN
                SELECT schemaname || '.' || relname AS table_name,
                    n_live_tup, n_dead_tup,
                    round(100.0 * n_dead_tup / NULLIF(n_live_tup + n_dead_tup, 0), 1) AS dead_pct,
                    last_vacuum, last_autovacuum
                FROM pg_stat_user_tables
                WHERE n_dead_tup > 0
                  AND round(100.0 * n_dead_tup / NULLIF(n_live_tup + n_dead_tup, 0), 1) > 20
                ORDER BY n_dead_tup DESC LIMIT 20
            LOOP
                findings := findings || jsonb_build_object(
                    'check', 'dead_tuples',
                    'severity', CASE WHEN rec.dead_pct > 50 THEN 'critical' ELSE 'warning' END,
                    'table', rec.table_name, 'dead_tuple_pct', rec.dead_pct,
                    'action', 'VACUUM ANALYZE ' || rec.table_name);
                IF rec.dead_pct > 50 THEN crit_count := crit_count + 1;
                ELSE warn_count := warn_count + 1; END IF;
            END LOOP;
            FOR rec IN
                SELECT datname, age(datfrozenxid) AS frozen_age,
                    round(100.0 * age(datfrozenxid) / frozen_max, 1) AS pct_to_wrap
                FROM pg_database WHERE datname = current_database()
            LOOP
                IF rec.pct_to_wrap > 75 THEN
                    findings := findings || jsonb_build_object(
                        'check', 'txid_wraparound',
                        'severity', CASE WHEN rec.pct_to_wrap > 90 THEN 'critical' ELSE 'warning' END,
                        'pct_to_wraparound', rec.pct_to_wrap,
                        'action', 'Run VACUUM FREEZE');
                    IF rec.pct_to_wrap > 90 THEN crit_count := crit_count + 1;
                    ELSE warn_count := warn_count + 1; END IF;
                END IF;
            END LOOP;
            IF findings = '[]'::jsonb THEN pass_count := pass_count + 1; END IF;

        -- ========== SEQUENCE HEALTH ==========
        ELSIF check_item = 'sequence' THEN
            FOR rec IN
                SELECT n.nspname || '.' || s.relname AS seq_name,
                    d.refobjid::regclass::text AS table_name,
                    pg_sequences.last_value,
                    pg_sequences.data_type::text AS data_type,
                    round(100.0 * pg_sequences.last_value
                        / CASE WHEN pg_sequences.data_type::text = 'bigint'
                            THEN seq_max_bigint ELSE seq_max_int END, 2)
                        AS usage_pct
                FROM pg_class s
                JOIN pg_namespace n ON n.oid = s.relnamespace
                JOIN pg_depend d ON d.objid = s.oid AND d.deptype = 'a'
                JOIN pg_sequences
                    ON pg_sequences.schemaname = n.nspname
                    AND pg_sequences.sequencename = s.relname
                WHERE s.relkind = 'S'
                  AND pg_sequences.last_value IS NOT NULL
            LOOP
                IF rec.usage_pct >= 75 THEN
                    findings := findings || jsonb_build_object(
                        'check', 'sequence_exhaustion',
                        'severity', CASE WHEN rec.usage_pct >= 90 THEN 'critical' ELSE 'warning' END,
                        'sequence', rec.seq_name, 'table', rec.table_name,
                        'usage_pct', rec.usage_pct);
                    IF rec.usage_pct >= 90 THEN crit_count := crit_count + 1;
                    ELSE warn_count := warn_count + 1; END IF;
                END IF;
            END LOOP;
            IF findings = '[]'::jsonb THEN pass_count := pass_count + 1; END IF;

        -- ========== REPLICATION HEALTH ==========
        ELSIF check_item = 'replication' THEN
            BEGIN
                SELECT pg_is_in_recovery() INTO is_replica;
                findings := findings || jsonb_build_object(
                    'check', 'replication_role', 'severity', 'ok',
                    'role', CASE WHEN is_replica THEN 'replica' ELSE 'primary' END);
                pass_count := pass_count + 1;
                IF is_replica THEN
                    FOR rec IN
                        SELECT extract(epoch FROM
                            (now() - pg_last_xact_replay_timestamp()))::integer AS lag_seconds
                    LOOP
                        IF rec.lag_seconds IS NOT NULL AND rec.lag_seconds > 300 THEN
                            findings := findings || jsonb_build_object(
                                'check', 'replication_lag',
                                'severity', CASE WHEN rec.lag_seconds > 3600 THEN 'critical' ELSE 'warning' END,
                                'lag_seconds', rec.lag_seconds);
                            warn_count := warn_count + 1;
                        END IF;
                    END LOOP;
                ELSE
                    FOR rec IN SELECT slot_name, slot_type, active FROM pg_replication_slots LOOP
                        IF NOT rec.active THEN
                            findings := findings || jsonb_build_object(
                                'check', 'inactive_slot', 'severity', 'warning',
                                'slot_name', rec.slot_name, 'slot_type', rec.slot_type);
                            warn_count := warn_count + 1;
                        END IF;
                    END LOOP;
                END IF;
            EXCEPTION WHEN OTHERS THEN
                findings := findings || jsonb_build_object(
                    'check', 'replication', 'severity', 'info',
                    'message', 'Not applicable: ' || SQLERRM);
            END;

        -- ========== BUFFER HEALTH ==========
        ELSIF check_item = 'buffer' THEN
            SELECT round(100.0 * sum(idx_blks_hit)
                / NULLIF(sum(idx_blks_hit) + sum(idx_blks_read), 0), 2)
            INTO idx_hit FROM pg_statio_user_indexes;
            SELECT round(100.0 * sum(heap_blks_hit)
                / NULLIF(sum(heap_blks_hit) + sum(heap_blks_read), 0), 2)
            INTO tbl_hit FROM pg_statio_user_tables;
            findings := findings || jsonb_build_object(
                'check', 'cache_hit_rate',
                'severity', CASE
                    WHEN LEAST(coalesce(idx_hit, 100), coalesce(tbl_hit, 100)) < 90 THEN 'critical'
                    WHEN LEAST(coalesce(idx_hit, 100), coalesce(tbl_hit, 100)) < 95 THEN 'warning'
                    ELSE 'ok' END,
                'index_hit_rate_pct', coalesce(idx_hit, 0),
                'table_hit_rate_pct', coalesce(tbl_hit, 0));
            IF LEAST(coalesce(idx_hit, 100), coalesce(tbl_hit, 100)) < 90 THEN
                crit_count := crit_count + 1;
            ELSIF LEAST(coalesce(idx_hit, 100), coalesce(tbl_hit, 100)) < 95 THEN
                warn_count := warn_count + 1;
            ELSE pass_count := pass_count + 1; END IF;

        -- ========== CONSTRAINT HEALTH ==========
        ELSIF check_item = 'constraint' THEN
            FOR rec IN
                SELECT c.conname AS constraint_name,
                    n.nspname || '.' || t.relname AS table_name,
                    CASE WHEN c.confrelid > 0
                        THEN fn.nspname || '.' || ft.relname ELSE null END AS referenced_table
                FROM pg_constraint c
                JOIN pg_class t ON t.oid = c.conrelid
                JOIN pg_namespace n ON n.oid = t.relnamespace
                LEFT JOIN pg_class ft ON ft.oid = c.confrelid
                LEFT JOIN pg_namespace fn ON fn.oid = ft.relnamespace
                WHERE NOT c.convalidated
            LOOP
                findings := findings || jsonb_build_object(
                    'check', 'invalid_constraint', 'severity', 'warning',
                    'constraint', rec.constraint_name,
                    'table', rec.table_name,
                    'referenced_table', rec.referenced_table);
                warn_count := warn_count + 1;
            END LOOP;
            IF findings = '[]'::jsonb THEN pass_count := pass_count + 1; END IF;
        END IF;

        result := result || jsonb_build_object(check_item, findings);
    END LOOP;

    result := result || jsonb_build_object('summary', jsonb_build_object(
        'checks_run', array_length(checks, 1),
        'pass', pass_count, 'warning', warn_count, 'critical', crit_count));
    PERFORM set_config('mcp.tool_result', result::text, true);
END mcp_block;
$mcp_custom_tool$ LANGUAGE plpgsql;

SELECT current_setting('mcp.tool_result', true);

COMMIT;
