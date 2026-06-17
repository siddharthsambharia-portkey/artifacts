# Warehouse

`artifact.warehouse` gives every internal site read-only access to your company's analytics
data (BigQuery, Snowflake, or Postgres) from the browser. The connection, credentials, and
query constraints live entirely on the server. Sites submit a SQL `SELECT` and get back rows.
They cannot modify data, access datasets outside the allowlist, or exceed the row or daily
limits you configure.

## How it works

```
browser  →  POST /api/v1/warehouse/query  →  Artifact
                                                │  validates SQL (SELECT-only)
                                                │  checks dataset allowlist
                                                │  enforces row limit
                                                ▼
                                        BigQuery / Snowflake / Postgres
                                         (read-only credentials)
```

Artifact enforces three independent defenses before any SQL reaches the database:

1. **SELECT-only parser** — the query must start with `SELECT`. Keywords like `INSERT`,
   `UPDATE`, `DELETE`, `DROP`, `ALTER`, `TRUNCATE`, `MERGE`, `EXEC`, `UNION`, and others
   are blocked. Multi-statement separators (`;`), comments (`--`, `/*`), and `INTO` are
   also rejected.
2. **Dataset allowlist** — the query must reference at least one dataset listed in
   `allowed_datasets`. An empty allowlist denies all queries.
3. **Row limit** — results are capped at `row_limit` rows. If your query has no `LIMIT`
   clause, Artifact appends one automatically.

Use read-only database credentials — a read-only user, a service account with
`roles/bigquery.dataViewer`, or a Snowflake role with only `SELECT` grants. The parser is a
secondary defense, not a replacement for proper credential scoping.

## Configuration

```yaml
# artifact.yaml
warehouse:
  driver: bigquery               # bigquery | snowflake | postgres | none (default)
  credentials_env: ARTIFACT_WAREHOUSE_CREDS  # name of env var holding credentials
  allowed_datasets:
    - analytics.events
    - analytics.sessions
    - finance.monthly_summary
  row_limit: 10000               # default; max rows returned per query
```

| Field | Default | What it does |
|---|---|---|
| `driver` | `none` | Which database backend to use. `none` disables the endpoint entirely. |
| `credentials_env` | *(empty)* | Name of the environment variable holding the credentials. Format depends on the driver (see below). |
| `allowed_datasets` | *(empty list)* | Datasets the proxy will permit queries against. **An empty list denies every query.** Must be non-empty to allow any access. |
| `row_limit` | `10000` | Maximum rows returned per query. Artifact appends `LIMIT <row_limit>` if the query has no `LIMIT` clause. |

### When driver is `none`

`POST /api/v1/warehouse/query` returns 503. No credentials are loaded. This is the default;
set a real driver to enable the feature.

## Credentials per driver

All credentials are passed via a single environment variable named by `credentials_env`
(default name shown in `.env.example` is `ARTIFACT_WAREHOUSE_CREDS`).

### BigQuery

BigQuery uses Application Default Credentials (ADC). Set `GOOGLE_APPLICATION_CREDENTIALS`
to the path of a service account JSON key, or run on GCE/GKE where the metadata server
supplies credentials automatically. The `credentials_env` field is not used by the BigQuery
driver; ADC is the mechanism.

Also set the project:

```bash
ARTIFACT_BIGQUERY_PROJECT=my-gcp-project   # or GOOGLE_CLOUD_PROJECT
```

```yaml
warehouse:
  driver: bigquery
  allowed_datasets:
    - my_dataset.my_table
    - analytics.events
  row_limit: 5000
```

The BigQuery service account needs `roles/bigquery.dataViewer` on the datasets you list in
`allowed_datasets`.

### Snowflake

The Snowflake driver accepts a **postgres-compatible DSN** via `credentials_env`:

```bash
ARTIFACT_WAREHOUSE_CREDS=postgres://my_user:my_password@my_account.snowflakecomputing.com/my_db
```

```yaml
warehouse:
  driver: snowflake
  credentials_env: ARTIFACT_WAREHOUSE_CREDS
  allowed_datasets:
    - ANALYTICS.PUBLIC.EVENTS
  row_limit: 10000
```

Use a Snowflake role that has only `SELECT` privileges on the datasets in `allowed_datasets`.

### Postgres

The Postgres driver accepts a standard connection DSN:

```bash
ARTIFACT_WAREHOUSE_CREDS=postgres://readonly_user:password@warehouse-host:5432/analytics?sslmode=require
```

```yaml
warehouse:
  driver: postgres
  credentials_env: ARTIFACT_WAREHOUSE_CREDS
  allowed_datasets:
    - public.events
    - reporting.monthly
  row_limit: 10000
```

Use a Postgres role with only `SELECT` grants (`CREATE ROLE readonly LOGIN; GRANT SELECT ON ALL TABLES IN SCHEMA public TO readonly;`).

## Dataset allowlist

`allowed_datasets` performs a case-insensitive substring check against the full query text.
A query passes if the lowercased SQL contains at least one of the lowercased dataset strings.

- **Empty list → deny all.** An empty or omitted `allowed_datasets` rejects every query
  with 403.
- Strings can be fully qualified (`project.dataset.table`) or partial
  (`analytics.events`). Use the most specific prefix that covers the tables you want to
  allow without accidentally matching unintended names.

Example: `allowed_datasets: [analytics]` permits any query containing the word `analytics`,
including `SELECT * FROM analytics.events` and `SELECT * FROM analytics.sessions`. Use
fully qualified names to be more restrictive.

## Rate limits and quota

**Per-server rate limit:** `POST /api/v1/warehouse/query` is rate-limited at **2 requests
per second with a burst of 5**, keyed per authenticated user. Requests that exceed the limit
receive 429.

**Per-user daily quota:** set `warehouse_daily_queries_per_user` in the `governance.quotas`
block:

```yaml
governance:
  quotas:
    warehouse_daily_queries_per_user: 200   # 0 = unlimited
```

The default is `200`. Set to `0` to disable the quota. Users who hit their quota receive
429 with a message explaining when the window resets.

See [Governance & admin](governance-and-admin.md) for all quota fields.

## Endpoint

### `POST /api/v1/warehouse/query`

Request body:

```json
{ "sql": "SELECT user_id, count(*) AS events FROM analytics.events WHERE date = '2026-06-16' GROUP BY 1 ORDER BY 2 DESC" }
```

Response:

```json
{
  "rows": [
    { "user_id": "u_123", "events": 47 },
    { "user_id": "u_456", "events": 31 }
  ],
  "truncated": false
}
```

`truncated: true` means the result was capped at `row_limit`. Add a `LIMIT` clause smaller
than `row_limit` to avoid truncation on large result sets.

Every successful query is recorded in the audit log as action `warehouse_query` with a
truncated copy of the SQL (first 200 characters). See
[Governance & admin](governance-and-admin.md) for how to query the audit log.

## SDK usage

```javascript
const { rows, truncated } = await artifact.warehouse.query(
  `SELECT product, sum(revenue) AS total
   FROM finance.monthly_summary
   WHERE month >= '2026-01-01'
   GROUP BY 1
   ORDER BY 2 DESC
   LIMIT 20`
);
```

See the [SDK reference](sdk-reference.md) for the full method signature.
