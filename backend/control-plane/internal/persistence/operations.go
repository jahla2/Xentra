package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type PostgresIntegrationRepository struct{ db *sql.DB }

func NewPostgresIntegrationRepository(db *sql.DB) *PostgresIntegrationRepository {
	return &PostgresIntegrationRepository{db: db}
}

func (r *PostgresIntegrationRepository) Save(ctx context.Context, item domain.RepositoryIntegration) error {
	query := "INSERT INTO repository_integrations(id,organization_id,environment_id,provider,owner_name,repo_name,credential_id,webhook_secret_credential_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(environment_id) DO UPDATE SET organization_id=EXCLUDED.organization_id,provider=EXCLUDED.provider,owner_name=EXCLUDED.owner_name,repo_name=EXCLUDED.repo_name,credential_id=EXCLUDED.credential_id,webhook_secret_credential_id=EXCLUDED.webhook_secret_credential_id"
	_, err := r.db.ExecContext(ctx, query, item.ID, item.OrganizationID, item.EnvironmentID, item.Provider, item.Owner, item.Repo, item.CredentialID, nullable(item.WebhookSecretCredentialID))
	return err
}

func (r *PostgresIntegrationRepository) FindByEnvironment(ctx context.Context, organizationID, id string) (domain.RepositoryIntegration, error) {
	var item domain.RepositoryIntegration
	query := "SELECT id,organization_id,environment_id,provider,owner_name,repo_name,credential_id,COALESCE(webhook_secret_credential_id,'') FROM repository_integrations WHERE organization_id=$1 AND environment_id=$2"
	err := r.db.QueryRowContext(ctx, query, organizationID, id).Scan(&item.ID, &item.OrganizationID, &item.EnvironmentID, &item.Provider, &item.Owner, &item.Repo, &item.CredentialID, &item.WebhookSecretCredentialID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RepositoryIntegration{}, errors.New("repository integration not found")
	}
	return item, err
}

func (r *PostgresIntegrationRepository) FindByID(ctx context.Context, id string) (domain.RepositoryIntegration, error) {
	var item domain.RepositoryIntegration
	query := "SELECT id,organization_id,environment_id,provider,owner_name,repo_name,credential_id,COALESCE(webhook_secret_credential_id,'') FROM repository_integrations WHERE id=$1"
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&item.ID, &item.OrganizationID, &item.EnvironmentID, &item.Provider,
		&item.Owner, &item.Repo, &item.CredentialID, &item.WebhookSecretCredentialID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RepositoryIntegration{}, errors.New("repository integration not found")
	}
	return item, err
}

type PostgresIncidentRepository struct{ db *sql.DB }

func NewPostgresIncidentRepository(db *sql.DB) *PostgresIncidentRepository {
	return &PostgresIncidentRepository{db: db}
}

func (r *PostgresIncidentRepository) Save(ctx context.Context, item domain.Incident) error {
	evidence, err := json.Marshal(item.Evidence)
	if err != nil {
		return err
	}
	timeline, err := json.Marshal(item.Timeline)
	if err != nil {
		return err
	}
	query := "INSERT INTO incidents(id,organization_id,environment_id,question,status,summary,root_cause,confidence,recommended_action,evidence,timeline,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT(id) DO UPDATE SET organization_id=EXCLUDED.organization_id,status=EXCLUDED.status,summary=EXCLUDED.summary,root_cause=EXCLUDED.root_cause,confidence=EXCLUDED.confidence,recommended_action=EXCLUDED.recommended_action,evidence=EXCLUDED.evidence,timeline=EXCLUDED.timeline"
	_, err = r.db.ExecContext(ctx, query,
		item.ID, item.OrganizationID, item.EnvironmentID, item.Question, item.Status, item.Summary,
		item.RootCause, item.Confidence, item.RecommendedAction, evidence, timeline, item.CreatedAt,
	)
	return err
}

func (r *PostgresIncidentRepository) Get(ctx context.Context, organizationID, id string) (domain.Incident, error) {
	var item domain.Incident
	var evidence, timeline []byte
	query := "SELECT id,organization_id,environment_id,question,status,summary,root_cause,confidence,recommended_action,evidence,timeline,created_at FROM incidents WHERE organization_id=$1 AND id=$2"
	err := r.db.QueryRowContext(ctx, query, organizationID, id).Scan(
		&item.ID, &item.OrganizationID, &item.EnvironmentID, &item.Question, &item.Status, &item.Summary,
		&item.RootCause, &item.Confidence, &item.RecommendedAction, &evidence, &timeline, &item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Incident{}, errors.New("incident not found")
	}
	if err != nil {
		return domain.Incident{}, err
	}
	if err := json.Unmarshal(evidence, &item.Evidence); err != nil {
		return domain.Incident{}, err
	}
	if err := json.Unmarshal(timeline, &item.Timeline); err != nil {
		return domain.Incident{}, err
	}
	return item, nil
}

func (r *PostgresIncidentRepository) List(ctx context.Context, organizationID string) ([]domain.Incident, error) {
	query := "SELECT id,organization_id,environment_id,question,status,summary,root_cause,confidence,recommended_action,evidence,timeline,created_at FROM incidents WHERE organization_id=$1 ORDER BY created_at DESC"
	rows, err := r.db.QueryContext(ctx, query, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.Incident{}
	for rows.Next() {
		var item domain.Incident
		var evidence, timeline []byte
		if err := rows.Scan(
			&item.ID, &item.OrganizationID, &item.EnvironmentID, &item.Question, &item.Status, &item.Summary,
			&item.RootCause, &item.Confidence, &item.RecommendedAction, &evidence, &timeline, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(evidence, &item.Evidence); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(timeline, &item.Timeline); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
