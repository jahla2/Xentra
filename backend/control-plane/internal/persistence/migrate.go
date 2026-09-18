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

//go:embed migrations/009_outbound_runner.sql
var outboundRunnerMigration string

//go:embed migrations/010_otel_trace_context.sql
var otelTraceContextMigration string

//go:embed migrations/011_environment_discovery.sql
var environmentDiscoveryMigration string

//go:embed migrations/012_action_rejection.sql
var actionRejectionMigration string

//go:embed migrations/013_environment_health_url.sql
var environmentHealthURLMigration string

//go:embed migrations/014_environment_aws_ssm.sql
var environmentAWSSSMMigration string

func Migrate(ctx context.Context, db *sql.DB) error {
	for _, migration := range []string{coreMigration, incidentMigration, actionsAuditMigration, authOrgMigration, projectsMigration, githubWebhooksMigration, githubAppMigration, incidentMemoryMigration, outboundRunnerMigration, otelTraceContextMigration, environmentDiscoveryMigration, actionRejectionMigration, environmentHealthURLMigration, environmentAWSSSMMigration} {
		if _, err := db.ExecContext(ctx, migration); err != nil {
			return err
		}
	}
	return nil
}
