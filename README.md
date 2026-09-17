# Xentra

Xentra is an AI-assisted DevOps investigation platform for safe, evidence-backed diagnostics across Linux and Docker environments.

## MVP flow

`React UI → Go control plane → Go runner → Python investigation service → evidence-backed finding`

## Structure

- `ui/` — React + TypeScript operator dashboard
- `backend/control-plane/` — deterministic Go orchestration/API layer
- `backend/runner/` — Go environment discovery and allowlisted tool execution
- `backend/ai-service/` — Python + FastAPI investigation intelligence
- `contracts/` — shared schemas
- `docs/` — architecture and plans

## Safety

The AI never receives raw infrastructure credentials and never executes arbitrary shell commands. All environment access is performed through typed runner tools. Mutation tools will require policy checks and explicit human approval in a later milestone.
