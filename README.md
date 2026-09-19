# Xentra

Xentra is an AI-assisted DevOps investigation platform for safe, evidence-backed diagnostics and human-approved remediation across Linux and Docker environments.

## MVP flow

React UI -> Go control plane -> Go runner or pinned SSH -> Python investigation service -> incident timeline -> human approval -> typed remediation -> verification -> audit

## Structure

- ui/ — React + TypeScript operator dashboard
- backend/control-plane/ — deterministic Go orchestration/API layer
- backend/runner/ — Go environment discovery and allowlisted tool execution
- backend/ai-service/ — Python + FastAPI investigation intelligence
- contracts/ — shared schemas
- docs/ — architecture and plans

## AI investigation

The AI service supports an OpenAI-compatible provider configured through environment variables. When no provider is configured, or when a configured provider returns an invalid response, Xentra falls back to deterministic heuristics.

LLM evidence is bounded and schema-validated before a finding is returned.

## Safety

The AI never receives raw infrastructure credentials and never executes arbitrary shell commands. Environment access is performed through typed tools. State-changing actions are restricted to allowlisted operations, require explicit human approval, run post-action verification, and are written to the audit trail.


## Docker quick start

Production Compose is selected automatically from `.env` through `COMPOSE_FILE=docker-compose.prod.yml`.

```bash
cp .env.example .env
# Replace placeholder secrets/URLs in .env and provision the files under ./secrets.
docker compose up -d --build
docker compose ps
```

Core services use `restart: unless-stopped`, so after a Docker/host restart they come back automatically unless they were explicitly stopped.

For AWS SSM, production deployments should give the Xentra control plane an IAM role. Temporary `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION_TOKEN` values in the local git-ignored `.env` are supported only as a self-hosted/development fallback.

See `docs/deployment/production-compose.md` and `docs/aws-ssm.md` for production prerequisites.
