# Quickstart

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/siddharthsambharia-portkey/artifacts/main/scripts/install.sh | sh
```

## Run locally

```bash
artifact dev
```

Visit `http://localhost:8443` for the site directory.

## Create and deploy a site

```bash
artifact init my-app
cd my-app
artifact deploy
open http://my-app.localhost:8443
```

Prefer the browser? Open `http://localhost:8443` and drag a folder, file, or zip onto the home page to deploy without the CLI.

## Production

Copy `artifact.yaml`, set `auth.mode: oidc`, configure storage and database, then:

```bash
artifact serve --config artifact.yaml
```

Or `docker compose up` / `helm install` — see deploy guides.
