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
	if err != nil { return err }
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO actions(id,incident_id,environment_id,action,target,reason,status,approved_by,result,verification,created_at,executed_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT(id) DO UPDATE SET
		status=EXCLUDED.status,approved_by=EXCLUDED.approved_by,result=EXCLUDED.result,
		verification=EXCLUDED.verification,executed_at=EXCLUDED.executed_at
	`, item.ID, nullable(item.IncidentID), item.EnvironmentID, item.Action, item.Target, item.Reason, item.Status,
		nullable(item.ApprovedBy), item.Result, verification, item.CreatedAt, item.ExecutedAt)
	return err
}

func (r *PostgresActionRepository) Get(ctx context.Context, id string) (domain.ActionRequest, error) {
	var item domain.ActionRequest
	var verification []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id,COALESCE(incident_id,''),environment_id,action,target,reason,status,
		COALESCE(approved_by,''),result,verification,created_at,executed_at
		FROM actions WHERE id=$1
	`, id).Scan(&item.ID, &item.IncidentID, &item.EnvironmentID, &item.Action, &item.Target, &item.Reason,
		&item.Status, &item.ApprovedBy, &item.Result, &verification, &item.CreatedAt, &item.ExecutedAt)
	if errors.Is(err, sql.ErrNoRows) { return domain.ActionRequest{}, errors.New("action not found") }
	if err != nil { return domain.ActionRequest{}, err }
	if err := json.Unmarshal(verification, &item.Verification); err != nil { return domain.ActionRequest{}, err }
	return item, nil
}

type PostgresAuditRepository struct{ db *sql.DB }

func NewPostgresAuditRepository(db *sql.DB) *PostgresAuditRepository {
	return &PostgresAuditRepository{db: db}
}

func (r *PostgresAuditRepository) Append(ctx context.Context, item domain.AuditEvent) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_events(id,environment_id,actor,event_type,detail,success,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7)
	`, item.ID, item.EnvironmentID, item.Actor, item.EventType, item.Detail, item.Success, item.CreatedAt)
	return err
}

func (r *PostgresAuditRepository) List(ctx context.Context) ([]domain.AuditEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id,environment_id,actor,event_type,detail,success,created_at
		FROM audit_events ORDER BY created_at DESC LIMIT 200
	`)
	if err != nil { return nil, err }
	defer rows.Close()

	items := []domain.AuditEvent{}
	for rows.Next() {
		var item domain.AuditEvent
		if err := rows.Scan(&item.ID, &item.EnvironmentID, &item.Actor, &item.EventType, &item.Detail, &item.Success, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
