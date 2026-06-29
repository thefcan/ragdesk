# Deploying ragdesk

ragdesk ships its own delivery pipeline. Every push to `main` builds and
publishes both service images; from there you can run the stack anywhere that
takes a container, or one-click the included Render Blueprint.

## Continuous delivery — container images (GHCR)

The [`CD`](../.github/workflows/cd.yml) workflow builds and pushes the API and
AI images to the GitHub Container Registry on every push to `main` and on
version tags (`v*`). No extra account or cost — public packages.

```
ghcr.io/thefcan/ragdesk-api:latest      # also :sha-<commit> and :<semver>
ghcr.io/thefcan/ragdesk-ai:latest
```

Pull and run them anywhere:

```bash
docker pull ghcr.io/thefcan/ragdesk-api:latest
docker pull ghcr.io/thefcan/ragdesk-ai:latest
```

> First publish: the packages are created private. Make them public (or grant
> your host pull access) under the repo's *Packages* settings.

## One-click deploy — Render Blueprint ($0)

[`render.yaml`](../render.yaml) describes the full stack: Postgres (with
pgvector), a Redis-compatible Key Value store, the API, the AI service and the
web app — all on Render's free tier.

1. **Fork** this repo.
2. In Render: **New → Blueprint**, connect your fork. Render reads `render.yaml`
   and provisions every service.
3. `DATABASE_URL`, `REDIS_URL`, `JWT_SECRET` and the shared `AI_INTERNAL_TOKEN`
   are wired **automatically**.
4. After the first deploy, set these URL values (they depend on the hostnames
   Render assigns) and redeploy:

   | Service | Variable | Value |
   |---------|----------|-------|
   | ragdesk-api | `AI_SERVICE_URL` | `https://ragdesk-ai.onrender.com` |
   | ragdesk-api | `CORS_ALLOWED_ORIGINS` | `https://ragdesk-web.onrender.com` |
   | ragdesk-api | `WEB_BASE_URL` | `https://ragdesk-web.onrender.com` |
   | ragdesk-web | `NEXT_PUBLIC_API_URL` | `https://ragdesk-api.onrender.com` |

5. Open the web URL and register. 🎉

> Render's free Postgres is time-limited and free services sleep when idle —
> fine for a demo. For always-on, bump the relevant plans.

## Real LLM answers

The Blueprint defaults to the deterministic **`fake`** providers so it runs at
$0 with no model server. The AI layer is provider-agnostic
([`ai/app/embeddings.py`](../ai/app/embeddings.py),
[`ai/app/chat.py`](../ai/app/chat.py)): add a hosted provider (e.g. a free-tier
chat API for `CHAT_PROVIDER` and an embeddings API for `EMBEDDING_PROVIDER`),
set its env vars, and redeploy — no other code changes.

## Real billing (optional)

To switch billing from `$0` dev mode to Stripe **test mode**, set
`STRIPE_SECRET_KEY`, `STRIPE_PRICE_PRO` and `STRIPE_WEBHOOK_SECRET` on
`ragdesk-api`, then add a Stripe webhook endpoint pointing at
`https://ragdesk-api.onrender.com/billing/webhook`.

## Other hosts

The GHCR images run on any container platform — Fly.io, Railway, a VPS with
`docker compose`, or Kubernetes. Point each service at a Postgres (with
pgvector) and a Redis, mirror the env vars from
[`.env.example`](../.env.example), and you're up. The web app also deploys
cleanly to Vercel (set `NEXT_PUBLIC_API_URL` to your API URL).
