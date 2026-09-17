# Xentra MVP Architecture

Xentra uses four independently deployable units:

1. React UI for operators.
2. Go control plane for deterministic orchestration, policy checks, and APIs.
3. Python AI service for investigation reasoning.
4. Go runner for safe access inside customer Linux/on-prem environments.

The AI never receives raw credentials and never executes arbitrary shell commands. Infrastructure access is represented as typed tools and enforced by deterministic services.
