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
