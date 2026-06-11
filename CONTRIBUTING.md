# Contributing to Artifact

## Dev setup

```bash
make dev      # start local server
make build    # compile binary
make test     # run tests
make sdk      # build browser SDK
```

## Pull requests

1. Fork and branch from `main`
2. Write table-driven tests for new handlers
3. Keep SQL compatible with SQLite + Postgres
4. Never trust client-supplied identity or site fields
5. Conventional commits (`feat:`, `fix:`, `docs:`)

## Code standards

- Interfaces only at seams (storage, db, auth, warehouse, notify)
- Every error message is a sentence with a fix
- SDK methods need doc comments + examples
