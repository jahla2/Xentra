package persistence

import (
	"context"
	"database/sql"
)

type PostgresWebhookDeliveryRepository struct{ db *sql.DB }

func NewPostgresWebhookDeliveryRepository(db *sql.DB) *PostgresWebhookDeliveryRepository {
	return &PostgresWebhookDeliveryRepository{db: db}
}

func (r *PostgresWebhookDeliveryRepository) Claim(ctx context.Context, integrationID, deliveryID, eventType string) (bool, error) {
	result, err := r.db.ExecContext(ctx,
		"INSERT INTO github_webhook_deliveries(delivery_id,integration_id,event_type) VALUES($1,$2,$3) ON CONFLICT(delivery_id) DO NOTHING",
		deliveryID, integrationID, eventType,
	)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows == 1, nil
}
