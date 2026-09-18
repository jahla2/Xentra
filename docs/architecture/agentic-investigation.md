# Agentic Investigation Loop

Xentra investigations can now iterate over bounded, typed, read-only diagnostic tools before producing a final finding.

Flow:

1. Collect baseline evidence.
2. Redact secrets.
3. Ask the AI service whether the evidence is sufficient.
4. If more evidence is needed, the AI may request up to three tools from the environment-specific read-only catalog.
5. The Go control plane validates each request and rejects unavailable or mutation tools.
6. Runner or SSH executes the typed diagnostic.
7. New output is redacted before being returned to the AI.
8. The loop is capped at three decision steps.
9. If the agent cannot complete within the bound or the decision endpoint fails, Xentra falls back to a final evidence-grounded investigation using all collected evidence.

Current agentic read-only catalog:

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

Mutation tools such as docker.restart and system.service_restart are intentionally excluded from this loop. They remain available only through the separate human-approved remediation service.

This preserves the product rule: the AI may investigate automatically, but it cannot mutate infrastructure automatically.
