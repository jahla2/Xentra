# Evidence Redaction

Xentra sanitizes collected diagnostic evidence before it is sent to the AI service and before it is returned for incident persistence.

The control-plane redactor currently masks:

- PEM/OpenSSH private-key blocks.
- Authorization Bearer values.
- password, secret, token, API-key, access-key and client-secret assignments.
- credentials embedded in PostgreSQL, MySQL, MongoDB, Redis and AMQP URLs.
- AWS access-key IDs.
- common GitHub token formats.
- OpenAI-style API-key formats.
- JWT-shaped bearer values.

Redaction is deterministic and runs in the control plane. This is intentionally separate from prompt instructions: the LLM is never relied on to protect secrets after receiving them.

The raw collection result should remain transient process memory only. Incident evidence receives the redacted copy.
