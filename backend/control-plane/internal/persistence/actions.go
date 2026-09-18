package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type PostgresActionRepository struct{ db *sql.DB }

func NewPostgresActionRepository(db *sql.DB) *PostgresActionRepository {
	return &PostgresActionRepository{db: db}
}

func (r *PostgresActionRepository) Save(ctx context.Context, item domain.ActionRequest) error {
	verification, err := json.Marshal(item.Verification)
	if err != nil {
		return err
	}
	query := "INSERT INTO actions(id,organization_id,incident_id,environment_id,action,target,reason,status,approved_by,rejected_by,result,verification,created_at,executed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) ON CONFLICT(id) DO UPDATE SET organization_id=EXCLUDED.organization_id,status=EXCLUDED.status,approved_by=EXCLUDED.approved_by,rejected_by=EXCLUDED.rejected_by,result=EXCLUDED.result,verification=EXCLUDED.verification,executed_at=EXCLUDED.executed_at"
	_, err = r.db.ExecContext(ctx, query,
		item.ID, item.OrganizationID, nullable(item.IncidentID), item.EnvironmentID, item.Action, item.Target,
		item.Reason, item.Status, nullable(item.ApprovedBy), nullable(item.RejectedBy), item.Result, verification, item.CreatedAt, item.ExecutedAt,
	)
	return err
}

func (r *PostgresActionRepository) Get(ctx context.Context, organizationID, id string) (domain.ActionRequest, error) {
	var item domain.ActionRequest
	var verification []byte
	query := "SELECT id,organization_id,COALESCE(incident_id,''),environment_id,action,target,reason,status,COALESCE(approved_by,''),COALESCE(rejected_by,''),result,verification,created_at,executed_at FROM actions WHERE organization_id=$1 AND id=$2"
	err := r.db.QueryRowContext(ctx, query, organizationID, id).Scan(
		&item.ID, &item.OrganizationID, &item.IncidentID, &item.EnvironmentID, &item.Action, &item.Target,
		&item.Reason, &item.Status, &item.ApprovedBy, &item.RejectedBy, &item.Result, &verification, &item.CreatedAt, &item.ExecutedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ActionRequest{}, errors.New("action not found")
	}
	if err != nil {
		return domain.ActionRequest{}, err
	}
	if err := json.Unmarshal(verification, &item.Verification); err != nil {
		return domain.ActionRequest{}, err
	}
	return item, nil
}

type PostgresAuditRepository struct{ db *sql.DB }

func NewPostgresAuditRepository(db *sql.DB) *PostgresAuditRepository {
	return &PostgresAuditRepository{db: db}
}

func (r *PostgresAuditRepository) Append(ctx context.Context, item domain.AuditEvent) error {
	query := "INSERT INTO audit_events(id,organization_id,environment_id,actor,event_type,detail,success,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)"
	_, err := r.db.ExecContext(ctx, query, item.ID, item.OrganizationID, item.EnvironmentID, item.Actor, item.EventType, item.Detail, item.Success, item.CreatedAt)
	return err
}

func (r *PostgresAuditRepository) List(ctx context.Context, organizationID string) ([]domain.AuditEvent, error) {
	query := "SELECT id,organization_id,environment_id,actor,event_type,detail,success,created_at FROM audit_events WHERE organization_id=$1 ORDER BY created_at DESC LIMIT 200"
	rows, err := r.db.QueryContext(ctx, query, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.AuditEvent{}
	for rows.Next() {
		var item domain.AuditEvent
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.EnvironmentID, &item.Actor, &item.EventType, &item.Detail, &item.Success, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
