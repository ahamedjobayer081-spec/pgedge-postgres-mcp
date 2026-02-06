# Building a Custom Knowledge Base

A custom knowledge base improves the accuracy of natural
language queries by giving the LLM domain-specific context
about your tables, columns, business rules, and query
patterns. When a user asks a question in natural language,
the LLM searches the knowledge base for relevant context
before generating SQL. This tutorial walks you through the
entire process of creating, building, and deploying a custom
knowledge base.

## How Custom Knowledge Bases Improve Queries

The knowledge base provides a feedback loop between your
domain documentation and the LLM query engine. The following
steps describe how the system uses this loop.

1. You document your schema, including tables, columns,
   relationships, and business rules.

2. The `pgedge-nla-kb-builder` tool processes the
   documentation into searchable chunks with embeddings.

3. The MCP server loads the custom knowledge base alongside
   or instead of the standard knowledge base.

4. When a user asks a natural language question, the LLM
   calls `search_knowledgebase` to find relevant domain
   context.

5. The LLM uses the retrieved context to generate more
   accurate SQL for the question.

The more thoroughly you document your domain, the better the
LLM performs when answering questions about your data.

## Creating Domain Documentation

Domain documentation describes your database schema, business
rules, and common query patterns. You write the documentation
as Markdown files and store the files in a dedicated
directory.

### Writing Schema Documentation

Schema documentation describes each table, its columns, and
the data the table contains. Create one Markdown file per
table group or major topic.

In the following example, the Markdown file documents an
e-commerce database schema with tables and common queries:

```markdown
# E-Commerce Database Schema

## Orders Table

The `orders` table stores all customer purchase records.

| Column | Type | Description |
|--------|------|-------------|
| id | SERIAL | The unique order identifier. |
| customer_id | INTEGER | References the customers table. |
| status | VARCHAR(20) | The order status value. |
| total_amount | NUMERIC(10,2) | The order total in USD. |
| created_at | TIMESTAMPTZ | The order creation timestamp. |

### Common Queries

Find orders by status:

` ` `sql
SELECT * FROM orders WHERE status = 'pending';
` ` `

Calculate daily revenue:

` ` `sql
SELECT DATE(created_at) AS day, SUM(total_amount)
FROM orders
WHERE status != 'cancelled'
GROUP BY DATE(created_at);
` ` `
```

### Writing Business Rules Documentation

Business rules documentation defines domain-specific terms
and metrics that the LLM needs to understand. The LLM uses
these definitions to translate natural language into correct
SQL.

In the following example, the Markdown file documents revenue
metrics and customer status definitions:

```markdown
# Business Rules and Glossary

## Revenue Metrics

Net revenue equals the sum of order amounts excluding
cancelled and refunded orders. Gross revenue equals the
sum of all order amounts including cancelled orders.
Average order value (AOV) equals net revenue divided
by the count of completed orders.

## Status Definitions

An active customer has at least one order in the last
90 days. A churned customer has no orders in the last
180 days.
```

### Writing Relationship Documentation

Relationship documentation describes how tables connect
through foreign keys and join patterns. The LLM uses this
context to generate correct JOIN clauses.

In the following example, the Markdown file documents a
one-to-many relationship between customers and orders:

```markdown
# Table Relationships

## Customer to Orders (One-to-Many)

Each customer can have many orders. Join customers to
their orders using the customer_id column.

` ` `sql
SELECT c.name, o.id, o.total_amount
FROM customers c
JOIN orders o ON o.customer_id = c.id;
` ` `
```

### Supported File Formats

The KB builder processes documentation in several formats.
The builder supports the following file types:

- Markdown (`.md`) files work best for new documentation.
- HTML (`.html`, `.htm`) files support existing web
  documentation.
- reStructuredText (`.rst`) files support Sphinx-based
  documentation.
- SGML (`.sgml`, `.sgm`) files support PostgreSQL-style
  documentation.
