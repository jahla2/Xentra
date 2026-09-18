# Projects

Xentra now models infrastructure as:

Organization -> Project -> Environment

Projects are tenant-scoped organizational units for grouping related development, staging and production environments.

Rules:

- Owners can create projects.
- Owners and members can list projects in their organization.
- Every newly created environment must reference a project.
- The environment service verifies that the referenced project belongs to the authenticated organization.
- PostgreSQL enforces project ownership and cascades environment deletion when a project is removed in a future deletion flow.
- Legacy environment rows may have a null project_id after migration, but all new writes require projectId at the application boundary.

The React MVP exposes project creation/selection and attaches new Runner or SSH environments to the selected project.
