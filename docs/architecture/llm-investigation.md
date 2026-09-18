# LLM Investigation Provider

Xentra's Python AI service supports an OpenAI-compatible chat-completions provider while preserving a deterministic heuristic fallback.

## Configuration

Set these variables on the AI service:

- XENTRA_LLM_API_KEY — provider credential.
- XENTRA_LLM_MODEL — provider model identifier.
- XENTRA_LLM_BASE_URL — API root ending before /chat/completions. Defaults to https://api.openai.com/v1.
- XENTRA_LLM_TIMEOUT_SECONDS — request timeout, default 20.
- XENTRA_LLM_MAX_EVIDENCE_CHARS — total evidence character budget, default 24000.
- XENTRA_LLM_MAX_ITEM_CHARS — per-evidence-item budget, default 6000.

If the API key or model is absent, Xentra runs the local heuristic provider. If a configured provider fails or returns invalid structured output, Xentra falls back to the heuristic provider rather than blocking incident investigation.

## Guardrails

- Only normalized environment metadata and already-redacted evidence are sent to the model.
- Raw SSH/GitHub credentials are never part of the AI service request schema.
- Evidence is bounded before being sent to the provider.
- The provider must return a validated JSON object with summary, confidence, probable root cause, and recommended action.
- Confidence is restricted to low, medium, or high.
- The model is instructed to use only supplied evidence and never invent infrastructure state.
- Recommended actions remain advisory; infrastructure mutations still pass through Xentra's separate human-approval flow.
