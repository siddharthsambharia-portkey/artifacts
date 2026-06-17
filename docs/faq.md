# FAQ / philosophy

## Is Artifact a fit for me?

Artifact is designed for one thing: **internal tools built and used by employees of the same
company**. If your use case fits inside that boundary, it is likely a good fit. If it does
not, Artifact is the wrong tool — and that is intentional.

A quick checklist:

| Use case | Fit? |
|---|---|
| Dashboard or admin panel for your ops or eng team | Yes |
| Internal poll, sign-up form, or lightweight app | Yes |
| Shared internal wiki, changelog, or knowledge base | Yes |
| Internal tool that reads your data warehouse | Yes |
| Customer-facing web app | No — sites are never exposed to the public internet |
| App that needs custom server-side logic or functions | No — there are no per-site backends |
| App that sends email or runs on a schedule | No — see the "not a fit" examples below |

---

## Why is it safe with no API keys?

The safety comes from the *trust bubble*. Every request reaching Artifact has already been
authenticated as a known employee — by Artifact's own OIDC login or by an identity proxy
(IAP, Pomerium, oauth2-proxy) in front of it. Because every caller is a verified employee,
Artifact can expose a database, file storage, and AI proxy directly to the browser — no
per-site API keys and no per-request auth code.

That single guarantee lets Artifact eliminate the hard 90% of a typical backend platform:
no signup, no multi-tenant isolation, no per-site credentials. The browser SDK calls
`/api/v1/*` with the session cookie, and the server stamps identity and the site name from
the request itself.

The trade-off is explicit: **Artifact must never be placed on the public internet.** Public
exposure breaks the guarantee entirely — anonymous users would be treated as trusted
employees. See [Concepts](concepts.md#2-the-trust-bubble).

---

## Why no custom backends?

The no-backends constraint is the product, not a missing feature.

Adding per-site backends means per-site secrets, per-site deployment pipelines, per-site
runtime environments, and per-site blast radius. Every internal-tool platform that has gone
down that road ends up as a miniature cloud. It becomes complex to operate, difficult to
audit, and impossible to secure without a dedicated team. Artifact refuses that trade-off.

The zero-config browser API covers the large majority of internal tool needs. It handles
reading and writing data, uploading files, streaming AI responses, real-time collaboration,
and Slack notifications. If your tool needs something outside that set, the answer is usually
a thin dedicated service that your Artifact site calls over the network.

### Not-a-fit examples

These three use cases are explicitly out of scope (from ADR-0002):

| Use case | Why it is out of scope | Alternative |
|---|---|---|
| **Sending email** | Requires a server-side email provider credential (a per-site secret) and a background delivery queue — exactly the kind of per-site backend Artifact refuses to host. | Use `artifact.notify.slack` for in-app notifications, or build a thin email-sending service and call it from your site. |
| **Scheduled / cron jobs** | Artifact has no job scheduler, no worker process, and no wake-up mechanism. Sites only execute code in response to browser requests. | Run a separate scheduled function (Cloud Scheduler, GitHub Actions on a schedule, a cron daemon) that drives any background work. |
| **Calling internal APIs with secret tokens** | There are no per-site environment variables or secrets. A site cannot hold a token the server injects into outbound calls — not a fit, by design. | Build a small backend service that holds the token and exposes a minimal authenticated endpoint; call that from your site. For read-only data, consider piping it through `artifact.warehouse`. |

---

## Trust mode vs governed mode

The difference is a single config toggle (`governance.mode`). See
[Concepts](concepts.md#5-trust-mode-vs-governed-mode) for the full description.

Short version:

- **Trust mode** (default): all employees can read and deploy all sites; no ownership,
  no visibility scoping. Good for small teams and early experimentation. The audit log still
  records every deploy and destructive call.
- **Governed mode**: the first deployer owns the site, visibility can be scoped to groups,
  deletion is protected, and the admin console exposes audit search, quotas, and usage.

When in doubt, start in trust mode and switch to governed mode when your team starts caring
about ownership or access control. Governance is middleware over nullable columns. Trust mode
is governed mode with every check returning "allow." There is no data migration when you
switch.

See [Governance & admin](governance-and-admin.md) for the full operator guide.

---

## Does it work offline or on a laptop?

Yes. `artifact dev` starts a fully functional server with no external dependencies:

| Component | Dev-mode setup |
|---|---|
| Database | SQLite at `.artifact-data/artifact.db` |
| Storage | Local disk under `.artifact-data/` |
| Auth | Dev mode — you are automatically signed in as `dev@localhost` |
| AI / Slack / warehouse | Disabled by default; enable individually in config when needed |

Everything in the [Quickstart](quickstart.md) works without a network connection once the
binary is installed. There is no cloud account, no API key, and no sign-up required.

---

## Why is there no MCP server (yet)?

The short answer: the auth model a *hosted* MCP server would require is a much bigger
commitment than the feature is worth today, and the cheap version of MCP is already covered
by what Artifact ships.

There are two shapes of MCP, and they have very different costs:

- **Local agents** (Cursor, Claude Code, a locally-run Codex): the agent runs on the user's
  own machine, inside the trust bubble, and can hold a credential. This already works with the
  service-account Bearer tokens from
  [ADR-0005](adr/0005-programmatic-deploy-tokens.md) — a token in the agent's MCP config
  authenticates `deploy`/`list` calls against `/api/*` with no SSO redirect, behind a proxy or
  in native-OIDC mode. A thin MCP wrapper over the existing deployer is days of work, not a
  new auth model.
- **Hosted assistants** (ChatGPT.com, Claude.ai): the agent runs on a vendor's servers,
  *outside* the trust bubble. A request from OpenAI's or Anthropic's cloud is, by definition,
  not a verified employee, so it is bounced at the identity proxy — the same `302`-to-login
  problem the deploy API had before tokens. Making this work requires turning Artifact into a
  full **OAuth 2.0 authorization server**: discovery endpoints, dynamic client registration,
  a consent screen, PKCE token exchange, refresh/revocation, and bridging the existing IdP
  into the consent step. That is weeks of security-critical code.

So the policy is the same "say no until it's real" discipline as custom backends and cron:

- The **local-agent** path is the Shopify Quick model (`quick init` drops skills + MCP into a
  local agent that is already behind IAP). It needs no new auth and stays inside the trust
  bubble. When we ship MCP, this is the version we ship first.
- The **hosted-assistant** path is the only thing that forces OAuth, and it punches a
  deliberate hole in the trust bubble — an external service acting on a user's behalf. We will
  not build that on a hypothesis. It needs a real, recurring "I must connect from
  ChatGPT.com and will not run a local agent" requirement to justify the cost and the new
  attack surface.

If you want agent-driven deploys today, point a local agent at the deploy API with a
service-account token. That covers the use case without any of the hosted-OAuth machinery.

---

## What if I need public access?

Artifact does not support it and is not designed for it. Placing Artifact on the public
internet breaks the trust bubble — unauthenticated visitors would be treated as trusted
employees. Artifact has no user sign-up, no per-user rate limiting for anonymous traffic,
and no multi-tenant isolation. None of that is necessary inside a company network.

If you need a public-facing site, Artifact is the wrong tool. Use a standard static host
(Vercel, Cloudflare Pages, S3 + CloudFront) for the public layer. Your internal Artifact
sites can then feed data into it through an API you control.
