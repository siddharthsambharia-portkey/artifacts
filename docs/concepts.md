# Concepts

## Trust bubble

Artifact runs inside your corporate network. Every HTTP request is from an authenticated employee. That single assumption removes auth code, API keys, and spam defense from every internal app.

## Sites

A site is a folder of static files. `mysite.artifact.corp.com` maps to `sites/mysite/` in object storage. Deploy with `artifact deploy` — overwrite to update.

## SDK

`<script src="/artifact.js"></script>` gives every site database, files, AI, warehouse, websockets, identity, and Slack — zero config, zero keys.

## Trust vs governed mode

- **Trust** (default): all sites open to all employees, anyone can overwrite anything
- **Governed**: first deployer owns the site, redeploy restricted, audit log searchable in admin console

## Constraints

No custom backends, no cron, no per-site secrets — ever. When something feels missing, combine two SDK primitives instead.