- DocBook XML (`.xml`) files support XML-based documentation.

The builder converts all formats to Markdown before chunking
and embedding the content.

## Configuring the KB Builder

The KB builder reads a YAML configuration file that specifies
the documentation sources and the embedding providers. The
following steps walk you through the configuration process.

### Step-by-Step Configuration

Follow these steps to configure and run the KB builder.

1. Create a directory for your domain documentation:

    ```bash
    mkdir -p ~/my-project/docs
    ```

2. Write your documentation files as described in the
   previous section. Place the files in the `docs`
   directory you created.

3. Create the KB builder configuration file. In the
   following example, the configuration uses OpenAI for
   embeddings:

    ```yaml
    # Output database file path
    database_path: "my-project-kb.db"

    # Directory for storing processed documentation
    doc_source_path: "doc-source"

    # Documentation sources to process
    sources:
        - local_path: "~/my-project"
          doc_path: "docs"
          project_name: "My E-Commerce App"
          project_version: "1.0"

    # Embedding provider configuration
    embeddings:
        openai:
            enabled: true
            api_key_file: "~/.openai-api-key"
            model: "text-embedding-3-small"
            dimensions: 1536

        voyage:
            enabled: false

        ollama:
            enabled: false
    ```

4. Set up your API key file for the embedding provider:

    ```bash
    echo "sk-your-openai-key" > ~/.openai-api-key
    chmod 600 ~/.openai-api-key
    ```

5. Build the knowledge base by running the builder:

    ```bash
    ./pgedge-nla-kb-builder \
        --config my-kb-builder.yaml
    ```

The builder processes each documentation file, splits the
content into chunks, generates embeddings, and stores the
results in the output SQLite database.

### Using Multiple Sources

You can combine your custom documentation with standard
PostgreSQL documentation in a single knowledge base. The
following configuration includes both a local source and a
Git repository source:

```yaml
sources:
    # Your domain documentation
    - local_path: "~/my-project"
      doc_path: "docs"
      project_name: "My E-Commerce App"
      project_version: "1.0"

    # PostgreSQL documentation for SQL reference
    - git_url: "https://github.com/postgres/postgres.git"
      branch: "REL_17_STABLE"
      doc_path: "doc/src/sgml"
      project_name: "PostgreSQL"
      project_version: "17"
```

The builder clones the Git repository and processes both
sources into the same database. The LLM can then search
across all projects or filter by project name.

### Using Ollama for Local Builds

Ollama provides a local embedding option that requires no
API key. Use Ollama when you want to build knowledge bases
without sending data to external services.

The following configuration enables Ollama as the embedding
provider:

```yaml
embeddings:
    openai:
        enabled: false

    voyage:
        enabled: false

    ollama:
        enabled: true
        endpoint: "http://localhost:11434"
        model: "nomic-embed-text"
```

Before building, pull the embedding model with the following
command:

```bash
ollama pull nomic-embed-text
```

Then run the builder using the same build command described
in the step-by-step section.

## Configuring the MCP Server

After building the knowledge base, configure the MCP server
to load the database file. Add the `knowledgebase` section to
your server configuration file.

The following example shows the server configuration for a
custom knowledge base built with OpenAI embeddings:

```yaml
knowledgebase:
    enabled: true
    database_path: "./my-project-kb.db"
    embedding_provider: "openai"
    embedding_model: "text-embedding-3-small"
    embedding_openai_api_key_file: "~/.openai-api-key"
```

The embedding provider and model in the server configuration
must match the provider and model you used to build the
knowledge base. The server uses the same embedding model to
convert search queries into vectors for comparison.

For Ollama-based knowledge bases, use the following
configuration:

```yaml
knowledgebase:
    enabled: true
    database_path: "./my-project-kb.db"
    embedding_provider: "ollama"
    embedding_model: "nomic-embed-text"
```

## Testing and Verifying Your Knowledge Base

