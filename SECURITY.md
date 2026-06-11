# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.1.x   | Yes       |

## Reporting a Vulnerability

Email security@artifact.dev (or open a private security advisory on GitHub).

We aim to respond within 48 hours. Please include:

- Description of the vulnerability
- Steps to reproduce
- Impact assessment

Do not disclose publicly until we've had a chance to patch.

## Security Model

- Artifact is designed for **internal-only** deployment behind corporate SSO
- Never expose Artifact to the public internet
- Header-trust mode requires proxy authentication
- Warehouse credentials are read-only with SELECT-only parser
- User uploads are served with restrictive CSP
