# Production Compose

The production Compose stack runs:

- `ui` — Nginx serving the React SPA and reverse proxying the user-facing `/api/` endpoints.
- `control-plane` — non-root distroless Go API plus the dedicated outbound Runner-control listener.
- `ai-service` — non-root FastAPI service.
- `postgres` — PostgreSQL 17 with pgvector.

The Xentra Runner is installed inside each target environment. The recommended `runner_outbound` transport does not require an inbound Runner port. The Runner connects outward to the dedicated control listener, polls typed tasks, executes only the allowlisted tool catalog, and posts results back.

## Production ports

- `8088` — bundled UI/API entry point. Put your normal public HTTPS ingress in front of this port.
- `8443` — dedicated Runner-control listener. This listener terminates TLS itself and requires a trusted client certificate plus the one-time Runner token.

Do not proxy Runner traffic through the regular user API listener.

## Required secrets

Copy `.env.example` to `.env`, replace every placeholder, and provide:

- `secrets/github_app_private_key.pem` when GitHub App mode is enabled.
- `secrets/runner_control_server_cert.pem` — TLS certificate whose SAN matches `XENTRA_PUBLIC_RUNNER_CONTROL_URL`.
- `secrets/runner_control_server_key.pem` — private key for that certificate.
- `secrets/runner_control_client_ca.pem` — CA used to verify target Runner client certificates.

Each installed Runner also needs its own client certificate/key signed by the configured client CA and a CA certificate that can verify the Runner-control server certificate.

## Setup

1. Copy `.env.example` to `.env` and replace every placeholder secret.
2. Set `XENTRA_ALLOWED_ORIGINS` to the public HTTPS UI origin.
3. Set `XENTRA_PUBLIC_RUNNER_CONTROL_URL` to the externally reachable mTLS endpoint, for example `https://xentra.example.com:8443`.
4. Provision the Runner-control server certificate/key and client CA files above.
5. Start with `docker compose --env-file .env -f docker-compose.prod.yml up -d --build`.
6. Put normal TLS termination in front of port 8088 for browser traffic. Port 8443 already terminates mutual TLS in the control plane.

The browser API trusts forwarded client IPs only because the bundled Nginx proxy is the direct upstream. If exposing the user API directly, set `XENTRA_TRUST_PROXY_HEADERS=false`.

The metrics endpoint is `GET /internal/metrics` and requires `Authorization: Bearer <XENTRA_METRICS_TOKEN>`.

## Legacy Runner transport

The older inbound Runner transport remains available for migration, but production Compose sets `XENTRA_LEGACY_RUNNER_ENABLED=false` by default. Re-enable it only while migrating existing environments and provide its separate legacy mTLS client credentials.
