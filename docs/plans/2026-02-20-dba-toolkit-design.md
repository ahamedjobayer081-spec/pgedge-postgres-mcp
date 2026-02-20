# DBA Toolkit Custom Definitions Design

This document describes the design for a bolt-on DBA toolkit
that ships as a custom definitions YAML file for the pgEdge
PostgreSQL MCP Server.

## Motivation

The CrystalDBA postgres-mcp server provides DBA-oriented
tools (health checks, top queries, index tuning) that the
pgEdge MCP server does not offer as built-in tools. However,
the pgEdge server already has the underlying capability to
perform these tasks through its `query_database` tool.

The problem is discoverability. An LLM scanning available
tools does not recognize `query_database` as a DBA
diagnostic tool because the tool description and examples
focus on business-data queries. A tool named
`analyze_db_health` is unambiguous; the LLM selects it
autonomously when a user asks about database performance.

Rather than modifying the core server, the toolkit ships as
a custom definitions file that users drop in and reference
from their server configuration. This approach requires no
code changes, no recompilation, and no new dependencies.

## Deliverable

A single file at `examples/pgedge-postgres-mcp-dba.yaml`
containing three custom tool definitions. Users enable the
toolkit by adding one line to their server configuration:

```yaml
custom_definitions_path: "./pgedge-postgres-mcp-dba.yaml"
```

## Dependencies

The toolkit uses only capabilities already available in the
pgEdge MCP server and standard PostgreSQL installations.

- PL/pgSQL is required for all three tools. PL/pgSQL
  ships with every PostgreSQL installation and is in the
  pgEdge server's default `allowed_pl_languages` list.

- The `pg_stat_statements` extension is required for
  `get_top_queries` and the workload mode of
  `recommend_indexes`. The tools detect its absence and
  return installation guidance.

- The `hypopg` extension is optional. It enhances
  `recommend_indexes` with simulation-based cost analysis.
  The tool degrades gracefully without it.

- No CGO, no plpython3u, no external Python libraries, and
  no core server changes are required.

## Tool Summary

| Tool | Type | Extensions | Purpose |
|------|------|------------|---------|
| `get_top_queries` | pl-do (plpgsql) | pg_stat_statements | Top resource-consuming queries |
| `analyze_db_health` | pl-do (plpgsql) | none | Combined health checks |
| `recommend_indexes` | pl-do (plpgsql) | pg_stat_statements (optional), hypopg (optional) | Index recommendations |

## Tool Design: get_top_queries

### Purpose

Reports the most resource-intensive queries from
`pg_stat_statements`. Replaces CrystalDBA's
`get_top_queries` tool with equivalent functionality.

### Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| sort_by | string | resources | Sort order: `total_time`, `mean_time`, or `resources` |
| limit | integer | 10 | Number of queries to return |
| min_calls | integer | 5 | Minimum call count to filter noise |

### Behavior

The tool first checks whether `pg_stat_statements` is
installed. If the extension is missing, the tool returns
installation instructions explaining what the extension
does and how to enable it.

The tool detects the PostgreSQL version to use the correct
column names. PostgreSQL 13 renamed timing columns from
`total_time` to `total_exec_time` and added `wal_bytes`.
The tool targets PostgreSQL 13 and later.

Three sort modes are available:

- The `total_time` mode sorts by total execution time and
  returns query text (truncated to 200 characters), call
  count, total and mean execution time, rows returned, and
  buffer statistics.

- The `mean_time` mode sorts by mean execution time per
  call with the same output columns.

- The `resources` mode (default) computes fractional
  resource consumption across five dimensions: execution
  time, shared blocks accessed, shared blocks read, shared
  blocks dirtied, and WAL bytes. It filters to queries
  consuming more than 5% of any resource dimension. This
  mode matches CrystalDBA's resource analysis approach.

The `min_calls` parameter filters out one-off queries that
are not worth optimizing. CrystalDBA does not offer this
filter.

### Output

The tool returns results as a JSON object containing an
array of query records. Each record includes the query
snippet, call count, timing statistics, buffer statistics,
and (in resources mode) fractional resource consumption.

### Comparison with CrystalDBA

The tool matches CrystalDBA's three sort modes including
the sophisticated resources mode. It adds a `min_calls`
filter and PostgreSQL version detection. It does not support
PostgreSQL 12 (which is end-of-life).

## Tool Design: analyze_db_health

### Purpose

Runs comprehensive database health checks across seven
categories. Replaces CrystalDBA's `analyze_db_health` tool
with equivalent or improved functionality.

### Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| check_type | string | all | Comma-separated: `index`, `connection`, `vacuum`, `sequence`, `replication`, `buffer`, `constraint`, or `all` |

### Health Check Categories

#### Index Health

