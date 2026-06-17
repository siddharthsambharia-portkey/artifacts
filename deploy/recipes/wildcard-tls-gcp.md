# Wildcard TLS via cert-manager DNS-01 (GCP profile)

Artifact's subdomain-per-site model (`my-site.<domain>`) requires a wildcard TLS certificate
for `*.<domain>`. This recipe explains why HTTP-01 cannot satisfy that requirement and walks
through the full DNS-01 setup using cert-manager and Cloud DNS on the GCP + Okta profile.

---

## Why DNS-01, not HTTP-01

The ACME HTTP-01 challenge works by placing a token at a well-known path on the exact domain
being validated. That means it can only validate `artifact.corp.example.com` — it cannot prove
ownership of `*.artifact.corp.example.com`. The ACME protocol explicitly [prohibits issuing
wildcard certificates over HTTP-01](https://datatracker.ietf.org/doc/html/rfc8555#section-10.2).

DNS-01 proves ownership by placing a `_acme-challenge.<domain>` TXT record in your DNS zone.
Because you control the zone, the challenge can be satisfied for both the apex and the wildcard
at once. This is the only way to obtain a `*.<domain>` certificate from Let's Encrypt or any
ACME CA.

---

## Prerequisites

- cert-manager ≥ 1.12 installed in the cluster.
- The `artifact.corp.example.com` zone hosted in **Cloud DNS** (the guaranteed GCP profile
  uses Cloud DNS by default via the GKE node pool's DNS zones; adjust the project/zone below).
- A GCP service account key (or Workload Identity binding) that has the
  `dns.resourceRecordSets.*` permissions on the zone. The narrowest built-in role is
  `roles/dns.admin`; create a custom role with only `dns.resourceRecordSets.*` for least
  privilege.

### 1. Create the Cloud DNS solver service account

```bash
gcloud iam service-accounts create cert-manager-dns01 \
  --display-name="cert-manager Cloud DNS solver" \
  --project=my-corp-project

gcloud projects add-iam-policy-binding my-corp-project \
  --member="serviceAccount:cert-manager-dns01@my-corp-project.iam.gserviceaccount.com" \
  --role="roles/dns.admin"

# Export the key for use as a Kubernetes Secret
gcloud iam service-accounts keys create /tmp/clouddns-sa-key.json \
  --iam-account=cert-manager-dns01@my-corp-project.iam.gserviceaccount.com
```

### 2. Store the key in Kubernetes

```bash
kubectl create secret generic cert-manager-clouddns-sa \
  --namespace cert-manager \
  --from-file=key.json=/tmp/clouddns-sa-key.json
rm /tmp/clouddns-sa-key.json
```

---

## Configure the ClusterIssuer

```yaml
# deploy/recipes/letsencrypt-clouddns-issuer.yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-clouddns
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: ops@corp.example.com       # receives cert expiry warnings
    privateKeySecretRef:
      name: letsencrypt-clouddns-acme-key
    solvers:
      - dns01:
          cloudDNS:
            project: my-corp-project
            serviceAccountSecretRef:
              name: cert-manager-clouddns-sa
              key: key.json
```

Apply it:

```bash
kubectl apply -f deploy/recipes/letsencrypt-clouddns-issuer.yaml
kubectl describe clusterissuer letsencrypt-clouddns
# Status.Conditions should show "Ready: True"
```

---

## Request the certificate via the Helm chart

Set `certificate.enabled=true` in `values-gcp.yaml` (see the commented block) and update
`ingress.hosts` / `ingress.tls` to use the wildcard host and TLS secret. Or pass flags
directly:

```bash
helm upgrade --install artifact ./deploy/helm/artifact/ \
  -f deploy/helm/artifact/values-gcp.yaml \
  --set config.domain=artifact.corp.example.com \
  --set certificate.enabled=true \
  --set certificate.clusterIssuer=letsencrypt-clouddns \
  --set certificate.secretName=artifact-wildcard-tls \
  --set "ingress.hosts[0].host=*.artifact.corp.example.com" \
  --set "ingress.hosts[0].paths[0].path=/" \
  --set "ingress.hosts[0].paths[0].pathType=Prefix" \
  --set "ingress.hosts[1].host=artifact.corp.example.com" \
  --set "ingress.hosts[1].paths[0].path=/" \
  --set "ingress.hosts[1].paths[0].pathType=Prefix" \
  --set "ingress.tls[0].hosts[0]=*.artifact.corp.example.com" \
  --set "ingress.tls[0].hosts[1]=artifact.corp.example.com" \
  --set "ingress.tls[0].secretName=artifact-wildcard-tls"
```

The Helm chart renders a `cert-manager.io/v1 Certificate` resource that covers:

```
dnsNames:
  - artifact.corp.example.com
  - *.artifact.corp.example.com
```

cert-manager picks it up, triggers a DNS-01 challenge via the Cloud DNS solver, and writes the
resulting TLS keypair into the `artifact-wildcard-tls` Secret. The Ingress then references that
Secret for TLS termination.

### Verify the certificate is issued

```bash
kubectl describe certificate <release>-wildcard
# Status should show "Certificate is up to date and has not expired"

kubectl get secret artifact-wildcard-tls -o jsonpath='{.data.tls\.crt}' \
  | base64 -d | openssl x509 -noout -subject -san
# Subject: CN=artifact.corp.example.com
# SAN: DNS:artifact.corp.example.com, DNS:*.artifact.corp.example.com
```

---

## Values reference

| Key | Default | Purpose |
|-----|---------|---------|
| `certificate.enabled` | `false` | Render the cert-manager `Certificate` resource |
| `certificate.clusterIssuer` | `letsencrypt-dns01` | Name of the `ClusterIssuer` to reference |
| `certificate.secretName` | `<release>-wildcard-tls` | TLS secret written by cert-manager and read by the Ingress |

---

## Cross-subdomain SSO

A wildcard cert ensures all subdomains are served over HTTPS; the session cookie also needs
to span them. Artifact handles this in both auth modes:

### Native OIDC (`auth.mode: oidc`)

Artifact already sets the session cookie with `Domain=.<domain>` (the leading dot makes it
valid on all subdomains). No configuration is required. The relevant code is in
`internal/auth/oidc.go` (`CallbackHandler`):

```go
if domain != "localhost" {
    cookie.Domain = "." + domain
}
```

After a user logs in at `admin.artifact.corp.example.com` the `artifact_session` cookie is
sent for every request to `*.artifact.corp.example.com`, so navigating to a different site
subdomain does not require a new login.

### Header-trust (`auth.mode: header-trust`) with oauth2-proxy

When oauth2-proxy fronts Artifact, its own session cookie must also span subdomains. Add
these settings to `oauth2-proxy.cfg`:

```ini
# oauth2-proxy.cfg — cross-subdomain session
cookie_domain   = .artifact.corp.example.com   # leading dot covers all subdomains
whitelist_domain = .artifact.corp.example.com  # allow redirects to any subdomain

# Forward identity headers to Artifact
set_xauthrequest = true
scope = openid email profile groups
pass_request_headers = X-Artifact-Proxy-Auth=<your-proxy-shared-secret>
```

| Setting | Value | Purpose |
|---------|-------|---------|
| `cookie_domain` | `.<domain>` (leading dot) | oauth2-proxy session cookie is valid on all subdomains |
| `whitelist_domain` | `.<domain>` | Permits post-login redirects back to any site subdomain |

> **Why the leading dot matters:** A cookie set for `artifact.corp.example.com` (no dot) is
> sent only to that exact hostname. A cookie set for `.artifact.corp.example.com` is sent to
> the domain and all its subdomains — which is what Artifact's subdomain-per-site model needs.

For further oauth2-proxy integration details see [Auth — header-trust](../docs/auth-header-trust.md).
