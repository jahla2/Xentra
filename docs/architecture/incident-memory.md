# Incident Memory

Xentra stores compact, redacted incident knowledge for future investigations.

## Flow

1. An incident is created from evidence-grounded investigation results.
2. Xentra builds a compact memory document from the question, summary, probable root cause, and recommended action.
3. The document is passed through the control-plane secret redactor.
4. The AI service generates a 384-dimensional embedding.
   - When `XENTRA_EMBEDDING_MODEL` is configured, Xentra uses the configured OpenAI-compatible embeddings endpoint.
   - Otherwise it uses a deterministic local hashed embedding fallback for development and offline operation.
5. PostgreSQL stores the document in `incident_memories` with `vector(384)`.
6. New investigations embed the user's question and retrieve the nearest memories within the same organization and environment.
7. Only sufficiently similar memories are injected as typed evidence (`incident.memory:<incident-id>`).

## Isolation and safety

Memory search is always scoped by both `organization_id` and `environment_id`. Cross-tenant and cross-environment retrieval is rejected by repository filtering.

Raw incident evidence and logs are never copied into long-term memory. Memory text is redacted before embedding and persistence, and recalled memory is redacted again before AI use.

## PostgreSQL

Production PostgreSQL requires the pgvector extension. CI uses the official pgvector PostgreSQL image and creates an HNSW cosine-distance index for retrieval.
