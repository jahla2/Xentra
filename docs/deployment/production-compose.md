# Production Compose

The production Compose stack runs:

- `ui` — Nginx serving the React SPA and reverse proxying `/api/`.
- `control-plane` — non-root distroless Go API.
- `ai-service` — non-root FastAPI service.
- `postgres` — PostgreSQL 17 with pgvector.

The Runner is intentionally not part of the central stack. Install it in the target environment and use mutual TLS in production. `XENTRA_RUNNER_INSECURE_DEV=true` in the central Compose file only allows existing SSH environments and local development-style Runner URLs; do not expose a plain-HTTP Runner publicly.

## Setup

1. Copy `.env.example` to `.env` and replace every placeholder secret.
2. Create `secrets/github_app_private_key.pem` when GitHub App mode is used.
3. Set `XENTRA_ALLOWED_ORIGINS` to the public HTTPS UI origin.
4. Start with `docker compose --env-file .env -f docker-compose.prod.yml up -d --build`.
5. Put TLS termination in front of port 8088 (load balancer, Caddy, Traefik, Nginx, or a cloud ingress).

The API trusts proxy headers only because the Compose topology places it behind the bundled Nginx proxy. If exposing the API directly, set `XENTRA_TRUST_PROXY_HEADERS=false`.

The metrics endpoint is `GET /internal/metrics` on the control plane and requires `Authorization: Bearer <XENTRA_METRICS_TOKEN>`.
