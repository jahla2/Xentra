# Secure SSH and Persistence

Xentra supports Runner and SSH environment connections.

SSH private keys and optional passphrases are encrypted with AES-256-GCM using XENTRA_MASTER_KEY before storage. The AI service never receives credential material.

SSH host keys are pinned by SHA256 fingerprint. Connections without a fingerprint are rejected.

When XENTRA_DATABASE_URL is configured, environments and encrypted credentials are persisted in PostgreSQL. XENTRA_MASTER_KEY is mandatory with persistent storage.

SSH evidence collection is deterministic and allowlisted: system information, disk usage, Docker status and bounded Docker logs. The LLM cannot submit arbitrary shell commands.