Checks for invalid indexes, duplicate indexes (using
covering-index logic that detects when one index's columns
are a prefix of another), index bloat (using the btree
space estimation query from CrystalDBA that calculates
expected vs actual page counts for indexes above 100MB),
and unused indexes with zero scans.

CrystalDBA comparison: matches all four sub-checks
including the sophisticated bloat estimation.

#### Connection Health

Reports active, idle, and idle-in-transaction connection
counts. Calculates utilization as a percentage of
`max_connections` from `pg_settings`. Detects long-running
queries exceeding five minutes.

CrystalDBA comparison: improves on CrystalDBA which only
checks two counts against hardcoded thresholds (500 total,
100 idle). Our version uses the actual `max_connections`
setting and adds long-running query detection.

#### Vacuum Health

Checks for tables with a dead tuple ratio above 20%
(indicating the need for VACUUM) and monitors transaction
ID wraparound risk by comparing frozen transaction age
against the 2.1 billion maximum.

CrystalDBA comparison: improves on CrystalDBA which only
checks transaction ID wraparound. Our version adds dead
tuple ratio analysis.

#### Sequence Health

Resolves sequence-to-table relationships by parsing
`nextval()` default expressions. Checks `last_value`
against the type-appropriate maximum (2.1 billion for
integer, 9.2 quintillion for bigint). Flags sequences at
or above 75% usage.

CrystalDBA comparison: matches CrystalDBA's thorough
approach of resolving sequence-to-table mappings and
handling integer vs bigint max values.

#### Replication Health

Detects whether the database is a primary or replica using
`pg_is_in_recovery()`. Reports replication lag in seconds
using version-appropriate WAL functions (PG10+ vs older).
Checks for active replication and inventories replication
slots (active and inactive).

CrystalDBA comparison: matches CrystalDBA's version-aware
replication monitoring.

#### Buffer Health

Calculates index cache hit rate from
`pg_statio_user_indexes` and table cache hit rate from
`pg_statio_user_tables`. Flags rates below 95%.

CrystalDBA comparison: equivalent implementation.

#### Constraint Health

Detects invalid constraints where `convalidated` is false
in `pg_constraint`. Reports the constraint name, table,
and referenced table if applicable.

CrystalDBA comparison: equivalent implementation.

### Output

The tool returns a JSON object with one key per check type.
Each key contains an array of findings. An empty array
means the check passed with no issues. A top-level
`summary` key provides pass, warning, and critical counts
for quick overview by the LLM.

Checks that query views which may not exist (such as
`pg_stat_replication` on a non-primary database) are
wrapped in exception handlers and return "not applicable."

## Tool Design: recommend_indexes

### Purpose

Analyzes database workload and recommends indexes. Provides
a two-tier approach that degrades gracefully based on
available extensions.

### Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| queries | string | null | Optional semicolon-separated SQL queries to analyze; if omitted, pulls from pg_stat_statements |
| max_index_size_mb | integer | 10000 | Storage budget for recommended indexes in MB |
| min_improvement_pct | integer | 10 | Minimum cost reduction percentage to recommend an index |

### Architecture: Two-Tier Approach

#### Tier 1: Heuristic Mode (No Extensions Required)

Tier 1 runs when neither `pg_stat_statements` (for
workload mode) nor `hypopg` is available. It provides
heuristic recommendations based on system catalog analysis.

Tier 1 checks include:

- Tables with high sequential scan counts relative to
  index scan counts from `pg_stat_user_tables`, indicating
  missing indexes.

- Foreign key columns without corresponding indexes by
  comparing `pg_constraint` foreign key references against
  `pg_index` entries.

- Unused or rarely-scanned indexes that waste space.

- Duplicate indexes where one index's columns are a prefix
  of another.

Tier 1 output includes the finding, the suggested action,
and a note that installing `hypopg` would enable
simulation-based cost analysis.

CrystalDBA comparison: CrystalDBA does not offer any
heuristic mode. It requires HypoPG and returns an error
if the extension is not installed.

#### Tier 2: Simulation Mode (With HypoPG)

Tier 2 runs when `hypopg` is available. It uses
hypothetical index simulation to validate and score
candidate indexes against actual query costs.

The process follows these steps:

1. Gather the workload. If the `queries` parameter is
   provided, use those queries. Otherwise, pull the top
   resource-consuming queries from `pg_stat_statements`
   (filtered by minimum call count and execution time).

2. Generate candidate indexes using regex-based column
   extraction from query text. The tool uses pattern
   matching to identify columns referenced in WHERE
   clauses, JOIN conditions, ORDER BY clauses, and GROUP
   BY clauses. It resolves table aliases by matching
   `FROM table alias` and `FROM table AS alias` patterns.

3. Filter candidates against existing indexes to avoid
   recommending duplicates.

