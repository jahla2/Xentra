# Authentication, Organizations and RBAC

Xentra uses opaque bearer sessions backed by the control-plane database.

## Account model

- Registration creates a user, organization and owner membership in one transaction.
- Owners can create member accounts for their organization.
- Roles are owner and member.
- Passwords are hashed with bcrypt.
- Session tokens are cryptographically random and only their SHA-256 hashes are stored.
- Sessions expire; XENTRA_SESSION_TTL_HOURS defaults to 24.

## Authorization

Owner-only operations:

- create environments
- connect GitHub repositories
- create remediation proposals
- approve remediation
- create member accounts

Owners and members can:

- list organization environments
- run read-only investigations
- create and list incidents
- read the organization audit trail

## Tenant isolation

Persistent operational records carry organization_id:

- environments
- repository integrations
- incidents
- actions
- audit events

Repository reads require both organization_id and resource ID. This prevents a resource ID from another organization being used to cross the tenant boundary.

## Browser session

The MVP React client stores the opaque bearer token in sessionStorage, not localStorage, and sends it in the Authorization header. Logout revokes the server-side session. A future hardened browser deployment can move UI sessions to same-site HttpOnly cookies while retaining bearer sessions for API/CLI clients.
