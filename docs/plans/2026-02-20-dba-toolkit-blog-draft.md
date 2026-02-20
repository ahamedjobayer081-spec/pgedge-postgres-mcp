# Bridging the DBA Gap: Building a Bolt-On Toolkit for pgEdge's MCP Server

## The Problem Nobody Told Us About

We built the pgEdge Postgres MCP Server to let AI agents
talk to PostgreSQL databases. It does that well. Query
execution, schema discovery, semantic search, EXPLAIN
analysis -- the core DBA and developer workflow is covered.

Then someone pointed us at CrystalDBA's postgres-mcp
server. It's popular, it's basically abandoned, and it has
features we don't. Health checks. Top query analysis. Index
tuning with hypothetical index simulation. The kind of
stuff a DBA reaches for daily.

My first instinct was to build those features into our
server. New tools, new code, ship it. But that's the wrong
instinct -- and figuring out why changed the entire
approach.

## The Feature That Already Existed

Here's what actually happened when we sat down to figure
out the gap.

We have a connected pgEdge MCP server. It has
`query_database`. So instead of *assuming* we needed new
tools, we tried using what we already had:

```sql
SELECT left(query, 80) as query_snippet,
       calls,
       total_exec_time::numeric(10,2) as total_ms
FROM pg_stat_statements
ORDER BY total_exec_time DESC
LIMIT 5
```

Worked perfectly. Top queries, done. No new tool needed.

Same story for health checks. Unused indexes? One query
against `pg_stat_user_indexes`. Connection counts? One
query against `pg_stat_activity`. Buffer cache hit rates?
`pg_statio_user_tables`. Every single "missing feature"
was already possible through the existing `query_database`
tool.

So why did we -- and why would an LLM -- miss this?

## The Real Problem: Tool Descriptions Are UX

The `query_database` tool describes itself as a tool for
"structured, exact data retrieval." The examples are all
business queries -- "How many orders were placed last
week?" and "Show all users with status = 'active'."

An LLM scanning that description doesn't think "DBA
diagnostic tool." It thinks "business data tool." So when
a user asks "why is my database slow?" the LLM doesn't
reach for `query_database` to check `pg_stat_statements`.
It doesn't know it should.

CrystalDBA has a tool literally named `get_top_queries`
with a description about "slowest or most
resource-intensive queries." Impossible to miss.

This is a UX problem, not a capability problem. The
database can do everything. The MCP server can do
everything. But the tool *descriptions* don't tell the LLM
that.

## Three Options, One Clear Winner

We considered three approaches:

**Option A: Improve the existing tool descriptions.** Add
DBA-oriented examples to `query_database`. Zero token
overhead, but the LLM still needs to know *which*
diagnostic queries to run. That's a lot to ask.

**Option B: Add custom tools.** Named tools like
`analyze_db_health` that an LLM picks up immediately. Ship
them as a drop-in YAML file using the custom definitions
system already built into the server. No core changes.

**Option C: Add custom prompts.** Guided workflows that
walk the LLM through DBA tasks. The problem -- prompts
require the user to invoke them. When someone says "my
database seems slow," the LLM scans *tools*, not prompts.

Option B won. A tool named `analyze_db_health` is
unambiguous. The LLM sees it, knows what it does, and
reaches for it when the situation calls for it. And because
the pgEdge server already supports custom tool definitions
via YAML, we don't touch the core server at all.

But there's a bonus effect we didn't expect: the mere
*presence* of a DBA-oriented tool in the tool list changes
how the LLM perceives the *other* tools. Once it sees
`analyze_db_health`, it understands this is a
DBA-capable server -- and becomes more willing to use
`query_database` for adjacent DBA queries too. One tool
shifts the entire context.

## Token Budget Thinking

More tools means more token usage. Every tool definition --
name, description, parameter schema -- gets sent to the LLM
on every request. Research shows LLMs also start making
worse tool choices as the list grows.

Our server has about 11 built-in tools. Adding three more
is roughly 600-900 extra tokens per request. That's fine at
this scale. The dynamic tool discovery pattern (where tools
are searched on-demand instead of listed upfront) solves
the problem at 40-400 tools. We're nowhere near that.

But we used the token budget concern to drive design
decisions. Instead of replicating CrystalDBA's seven
separate health check tools, we built one combined
`analyze_db_health` with a `check_type` parameter. One tool
in the list, seven capabilities behind it.

