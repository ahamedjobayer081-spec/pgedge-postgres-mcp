-- Test recommend_indexes tool (Tier 2 with HypoPG)
-- Run: psql -U demo -d northwind -f test_recommend_indexes.sql
--
-- Wraps the tool code in BEGIN/COMMIT to preserve the
-- transaction-local set_config result for retrieval.
-- Requires: hypopg extension for Tier 2 simulation

BEGIN;

DO $mcp_custom_tool$
<<mcp_block>>
DECLARE
    args jsonb := '{"queries":"SELECT * FROM orders WHERE customer_id = ''ALFKI'';SELECT o.order_id, p.product_name FROM order_details od JOIN orders o ON o.order_id = od.order_id JOIN products p ON p.product_id = od.product_id WHERE o.employee_id = 5"}'::jsonb;
    result jsonb;
    has_hypopg boolean;
    has_pgss boolean;
    tier text;
    rec RECORD;
    recommendations jsonb := '[]'::jsonb;
    notes jsonb := '[]'::jsonb;
BEGIN
    SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'hypopg') INTO has_hypopg;
    SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements') INTO has_pgss;

    IF has_hypopg THEN tier := 'simulation';
    ELSE
        tier := 'heuristic';
        notes := notes || to_jsonb('Install hypopg for simulation-based index recommendations: CREATE EXTENSION hypopg;'::text);
    END IF;

    IF NOT has_pgss THEN
        notes := notes || to_jsonb('Install pg_stat_statements for workload-based analysis: CREATE EXTENSION pg_stat_statements;'::text);
    END IF;

    -- ======= TIER 1: HEURISTIC MODE =======
    -- FK columns without indexes
    FOR rec IN
        SELECT n.nspname || '.' || t.relname AS table_name,
            a.attname AS column_name,
            fn.nspname || '.' || ft.relname AS referenced_table
        FROM pg_constraint c
        JOIN pg_class t ON t.oid = c.conrelid
        JOIN pg_namespace n ON n.oid = t.relnamespace
        JOIN pg_class ft ON ft.oid = c.confrelid
        JOIN pg_namespace fn ON fn.oid = ft.relnamespace
        JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = c.conkey[1]
        WHERE c.contype = 'f'
          AND NOT EXISTS (SELECT 1 FROM pg_index i
              WHERE i.indrelid = c.conrelid AND a.attnum = ANY(i.indkey))
          AND n.nspname NOT IN ('pg_catalog', 'information_schema')
    LOOP
        recommendations := recommendations || jsonb_build_object(
            'type', 'missing_fk_index', 'table', rec.table_name,
            'column', rec.column_name, 'referenced_table', rec.referenced_table,
            'create_index', 'CREATE INDEX ON ' || rec.table_name || ' (' || rec.column_name || ')',
            'reason', 'FK without index slows JOINs and cascading deletes');
    END LOOP;

    -- Tables with high sequential scan ratio
    FOR rec IN
        SELECT schemaname || '.' || relname AS table_name,
            seq_scan, idx_scan,
            CASE WHEN seq_scan + idx_scan > 0
                THEN round(100.0 * seq_scan / (seq_scan + idx_scan), 1) ELSE 0 END AS seq_scan_pct,
            pg_size_pretty(pg_relation_size(relid)) AS table_size
        FROM pg_stat_user_tables
        WHERE seq_scan > idx_scan * 10 AND seq_scan > 100
          AND pg_relation_size(relid) > 10485760
        ORDER BY seq_scan DESC LIMIT 10
    LOOP
        recommendations := recommendations || jsonb_build_object(
            'type', 'high_seq_scan', 'table', rec.table_name,
            'sequential_scans', rec.seq_scan, 'index_scans', rec.idx_scan,
            'seq_scan_pct', rec.seq_scan_pct, 'table_size', rec.table_size,
            'reason', rec.seq_scan_pct || '% sequential scans');
    END LOOP;

    -- Unused indexes
    FOR rec IN
        SELECT schemaname || '.' || indexrelname AS index_name,
            relname AS table_name,
            pg_size_pretty(pg_relation_size(indexrelid)) AS index_size
        FROM pg_stat_user_indexes
        WHERE idx_scan = 0 AND pg_relation_size(indexrelid) > 1048576
        ORDER BY pg_relation_size(indexrelid) DESC LIMIT 10
    LOOP
        recommendations := recommendations || jsonb_build_object(
            'type', 'unused_index', 'index', rec.index_name,
            'table', rec.table_name, 'index_size', rec.index_size,
            'action', 'DROP INDEX ' || rec.index_name,
            'reason', 'Index has zero scans; wasting storage');
    END LOOP;

    -- Duplicate / prefix indexes
    FOR rec IN
        WITH idx_cols AS (
            SELECT n.nspname || '.' || ct.relname AS table_name,
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
        SELECT a.table_name, a.index_name AS redundant_index,
            b.index_name AS covering_index,
            pg_size_pretty(a.index_bytes) AS wasted_size
        FROM idx_cols a JOIN idx_cols b
            ON a.indrelid = b.indrelid AND a.indexrelid <> b.indexrelid
            AND array_length(a.columns, 1) <= array_length(b.columns, 1)
            AND a.columns = b.columns[1:array_length(a.columns, 1)]
    LOOP
        recommendations := recommendations || jsonb_build_object(
            'type', 'duplicate_index', 'table', rec.table_name,
            'redundant_index', rec.redundant_index,
            'covering_index', rec.covering_index,
            'wasted_size', rec.wasted_size,
            'action', 'DROP INDEX ' || rec.redundant_index,
            'reason', 'Redundant; covered by ' || rec.covering_index);
    END LOOP;

    -- ======= TIER 2: SIMULATION MODE =======
    IF has_hypopg THEN
        DECLARE
            user_queries text;
            min_imp integer;
            pg_ver integer;
            time_col text;
            workload jsonb := '[]'::jsonb;
            candidates jsonb := '[]'::jsonb;
            sim_recs jsonb := '[]'::jsonb;
            seen jsonb := '{}'::jsonb;
            deduped jsonb := '[]'::jsonb;
            q_rec RECORD;
            alias_map jsonb;
            qtext text;
            cand_key text;
            tbl_name text;
            col_name text;
            create_stmt text;
            explain_text text;
            baseline numeric;
            new_cost numeric;
            improvement numeric;
            hypo_oid bigint;
            start_ts timestamptz;
            i integer;
            j integer;
        BEGIN
            user_queries := args->>'queries';
            min_imp := coalesce((args->>'min_improvement_pct')::integer, 10);
            start_ts := clock_timestamp();

            PERFORM hypopg_reset();

            -- Gather workload
            IF user_queries IS NOT NULL THEN
                FOR q_rec IN
                    SELECT trim(q) AS query_text
                    FROM unnest(string_to_array(user_queries, ';')) AS q
                    WHERE trim(q) <> ''
                LOOP
                    workload := workload || jsonb_build_object(
                        'query', q_rec.query_text, 'calls', 1);
                END LOOP;
            ELSIF has_pgss THEN
                pg_ver := current_setting('server_version_num')::integer / 10000;
                IF pg_ver >= 13 THEN time_col := 'total_exec_time';
                ELSE time_col := 'total_time'; END IF;

                FOR q_rec IN EXECUTE
                    'SELECT query AS query_text, calls'
                    || ' FROM pg_stat_statements'
                    || ' WHERE calls >= 5 AND ' || time_col || ' > 100'
                    || ' AND query !~ ''^(SET|SHOW|BEGIN|COMMIT|ROLLBACK)'''
                    || ' ORDER BY ' || time_col || ' DESC LIMIT 20'
                LOOP
                    workload := workload || jsonb_build_object(
                        'query', q_rec.query_text, 'calls', q_rec.calls);
                END LOOP;
            END IF;

            IF jsonb_array_length(workload) = 0 THEN
                notes := notes || to_jsonb(
                    'No workload queries found.'::text);
            END IF;

            -- Extract candidates from workload
            FOR i IN 0..CASE WHEN jsonb_typeof(workload) = 'array'
                THEN jsonb_array_length(workload) - 1 ELSE -1 END
            LOOP
                qtext := (workload->i)->>'query';
                alias_map := '{}'::jsonb;
                qtext := regexp_replace(qtext, '\$\d+', 'NULL', 'g');

                -- Build alias map
                FOR q_rec IN
                    SELECT cols[1] AS tbl, cols[2] AS als
                    FROM regexp_matches(qtext,
                        '(?:FROM|JOIN)\s+(\w+(?:\.\w+)?)\s+(?:AS\s+)?(\w+)', 'gi') AS cols
                LOOP
                    alias_map := alias_map || jsonb_build_object(q_rec.als, q_rec.tbl);
                END LOOP;

                -- Extract table.column refs
                FOR q_rec IN
                    SELECT cols[1] AS prefix, cols[2] AS col
                    FROM regexp_matches(qtext, '(\w+)\.(\w+)', 'g') AS cols
                LOOP
                    tbl_name := alias_map->>q_rec.prefix;
                    IF tbl_name IS NULL THEN tbl_name := q_rec.prefix; END IF;
                    col_name := q_rec.col;

                    IF EXISTS (
                        SELECT 1 FROM pg_attribute a
                        JOIN pg_class c ON c.oid = a.attrelid
                        JOIN pg_namespace n ON n.oid = c.relnamespace
                        WHERE a.attname = col_name
                          AND (n.nspname || '.' || c.relname = tbl_name OR c.relname = tbl_name)
                          AND c.relkind = 'r'
                          AND n.nspname NOT IN ('pg_catalog', 'information_schema')
                          AND a.atttypid NOT IN (25)
                          AND NOT (a.atttypid = 1043 AND a.atttypmod = -1)
                    ) THEN
                        SELECT n.nspname || '.' || c.relname INTO tbl_name
                        FROM pg_attribute a
                        JOIN pg_class c ON c.oid = a.attrelid
                        JOIN pg_namespace n ON n.oid = c.relnamespace
                        WHERE a.attname = col_name
                          AND (n.nspname || '.' || c.relname = tbl_name OR c.relname = tbl_name)
                          AND c.relkind = 'r'
                          AND n.nspname NOT IN ('pg_catalog', 'information_schema')
                          AND a.atttypid NOT IN (25)
                          AND NOT (a.atttypid = 1043 AND a.atttypmod = -1)
                        LIMIT 1;
                        candidates := candidates || jsonb_build_object(
                            'table', tbl_name, 'column', col_name);
                    END IF;
                END LOOP;
            END LOOP;

            -- Deduplicate candidates
            FOR i IN 0..CASE WHEN jsonb_typeof(candidates) = 'array'
                THEN jsonb_array_length(candidates) - 1 ELSE -1 END
            LOOP
                tbl_name := (candidates->i)->>'table';
                col_name := (candidates->i)->>'column';
                cand_key := tbl_name || '.' || col_name;
                IF NOT seen ? cand_key THEN
                    seen := seen || jsonb_build_object(cand_key, true);
                    deduped := deduped || candidates->i;
                END IF;
            END LOOP;
            candidates := deduped;

            -- Filter existing indexes
            deduped := '[]'::jsonb;
            FOR i IN 0..CASE WHEN jsonb_typeof(candidates) = 'array'
                THEN jsonb_array_length(candidates) - 1 ELSE -1 END
            LOOP
                tbl_name := (candidates->i)->>'table';
                col_name := (candidates->i)->>'column';
                IF NOT EXISTS (
                    SELECT 1 FROM pg_index idx
                    JOIN pg_class c ON c.oid = idx.indrelid
                    JOIN pg_namespace n ON n.oid = c.relnamespace
                    JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = idx.indkey[1]
                    WHERE n.nspname || '.' || c.relname = tbl_name
                      AND a.attname = col_name
                ) THEN
                    deduped := deduped || candidates->i;
                END IF;
            END LOOP;
            candidates := deduped;

            -- HypoPG simulation
            FOR i IN 0..CASE WHEN jsonb_typeof(candidates) = 'array'
                THEN jsonb_array_length(candidates) - 1 ELSE -1 END
            LOOP
                IF clock_timestamp() - start_ts > interval '30 seconds' THEN
                    notes := notes || to_jsonb('Time budget exceeded'::text);
                    EXIT;
                END IF;

                tbl_name := (candidates->i)->>'table';
                col_name := (candidates->i)->>'column';
                create_stmt := 'CREATE INDEX ON ' || tbl_name || ' (' || col_name || ')';

                improvement := 0;
                FOR j IN 0..CASE WHEN jsonb_typeof(workload) = 'array'
                    THEN jsonb_array_length(workload) - 1 ELSE -1 END
                LOOP
                    qtext := (workload->j)->>'query';
                    qtext := regexp_replace(qtext, '\$\d+', 'NULL', 'g');

                    BEGIN
                        EXECUTE 'EXPLAIN (FORMAT JSON) ' || qtext INTO explain_text;
                        baseline := (explain_text::jsonb->0->'Plan'->>'Total Cost')::numeric;

                        SELECT indexrelid INTO hypo_oid FROM hypopg_create_index(create_stmt);

                        EXECUTE 'EXPLAIN (FORMAT JSON) ' || qtext INTO explain_text;
                        new_cost := (explain_text::jsonb->0->'Plan'->>'Total Cost')::numeric;

                        IF baseline > 0 THEN
                            improvement := GREATEST(improvement,
                                round(100.0 * (baseline - new_cost) / baseline, 1));
                        END IF;

                        PERFORM hypopg_drop_index(hypo_oid);
                    EXCEPTION WHEN OTHERS THEN
                        BEGIN PERFORM hypopg_reset();
                        EXCEPTION WHEN OTHERS THEN NULL; END;
                    END;
                END LOOP;

                IF improvement >= min_imp THEN
                    sim_recs := sim_recs || jsonb_build_object(
                        'type', 'simulation', 'table', tbl_name,
                        'column', col_name, 'create_index', create_stmt,
                        'improvement_pct', improvement,
                        'reason', 'HypoPG shows ' || improvement || '% cost reduction');
                END IF;
            END LOOP;

            PERFORM hypopg_reset();
            recommendations := recommendations || sim_recs;
        END;
    END IF;

    result := jsonb_build_object(
        'tier', tier, 'recommendations', recommendations, 'notes', notes);
    PERFORM set_config('mcp.tool_result', result::text, true);
END mcp_block;
$mcp_custom_tool$ LANGUAGE plpgsql;

SELECT current_setting('mcp.tool_result', true);

COMMIT;
