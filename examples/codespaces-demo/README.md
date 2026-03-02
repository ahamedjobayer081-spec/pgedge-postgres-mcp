# pgEdge MCP Server — Codespaces Demo

Talk to a PostgreSQL database in plain English. This demo gives you
a running pgEdge MCP Server with the Northwind sample database —
ready to query in about 60 seconds.

## Before you launch

You'll need an API key from **Anthropic** or **OpenAI** (or both).

For the most secure setup, add your key as a
[Codespace secret](https://github.com/settings/codespaces) before
launching:

| Secret name | Where to get a key |
|---|---|
| `PGEDGE_ANTHROPIC_API_KEY` | [console.anthropic.com](https://console.anthropic.com/) |
| `PGEDGE_OPENAI_API_KEY` | [platform.openai.com](https://platform.openai.com/) |

> Your key is encrypted by GitHub. When the demo starts, it is
> written to a `.env` file inside your private Codespace VM.

Then click **Open in Codespaces** from the repo page.

## Getting started

1. Open the **Terminal** in the bottom pane
2. Run:

   ```bash
   ./examples/codespaces-demo/start.sh
   ```

3. If you didn't set a Codespace secret, the script will prompt
   you to paste your API key (hidden input, like a password)
4. Wait ~60 seconds for services to start
5. The Web UI opens automatically — log in with `demo` / `demo123`

## Try these queries

- "What tables are in the database?"
- "Show me the top 10 products by sales"
- "Which customers have placed more than 5 orders?"
- "Analyze order trends by month"
- "Show me the slowest queries from pg_stat_statements"

## What's inside

| Component | Details |
|-----------|---------|
| PostgreSQL 17 | pgEdge Enterprise with Spock, pg_stat_statements |
| Northwind dataset | Customers, orders, products (13 tables, ~1000 rows) |
| pgEdge MCP Server | Natural language to SQL, read-only queries |
| Web UI | Chat interface on port 8081 |

## Managing the demo

```bash
docker compose -f examples/codespaces-demo/docker-compose.yml logs -f    # View logs
docker compose -f examples/codespaces-demo/docker-compose.yml restart    # Restart
docker compose -f examples/codespaces-demo/docker-compose.yml down       # Stop
docker compose -f examples/codespaces-demo/docker-compose.yml down -v    # Stop + reset data
```

## Your API key stays private

Your key is stored as a Codespace secret (encrypted by GitHub) or
in a `.env` file inside your personal Codespace — a private,
ephemeral VM that only you can access. The key passes directly to
the LLM provider. **pgEdge never receives, stores, or proxies your
API key.** When you delete the Codespace, everything is deleted.

## Learn more

- [pgEdge MCP Server](https://github.com/pgEdge/pgedge-postgres-mcp)
- [pgEdge docs](https://docs.pgedge.com)
- [pgEdge website](https://www.pgedge.com)