## Reverse-Engineering CrystalDBA

We didn't just look at what CrystalDBA does -- we read
every line of how it does it. Here's what we found.

**Health checks:** Seven categories -- index, connection,
vacuum, sequence, replication, buffer, constraint. Some
are sophisticated (the index bloat calculation is a
multi-CTE query that estimates btree page counts). Some are
surprisingly basic (the connection health check just counts
connections against hardcoded thresholds of 500 total and
100 idle). We matched the sophisticated parts and improved
the basic ones.

**Top queries:** Three sort modes including a "resources"
mode that computes fractional consumption across five
dimensions -- execution time, shared blocks accessed/read/
dirtied, and WAL bytes. We matched all three modes and
added a `min_calls` filter to cut noise from one-off
queries.

**Index tuning:** This is where it gets interesting.

## The pglast Question

CrystalDBA's index tuner is a ~670-line Python system built
on two key pieces: pglast (a Python SQL parser) and HypoPG
(a PostgreSQL extension for hypothetical index simulation).

pglast parses SQL queries into abstract syntax trees and
extracts the columns referenced in WHERE clauses, JOIN
conditions, ORDER BY, and GROUP BY. That's how it generates
index candidates -- it knows exactly which columns are being
filtered and joined on.

HypoPG lets you create a "what-if" index without actually
building it, then run EXPLAIN to see if the query planner
would use it. That's how CrystalDBA validates whether a
candidate index actually helps.

Our server is written in Go. pglast is Python. The Go
equivalent (pg_query_go) wraps the same C library -- which
means it requires CGO. We literally just spent effort
removing CGO from the project (PR #72) to get pure Go
static binaries. Adding it back would undo that work.

We could use pglast inside a PL/Python custom tool running
inside PostgreSQL. But that requires the plpython3u
extension, which most managed Postgres services don't
support. Not a great look for a "bolt-on" tool.

So we asked a different question: what does pglast actually
give us that we can't approximate?

## Regex Beats AST (When You Have a Validator)

pglast extracts columns from query conditions precisely.
Regex pattern matching does it noisily -- it catches most
columns but also picks up some it shouldn't.

Here's the thing: it doesn't matter.

Every candidate index gets tested through HypoPG. A bad
candidate shows no cost improvement and gets discarded.
The algorithm self-corrects. Noisy candidate generation
produces the same final recommendations -- it just
evaluates a few extra candidates along the way.

Table alias resolution? `FROM orders o` is a regex pattern.
`FROM orders AS o` is a regex pattern. You don't need an
AST for that.

Column extraction from WHERE clauses? `WHERE column_name =`
is a regex pattern. `JOIN ... ON table.column =` is a regex
pattern. Imprecise, sure. But HypoPG handles the
imprecision.

The worst case is slightly slower analysis (more HypoPG
rounds), not worse recommendations. And we gave the search
loop a 30-second time budget anyway.

## Graceful Degradation: What CrystalDBA Doesn't Do

CrystalDBA requires HypoPG. If the extension isn't
installed, the tool errors out. Full stop.

We built two tiers:

**Tier 1** runs without any extensions. It checks system
catalogs for missing foreign key indexes, tables with
excessive sequential scans, unused indexes, and duplicate
indexes. This is heuristic -- no cost simulation -- but
it's useful information CrystalDBA doesn't offer at all.

**Tier 2** runs when HypoPG is available. Full
simulation-based analysis with the regex candidate
generation and greedy search loop.

The tool auto-detects what's installed and uses the best
available tier. One tool, two modes, always useful.

## What We Shipped

Three custom tools in a single YAML file:

- **get_top_queries** -- PL/pgSQL, three sort modes,
  extension detection, PG version-aware column names.
- **analyze_db_health** -- PL/pgSQL, seven health check
  categories, JSON output with summary scoring.
- **recommend_indexes** -- PL/pgSQL, two-tier degradation,
  regex-based candidate generation, HypoPG simulation.

No core server changes. No new dependencies. No CGO. Drop
in the YAML file, add one line to the config, restart.

The design prioritizes the decisions that matter for
production use: graceful degradation over hard requirements,
correct results over elegant implementation, and zero
dependencies over feature completeness.

<!-- TODO: Update this section with implementation details
and any decisions that changed during development -->

## What's Next

<!-- TODO: Fill in after implementation is complete -->
