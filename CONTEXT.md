# Artifact

Artifact is an internal hosting platform: authenticated employees deploy folders of
static files that become live sites on a subdomain, backed by a small set of
server-held SDK primitives (db, kv, files, ai, warehouse, ws, notify).

## Language

**Deployment Profile**:
The one configuration combination Artifact guarantees works end-to-end —
documented, tested, and kept honest. The current profile is GCP + Okta:
`auth.mode: oidc` (Okta), `storage.driver: gcs`, `database.driver: postgres`,
`warehouse.driver: bigquery`, `notify.slack.mode: webhook`. Anything outside the
profile is either trimmed or shipped later as an explicit feature.
_Avoid_: deployment target, environment, config preset

**Site**:
A deployed folder of static files served on its own subdomain. The site namespace
is derived from the host by the server, never sent by the client.
_Avoid_: app, project, deployment

**Operator**:
The person who self-hosts and configures an Artifact instance (sets the config,
chooses the deployment profile, holds the server-side secrets).
_Avoid_: admin (admin is a governed-mode role, not the host), host, sysadmin

**Builder**:
An authenticated employee who deploys and changes sites via the CLI, the drop-to-deploy
UI, or the HTTP deploy API.
_Avoid_: developer, user, deployer

**Trust mode**:
Governance mode where every employee can read and write every site. The default.
_Avoid_: open mode, public mode

**Governed mode**:
Governance mode where sites have owners, visibility, and group-scoped access, and
admins are distinguished from ordinary Builders.
_Avoid_: secure mode, private mode, locked mode

**Native OIDC** (`auth.mode: oidc`):
Auth mode where Artifact is itself the OIDC client — it runs the login flow and reads
identity and groups from the ID token. The recommended default; the only mode that
delivers governed mode and admins today.
_Avoid_: SSO mode, OAuth mode

**Header-trust** (`auth.mode: header-trust`):
Auth mode where an identity proxy authenticates the user and forwards their identity in
trusted HTTP headers (guarded by a shared secret). Supported for proxy-fronted and
GCP-without-Okta deployments; out of the guaranteed profile.
_Avoid_: proxy mode, trusted-header mode

**Identity proxy**:
An external gateway (oauth2-proxy, Pomerium, Google IAP, ZTNA) that authenticates users
and fronts Artifact in header-trust mode. Distinct from the Operator and from Artifact.
_Avoid_: gateway, auth proxy
