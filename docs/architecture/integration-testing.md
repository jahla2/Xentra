# Integration Smoke Environment

The CI integration job validates Xentra against real process and infrastructure boundaries after unit/build jobs pass.

It starts:

- PostgreSQL 17 as a GitHub Actions service.
- The Python AI service in heuristic mode.
- The Go Xentra Runner as a real local process.
- The Go control plane using PostgreSQL persistence.
- A disposable Alpine Linux SSH server with OpenSSH.
- Docker on the GitHub Actions host.

The smoke flow verifies:

1. Owner registration and persisted server-side session.
2. Runner environment discovery.
3. SSH private-key authentication with SHA256 host-key pinning.
4. Rejection of a mismatched SSH host fingerprint.
5. Evidence collection and AI investigation.
6. Incident creation.
7. Human-approved typed Docker restart through the Runner.
8. Post-action Docker running-state verification.
9. Audit records for approval and execution.
10. Owner-created member login and member read access.
11. Member denial for owner-only environment creation.
12. Cross-organization resource isolation.
13. Control-plane restart with environment and session persistence in PostgreSQL.
14. Logout session revocation.

The integration suite intentionally runs after the fast Go, Python and UI jobs so expensive Docker/SSH setup is only performed after the repository already passes unit and build checks.
