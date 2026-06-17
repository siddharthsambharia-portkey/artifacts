# Quickstart

Get Artifact running and deploy your first site. On a laptop this takes about a minute. You
need no cloud account, no database, and no API keys — dev mode uses SQLite, local-disk
storage, and a fake signed-in user.

## 1. Install

```bash
curl -fsSL https://raw.githubusercontent.com/siddharthsambharia-portkey/artifacts/main/scripts/install.sh | sh
```

Or build from a clone (requires Go 1.25+):

```bash
git clone https://github.com/siddharthsambharia-portkey/artifacts
cd artifacts
make build        # produces ./bin/artifact
```

In a clone you can also skip the build and run `go run ./cmd/artifact <command>` anywhere
this guide says `artifact <command>`.

## 2. Start a local server

```bash
artifact dev
```

This starts Artifact on `:8443` with dev defaults: `domain: localhost`, `auth: dev`, SQLite
at `.artifact-data/artifact.db`, and local-disk storage under `.artifact-data/`. In dev mode
you are always signed in as `dev@localhost`. Migrations run on boot.

Visit `http://localhost:8443` — the apex shows the home page (a searchable directory of
deployed sites). Any subdomain that has no site yet, e.g. `http://anything.localhost:8443`,
shows a "no such site" page.

## 3. Create and deploy a site

In a second terminal:

```bash
artifact init my-poll      # creates ./my-poll with index.html, artifact.json, and the agent skill
cd my-poll
artifact deploy            # uploads the folder; prints the live URL
artifact open --site my-poll
```

Open `http://my-poll.localhost:8443`. Edit `index.html` and run `artifact deploy` again to
update — deploys overwrite the site atomically (upload files → write manifest → flip
pointer), so a half-deployed site is never served.

> **No CLI?** Open `http://localhost:8443` and drag a folder, file, or `.zip` onto the page
> (drop-to-deploy). The same thing is available programmatically at
> `POST /api/v1/deploy`.

## 4. Use the SDK in your site

Every site loads the SDK from the same origin and talks to `/api/v1/*` with the session
cookie — no keys, no config:

```html
<!DOCTYPE html>
<html>
<head><script src="/artifact.js"></script></head>
<body>
  <h1>Lunch poll</h1>
  <ul id="list"></ul>
  <button id="pizza">Vote pizza</button>
  <script>
    await artifact.ready();
    const votes = artifact.db.collection('votes');
    const render = (docs) =>
      document.getElementById('list').innerHTML =
        docs.map(d => `<li>${d.data.option} — ${d.created_by}</li>`).join('');
    render(await votes.list({ order: '-created_at' }));
    votes.subscribe({ onCreate: async () => render(await votes.list({ order: '-created_at' })) });
    document.getElementById('pizza').onclick = () =>
      votes.create({ option: 'Pizza', voter: artifact.me.email });
  </script>
</body>
</html>
```

Open the site in two browser windows and watch votes sync in real time. See the full
[SDK reference](sdk-reference.md) for every method.

## Next steps

- Understand the model: [Concepts](concepts.md).
- Deploy for your team: [Self-hosting](self-hosting.md) and pick an [auth mode](auth-okta.md).
- Let a coding agent build sites: the skill in [`skills/`](../skills/) is dropped into every
  `artifact init` project.
