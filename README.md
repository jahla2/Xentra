# Xentra

Xentra is an AI-assisted DevOps investigation platform focused on safe infrastructure visibility and evidence-backed incident diagnosis.

## MVP architecture

- `ui/` — React + TypeScript dashboard
- `backend/control-plane/` — Go API and orchestration layer
- `backend/ai-service/` — Python + FastAPI investigation intelligence
- `backend/runner/` — Go environment runner for Linux/on-prem execution
- `contracts/` — shared request/response schemas
- `infra/` — local infrastructure assets
- `docs/` — architecture and implementation plans

## Safety principle

The AI can investigate automatically, but infrastructure changes must pass through typed tools, policy checks, and explicit approval.

## Development

Each service owns its dependencies and tests. The integration branch is `development`; feature work is merged there through pull requests.
