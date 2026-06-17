# CLI reference

`artifact` is a single Go binary. All subcommands accept the global `--config` flag described
below.

## Global flag

| Flag | Default | Description |
|---|---|---|
| `--config <path>` | `$ARTIFACT_CONFIG`, then dev defaults | Path to `artifact.yaml`. If neither the flag nor the env var is set, the binary falls back to dev defaults (SQLite + local disk + `dev@localhost`). |

---

## artifact serve

Start the Artifact server.

```bash
artifact serve [--config path/to/artifact.yaml]
```

Loads `artifact.yaml` from `--config` or `$ARTIFACT_CONFIG`. If neither is provided, falls
back to dev defaults (same as `artifact dev`). For production, always supply a config file.
Logs at INFO level to stdout. Shuts down gracefully on `SIGINT` or `SIGTERM`.

---

## artifact dev

Start a local development server with no config required.

```bash
artifact dev
```

Use on a laptop for local development. Falls back to dev defaults automatically, even if `--config` points to an unreadable file. Logs the server URL and identity on startup.

**Dev defaults**

| Setting | Value |
|---|---|
| Listen address | `:8443` |
| Domain | `localhost` (sites at `http://<site>.localhost:8443`) |
| Auth | `dev` — you are always signed in as `dev@localhost` |
| Database | SQLite at `.artifact-data/artifact.db` |
| Storage | Local disk under `.artifact-data/` |
| Governance | Trust mode |

Migrations run automatically on first boot.

---

## artifact deploy

Deploy a folder of static files.

```bash
artifact deploy [dir] [--site <name>] [--yes]
```

| | Default | Description |
|---|---|---|
| `dir` | `.` (current directory) | Directory of files to upload |
| `--site <name>` | Name from `artifact.json` in `dir`, else directory name | Site name to deploy to |
| `--yes` / `-y` | false | Overwrite an existing site without prompting |

If the site exists, the CLI prompts you to confirm. Pass `--yes` to skip.

> **Current behavior — deploys as `dev@localhost`**: `artifact deploy` opens storage and the
> database directly without going through the HTTP server. All deploys are recorded as
> `dev@localhost` regardless of the actual caller, and the server does not need to be running.
> For production, use the web UI (drag-and-drop on the home page) or `POST /api/v1/deploy`
> behind SSO so that deploys are attributed to the real employee and go through governed-mode
> checks. See [Quickstart](quickstart.md).

---

## artifact init

Create a new site directory with starter files.

```bash
artifact init [name]
```

| Argument | Default |
|---|---|
| `name` | `my-site` |

Creates `./<name>/` with:

| File | Contents |
|---|---|
| `index.html` | Minimal HTML that loads `/artifact.js` and displays the signed-in user's name |
| `artifact.json` | `{"site":"<name>"}` — read by `artifact deploy` for the site name |
| `AGENTS.md` | Site-builder agent skill (copied from `skills/AGENTS.md`; a stub if not found) |
| `CLAUDE.md` | Same site-builder skill, for Claude Code (copied from `skills/CLAUDE.md`) |

```bash
artifact init my-poll
cd my-poll
artifact deploy
```

---

## artifact login

```bash
artifact login
```

> **Current behavior — stub**: There is no device-code or browser-based authentication flow
> implemented yet. Running `artifact login` prints guidance:
>
> - **Dev mode** (`artifact dev`): you are always signed in as `dev@localhost`. No login step
>   is needed.
> - **Production**: visit your Artifact domain in a browser and sign in via your company's
>   SSO. The CLI does not manage credentials at this time.

---

## artifact list

List all deployed sites.

```bash
artifact list
```

Opens the database directly (the server does not need to be running). Prints one line per
site:

```
my-poll               2026-06-15 14:32  (dev@localhost, 41280 bytes)
team-blog             2026-06-14 09:10  (alice@corp.com, 183441 bytes)
```

---

## artifact open

Print the URL for a site.

```bash
artifact open --site <name>
artifact open <name>          # positional form also works
```

| Flag | Description |
|---|---|
| `--site <name>` | Site name (required) |

Constructs the URL from the config:

- Dev (`domain: localhost`): `http://<site>.localhost:8443`
- Production: `https://<site>.<domain>`

> **Current behavior**: This command prints the URL to stdout. It does not launch a browser
> process.

---

## artifact logs

View audit log entries.

```bash
artifact logs [--site <name>]
```

| Flag | Default | Description |
|---|---|---|
| `--site <name>` | (all sites) | Filter entries to a specific site |

Opens the database directly (the server does not need to be running). Prints up to 50 entries:

```
2026-06-15 14:32:01  dev@localhost  my-poll  deploy  41280 bytes, 12 files
```

---

## artifact mcp

Start the MCP (Model Context Protocol) server on stdio for coding-agent integration.

```bash
artifact mcp
```

Reads newline-delimited JSON requests from stdin and writes newline-delimited JSON responses
to stdout. Wire it into your agent's tool config (e.g. `claude_desktop_config.json`,
`.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "artifact": {
      "command": "artifact",
      "args": ["mcp"]
    }
  }
}
```

**Request / response shape**

```json
// request
{ "id": "1", "method": "call_tool", "params": { "name": "deploy_site", "dir": "./my-poll", "site": "my-poll" } }

// success
{ "id": "1", "result": { "url": "http://my-poll.localhost:8443", "site": "my-poll" } }

// error
{ "id": "1", "error": "unknown tool \"foo\": use deploy_site, list_sites, read_logs, or query_db" }
```

**Available tools**

| Tool | Params | Returns |
|---|---|---|
| `deploy_site` | `dir` (string, default `"."`) · `site` (string, default from dir name) | `{ url, site }` |
| `list_sites` | — | Array of site objects |
| `read_logs` | `site` (string) | Array of audit log entries (up to 50) |
| `query_db` | `site` (string) · `collection` (string) | Array of documents (up to 50) |

> **Current behavior — deploys as `dev@localhost`**: `deploy_site` uses the same direct-DB
> path as `artifact deploy` and records all deploys as `dev@localhost`. For production use,
> point the agent at `POST /api/v1/deploy` instead.

---

## artifact version

Print the binary version.

```bash
artifact version
# artifact v0.1.0
```
