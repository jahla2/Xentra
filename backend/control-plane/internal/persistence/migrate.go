package persistence

import (
	"context"
	"database/sql"
	_ "embed"
)

//go:embed migrations/001_core.sql
var coreMigration string

//go:embed migrations/002_incidents.sql
var incidentMigration string

//go:embed migrations/003_actions_audit.sql
var actionsAuditMigration string

//go:embed migrations/004_auth_org.sql
var authOrgMigration string

//go:embed migrations/005_projects.sql
var projectsMigration string

//go:embed migrations/006_github_webhooks.sql
var githubWebhooksMigration string

//go:embed migrations/007_github_app.sql
var githubAppMigration string

//go:embed migrations/008_incident_memory.sql
var incidentMemoryMigration string

func Migrate(ctx context.Context, db *sql.DB) error {
	for _, migration := range []string{coreMigration, incidentMigration, actionsAuditMigration, authOrgMigration, projectsMigration, githubWebhooksMigration, githubAppMigration, incidentMemoryMigration} {
		if _, err := db.ExecContext(ctx, migration); err != nil {
			return err
		}
	}
	return nil
}
