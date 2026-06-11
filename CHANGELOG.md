# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-06-11

### Added

- DB realtime subscribe wired to write-path events (live-poll updates across tabs)
- Admin API: audit search, AI usage, stats, config/quotas
- files.list() endpoint
- NATS pubsub adapter for multi-replica WebSocket broadcast
- Warehouse drivers: postgres, BigQuery, Snowflake (postgres-compatible DSN)
- OIDC sessions persisted in database
- Deploy manifest LRU cache
- AI per-user daily token quotas and model allowlist
- Warehouse per-user daily query quotas
- Governed mode ownership enforcement (tested)
- Single Go binary with dev, serve, deploy, init, list, open, logs, mcp, version commands
- Browser SDK (`artifact.js`) with db, kv, files, ai, warehouse, ws, notify, me
- Static site hosting with atomic manifest-pointer deploys
- Pluggable auth: dev, OIDC, header-trust
- Pluggable storage: local, S3, GCS
- SQLite (dev) and Postgres (production) database
- WebSocket hub with rooms and presence
- AI proxy (OpenAI-compatible upstream)
- Warehouse query API (SELECT-only)
- Slack notifications
- Trust and governed governance modes
- Admin console and home directory
- Agent skill files (SKILL.md, AGENTS.md)
- MCP server for agent integration
- Docker, Helm, Terraform examples
- Five example sites: guestbook, live-poll, team-dashboard, multiplayer-cursors, lunch-vote
