# Approval, Remediation, Verification and Audit

Xentra keeps infrastructure mutation behind an explicit human approval boundary.

Supported MVP mutation actions:

- `docker.restart`
- `system.service_restart`

The AI/control plane may propose these actions, but proposal creation never executes them. A separate approval call containing the human approver identity is required.

Execution routes through the environment connection adapter (Runner or SSH). Both implementations map typed actions to hard-coded commands and validate target names. There is no arbitrary shell endpoint.

After execution Xentra runs deterministic verification:

- Docker restart -> Docker running-state inspection.
- systemd restart -> `systemctl is-active`.

An incident is marked resolved only when verification reports healthy. Proposals, approvals and execution results are persisted in the audit trail.