After deploying the knowledge base, verify that the server
loads the data correctly and returns relevant results. Follow
these steps to test the setup.

1. Start the MCP server with the custom knowledge base
   configuration.

2. List the available products in the knowledge base. Use
   `search_knowledgebase` with `list_products` set to
   `true`:

    ```json
    {
        "list_products": true
    }
    ```

3. Confirm that your project name appears in the product
   list. The output should include the project name and
   version you specified in the builder configuration.

4. Search for a domain-specific term to verify the content.
   In the following example, the search targets your custom
   project:

    ```json
    {
        "query": "how to calculate net revenue",
        "project_names": ["My E-Commerce App"]
    }
    ```

5. Verify that the results contain relevant chunks from
   your documentation.

6. Ask a natural language question through your MCP client
   and observe whether the LLM uses the knowledge base
   context to generate accurate SQL.

If the search returns no results, confirm that the embedding
provider in the server configuration matches the provider
you used during the build.

## Incremental Updates

The KB builder supports incremental processing when you
update your documentation. The builder reprocesses only the
files that changed since the last build.

Follow these steps to update an existing knowledge base.

1. Edit or add documentation files in your source directory.

2. Run the builder again with the same configuration file:

    ```bash
    ./pgedge-nla-kb-builder \
        --config my-kb-builder.yaml
    ```

The builder pulls the latest changes from Git repositories
and reprocesses only modified files. Unchanged files reuse
their existing chunks and embeddings.

Use the `--skip-updates` flag to skip Git pull operations
during development:

```bash
./pgedge-nla-kb-builder \
    --config my-kb-builder.yaml --skip-updates
```

If you enable a new embedding provider after the initial
build, use `--add-missing-embeddings` to generate embeddings
for the new provider without reprocessing documents:

```bash
./pgedge-nla-kb-builder \
    --config my-kb-builder.yaml \
    --add-missing-embeddings
```

## Knowledge Base Database Schema

The KB builder stores data in a SQLite database with a
single main table. You do not need to interact with the
database directly; the MCP server handles all queries.

The `chunks` table stores text chunks with metadata and
embeddings. The following SQL shows the table schema:

```sql
CREATE TABLE chunks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    text TEXT NOT NULL,
    title TEXT,
    section TEXT,
    project_name TEXT NOT NULL,
    project_version TEXT NOT NULL,
    file_path TEXT,
    openai_embedding BLOB,
    voyage_embedding BLOB,
    ollama_embedding BLOB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

The table includes the following indexes for fast filtering:

```sql
CREATE INDEX idx_project
    ON chunks(project_name, project_version);
CREATE INDEX idx_title ON chunks(title);
CREATE INDEX idx_section ON chunks(section);
```

Each embedding column stores a float32 array serialized as
a binary BLOB. The server deserializes the embeddings at
query time and computes similarity scores against the query
vector.

## Best Practices for Domain Documentation

Follow these guidelines to maximize the accuracy of natural
language queries against your custom knowledge base.

- Document every table with its purpose and column
  descriptions.
- Include example queries for common business questions.
- Define business terms and domain jargon in a glossary
  file.
- Document join patterns between related tables.
- Include sample data to illustrate expected column values.
- Keep the documentation current when the schema changes.
- Use one Markdown file per major topic or table group.
- Include both simple and complex query examples.
- Write clear column descriptions that distinguish
  similarly named columns.
- Document enum values and status codes with their
  meanings.

## See Also

The following pages provide additional reference material
for knowledge base configuration and usage.

- [Knowledgebase Configuration](knowledgebase.md) covers
  the server-side knowledge base settings.
- [KB Builder Configuration](../reference/config-examples/kb-builder.md)
  provides the complete builder configuration reference.
- [Available Tools](../reference/tools.md) documents all
  MCP tools including `search_knowledgebase`.
- [Querying Best Practices](../guide/querying.md)
  describes techniques for effective natural language
  queries.