4. Filter out long text columns (text type or varchar
   without a length limit) using `pg_stats.avg_width`.

5. For each candidate, create a hypothetical index using
   `hypopg_create_index()`, run `EXPLAIN (FORMAT JSON)`
   on the workload queries, and extract the total cost.

6. Compare the cost with and without the hypothetical
   index. Only recommend indexes that meet the
   `min_improvement_pct` threshold.

7. Apply a greedy selection loop: iteratively pick the
   candidate with the best cost improvement, add it to
   the recommended set, and re-evaluate remaining
   candidates with the accumulated indexes. Stop when no
   candidate meets the improvement threshold or the
   storage budget is exceeded.

8. Clean up by calling `hypopg_reset()`.

##### Column Extraction via Regex

The tool extracts candidate columns from query text using
pattern matching rather than AST parsing. Patterns include:

- `WHERE column_name =` and similar operators
- `JOIN ... ON table.column = ...`
- `ORDER BY column_name`
- `GROUP BY column_name`
- Table alias resolution via `FROM table alias` patterns

This approach produces noisier candidate sets than
CrystalDBA's pglast-based AST parsing. However, the HypoPG
simulation step validates each candidate against actual
query costs, filtering out ineffective candidates. The end
recommendations are equivalent in quality; the process may
evaluate more candidates and take slightly longer.

##### Time Budget

The greedy search loop operates within a 30-second time
budget (matching CrystalDBA). If the budget is exceeded,
the tool returns the best recommendations found so far.

### Output

The tool returns a JSON object containing:

- The tier used (heuristic or simulation).
- An array of recommendations, each with the suggested
  CREATE INDEX statement, the estimated index size, the
  cost improvement percentage, and the queries that
  benefit.
- A note about any extensions that could enhance the
  analysis if not currently installed.

### Comparison with CrystalDBA

| Aspect | CrystalDBA | This Design |
|--------|------------|-------------|
| HypoPG required | Yes (hard failure) | No (graceful degradation) |
| SQL parsing | pglast AST (precise) | Regex patterns (noisier) |
| Candidate filtering | HypoPG validates | HypoPG validates (same) |
| Recommendation quality | High | Equivalent (HypoPG filters noise) |
| Heuristic mode | Not available | Available (Tier 1) |
| Dependencies | Python, pglast, HypoPG | PL/pgSQL only |
| LLM optimizer | Yes (experimental) | Not included |
| Bind parameter replacement | pglast-based | Not included (uses normalized forms) |

## Design Decisions

### Custom Tools Over Core Changes

The toolkit ships as a YAML definitions file rather than
core server modifications. This avoids introducing new
dependencies, keeps the core server focused, and allows
users to opt in to DBA functionality only when needed.

### PL/pgSQL Over Pure SQL

All three tools use PL/pgSQL (pl-do type) because
they require procedural logic: multiple diagnostic queries
aggregated into a single result, conditional branching
based on extension availability, and iterative search
loops. PL/pgSQL ships with every PostgreSQL installation
and requires no additional setup.

### Regex Over AST Parsing

The `recommend_indexes` tool uses regex-based column
extraction instead of a SQL parser. Adding a Go SQL parser
(pg_query_go) would re-introduce CGO, which was
deliberately removed in PR #72. Using pglast via plpython3u
would add a heavy dependency. The regex approach is
imprecise but functional because HypoPG simulation
validates every candidate.

### Token Budget Awareness

Three tools add approximately 600-900 tokens to the tool
list. This is within the range where static tool listing
works well (the server has approximately 11 built-in tools).
Dynamic tool discovery patterns are unnecessary at this
scale but may become relevant if additional toolkit packs
are developed in the future.

### Graceful Degradation

Each tool detects its dependencies at runtime and provides
useful output regardless of what is available:

- `get_top_queries` checks for `pg_stat_statements` and
  returns installation guidance if missing.

- `analyze_db_health` wraps checks that depend on
  potentially unavailable views in exception handlers.

- `recommend_indexes` offers Tier 1 heuristics without
  any extensions and Tier 2 simulation when HypoPG is
  available.

## File Placement

```
examples/
    pgedge-postgres-mcp-dba.yaml    # The toolkit file
docs/
    plans/
        2026-02-20-dba-toolkit-design.md  # This document
```

## Future Considerations

- Additional toolkit packs (security audit, migration
  helper, monitoring) could follow the same pattern.

- If the number of custom tools grows beyond 20-30, the
  dynamic tool discovery pattern (search_tools,
  describe_tools, execute_tool) could reduce token usage.

- A Go SQL parser could replace regex-based column
  extraction if a pure-Go option becomes available that
  does not require CGO.

- The LLM optimizer method from CrystalDBA (using OpenAI
  for iterative index optimization) was not included in
  this design but could be added as a separate tool.
