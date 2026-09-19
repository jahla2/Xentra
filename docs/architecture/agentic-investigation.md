# Agentic Investigation Loop

Xentra investigations iterate over bounded, typed, read-only diagnostic tools before producing a final finding.

## Execution flow

1. Collect the five baseline diagnostics concurrently: system info, disk, CPU, memory, and Docker list.
2. Redact secrets and bound retained evidence before sending anything to the AI service.
3. Stream collected evidence and reasoning stages to the authenticated UI.
4. Ask the AI service whether the evidence is sufficient.
5. If more evidence is needed, the AI may request up to three environment-specific read-only typed tools in one reasoning step.
6. The Go control plane validates every request and rejects unavailable, malformed, duplicate, or mutation tools.
7. Independent requested tools execute concurrently.
8. Every result is redacted and bounded before it is returned to the AI or persisted.
9. The loop is capped at three reasoning steps and by the total investigation deadline/tool budget.
10. If the decision endpoint fails, Xentra performs one final evidence-grounded analysis within the same deadline.

## Default guardrails

All values are configurable through environment variables and validated against safe ranges.

- Total investigation deadline: 45 seconds.
- Total typed-tool budget: 8 calls, including the five baseline diagnostics.
- Dynamic tools per reasoning step: 3.
- Reasoning steps: 3.
- Retained evidence items: 48.
- Retained control-plane evidence context: approximately 96 KiB.
- Maximum single retained evidence output: 8 KiB.
- Maximum investigation question: 4,000 characters.
- AI-provider evidence prompt budget: 24,000 characters total and 6,000 characters per item.
- AI-provider output budget: 900 tokens.
- AI-provider request timeout: 20 seconds.

The control plane HTTP server has a 60-second write timeout, while the investigation deadline remains lower at 45 seconds. The bundled Nginx streaming route has buffering disabled and a 65-second read timeout.

## Live progress

`POST /api/investigations/stream` returns authenticated Server-Sent Events. Progress events include:

- collecting
- evidence
- reasoning
- tools
- limit
- reasoning_error
- complete
- error

The React Ask page consumes the stream with `fetch()` so it can continue using the existing bearer-token session. The UI shows partial evidence, elapsed time, tool-call count, and retained-evidence count before the final finding is available.

## Current agentic read-only catalog

- system.info
- system.disk
- system.cpu
- system.memory
- system.service_status
- system.journal
- docker.list
- docker.logs
- docker.inspect
- docker.stats
- git.status
- git.log
- git.diff
- git.show_commit
- http.health_check
- dns.lookup

Mutation tools such as `docker.restart` and `system.service_restart` are intentionally excluded from the investigation loop. They remain available only through the separate owner-approved remediation service.

This preserves the product rule: the AI may investigate automatically, but it cannot mutate infrastructure automatically.
