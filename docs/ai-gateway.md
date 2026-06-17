# AI gateway

`artifact.ai` is a thin, server-side proxy to any OpenAI-compatible upstream. The API key
never leaves the server. Sites call `artifact.ai.chat()` and `artifact.ai.image()` from the
browser with no credentials at all. Artifact handles authentication, rate limiting, quota
enforcement, and usage recording.

## How it works

```
browser  →  POST /api/v1/ai/chat  →  Artifact (adds API key + cost-attribution headers)
                                           │
                                           ▼
                                  upstream (OpenAI / Portkey / LiteLLM / ...)
```

Artifact forwards requests to exactly one upstream URL. The operator sets the URL; it is
never user-controlled, so crafted requests carry no SSRF risk.

## Configuration

```yaml
# artifact.yaml
ai:
  upstream_url: https://api.openai.com/v1  # required to enable AI
  api_key_env: ARTIFACT_AI_KEY             # default; name of env var holding the API key
  image_model: dall-e-3                    # optional; enables artifact.ai.image
  models_allowlist:                        # optional; restricts the model parameter
    - gpt-4o
    - gpt-4o-mini
```

| Field | Default | What it does |
|---|---|---|
| `upstream_url` | *(empty)* | Base URL of the OpenAI-compatible upstream. Required to enable AI. If empty, both endpoints return 503. |
| `api_key_env` | `ARTIFACT_AI_KEY` | Name of the environment variable holding the upstream API key. The key is never sent to the client. |
| `image_model` | *(empty)* | Model name to use for image generation. If empty, `POST /api/v1/ai/image` returns 503. |
| `models_allowlist` | *(empty list)* | If non-empty, the `model` field in chat requests must match one of these values. Requests with an unlisted model are rejected with 403. An empty list allows any model string. |

The actual API key goes in the environment variable named by `api_key_env`:

```bash
ARTIFACT_AI_KEY=sk-...your-key...
```

## Upstream examples

### OpenAI direct

```yaml
ai:
  upstream_url: https://api.openai.com/v1
  api_key_env: ARTIFACT_AI_KEY
  image_model: dall-e-3
  models_allowlist:
    - gpt-4o
    - gpt-4o-mini
```

```bash
ARTIFACT_AI_KEY=sk-proj-...
```

### Portkey

Portkey presents an OpenAI-compatible API and adds routing, fallbacks, and cost tracking.
Point `upstream_url` at your Portkey gateway and set the API key:

```yaml
ai:
  upstream_url: https://api.portkey.ai/v1
  api_key_env: ARTIFACT_AI_KEY
```

```bash
ARTIFACT_AI_KEY=pk-...your-portkey-key...
```

Portkey uses the `Authorization: Bearer` header that Artifact already sends, so no
additional headers are required.

### LiteLLM

LiteLLM exposes an OpenAI-compatible endpoint that proxies to 100+ model providers. Run
it as a sidecar or on a shared host:

```yaml
ai:
  upstream_url: http://litellm:4000/v1
  api_key_env: ARTIFACT_AI_KEY
  models_allowlist:
    - gpt-4o
    - claude-3-5-sonnet
    - gemini/gemini-2.0-flash
```

```bash
ARTIFACT_AI_KEY=your-litellm-master-key
```

### Bedrock-compatible gateway

Any gateway that exposes an OpenAI-compatible HTTP API — such as a custom proxy or
AWS Bedrock's OpenAI-compatible endpoint — works the same way:

```yaml
ai:
  upstream_url: https://bedrock-runtime.us-east-1.amazonaws.com/v1
  api_key_env: ARTIFACT_AI_KEY
```

```bash
ARTIFACT_AI_KEY=your-aws-or-gateway-key
```

## Cost attribution headers

On every upstream request, Artifact sets two headers for cost attribution:

| Header | Value |
|---|---|
| `x-artifact-user` | The authenticated user's email (`artifact.me.email`) |
| `x-artifact-site` | The site name derived from the request host |

These headers appear in upstream dashboards that support custom headers (Portkey, LiteLLM,
and others). The server stamps them from the authenticated session and the request hostname
— they are not forwarded from the client and cannot be spoofed.

## Rate limits and quota

**Per-server rate limit:** `POST /api/v1/ai/chat` and `POST /api/v1/ai/image` share one
rate limiter at **5 requests per second with a burst of 10**, keyed per authenticated user.
Requests that exceed the limit receive 429.

**Per-user daily quota:** set `ai_daily_calls_per_user` in the `governance.quotas` block:

```yaml
governance:
  quotas:
    ai_daily_calls_per_user: 200   # 0 = unlimited (default)
```

The quota counts all AI calls (chat + image) per user in a rolling 24-hour window. A user
who hits their quota receives 429 with a message explaining when the window resets. Set to
`0` (the default) to disable the quota.

See [Governance & admin](governance-and-admin.md) for all quota fields.

## Endpoints

### `POST /api/v1/ai/chat`

Proxies a chat-completions request to `<upstream_url>/chat/completions`. Supports streaming
(`"stream": true`).

Request body (OpenAI chat format):

```json
{
  "messages": [
    { "role": "user", "content": "Summarise last week's incidents." }
  ],
  "model": "gpt-4o",
  "stream": true
}
```

- If `models_allowlist` is non-empty and `model` is not in the list, returns 403.
- If `upstream_url` is not configured, returns 503.
- Streams the upstream response verbatim as `text/event-stream` when `stream: true`.

### `POST /api/v1/ai/image`

Proxies an image-generation request to `<upstream_url>/images/generations` using
`image_model` as the model. Requires `ai.image_model` to be set; returns 503 otherwise.

Request body:

```json
{ "prompt": "An isometric diagram of a deployment pipeline" }
```

Returns the upstream JSON response (typically an array of image URLs or base64 data).

## SDK usage

Sites call `artifact.ai` from the browser — no API key required:

```javascript
// Streaming chat
const stream = await artifact.ai.chat([
  { role: 'user', content: 'Explain this chart in plain English.' }
], { model: 'gpt-4o', stream: true });

// Image generation (requires ai.image_model to be set)
const result = await artifact.ai.image('A retro pixel-art dashboard icon');
```

See the [SDK reference](sdk-reference.md) for the full method signatures.
