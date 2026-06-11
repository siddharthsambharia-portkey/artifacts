# Artifact — Developer Guide

This project is an **Artifact site** — a static HTML app hosted on your company's internal platform.

## Rules

1. Load the SDK: `<script src="/artifact.js"></script>`
2. Always `await artifact.ready()` before using the SDK
3. Never add API keys, auth flows, or custom backends
4. Use `artifact.db`, `artifact.files`, `artifact.ai`, `artifact.ws`, `artifact.warehouse`, `artifact.notify` for all server needs
5. Deploy with `artifact deploy` when done

## Full SDK docs

See `SKILL.md` in this folder for complete API reference with examples.

## Deploy

```bash
artifact deploy          # from this directory
artifact deploy --yes    # skip overwrite confirmation
```

## Dev

```bash
artifact dev             # starts server — visit http://<site>.localhost:8443
```
