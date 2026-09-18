package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type PostgresIncidentMemoryRepository struct {
	db *sql.DB
}

func NewPostgresIncidentMemoryRepository(db *sql.DB) *PostgresIncidentMemoryRepository {
	return &PostgresIncidentMemoryRepository{db: db}
}

func (r *PostgresIncidentMemoryRepository) Save(ctx context.Context, item domain.IncidentMemory) error {
	vector, err := formatVector(item.Embedding)
	if err != nil {
		return err
	}
	query := "INSERT INTO incident_memories(id,organization_id,environment_id,incident_id,content,embedding,created_at) " +
		"VALUES($1,$2,$3,$4,$5,$6::vector,$7) " +
		"ON CONFLICT(incident_id) DO UPDATE SET content=EXCLUDED.content,embedding=EXCLUDED.embedding,created_at=EXCLUDED.created_at"
	_, err = r.db.ExecContext(ctx, query, item.ID, item.OrganizationID, item.EnvironmentID, item.IncidentID, item.Content, vector, item.CreatedAt)
	return err
}

func (r *PostgresIncidentMemoryRepository) Search(
	ctx context.Context,
	organizationID, environmentID string,
	embedding []float64,
	limit int,
) ([]domain.IncidentMemoryMatch, error) {
	vector, err := formatVector(embedding)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 3
	}
	query := "SELECT id,organization_id,environment_id,incident_id,content,created_at,embedding <=> $3::vector AS distance " +
		"FROM incident_memories WHERE organization_id=$1 AND environment_id=$2 " +
		"ORDER BY embedding <=> $3::vector LIMIT $4"
	rows, err := r.db.QueryContext(ctx, query, organizationID, environmentID, vector, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []domain.IncidentMemoryMatch{}
	for rows.Next() {
		var item domain.IncidentMemory
		var distance float64
		if err := rows.Scan(
			&item.ID,
			&item.OrganizationID,
			&item.EnvironmentID,
			&item.IncidentID,
			&item.Content,
			&item.CreatedAt,
			&distance,
		); err != nil {
			return nil, err
		}
		result = append(result, domain.IncidentMemoryMatch{Memory: item, Distance: distance})
	}
	return result, rows.Err()
}

func formatVector(values []float64) (string, error) {
	if len(values) != domain.IncidentMemoryDimensions {
		return "", fmt.Errorf("embedding must contain %d dimensions", domain.IncidentMemoryDimensions)
	}
	parts := make([]string, len(values))
	for index, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return "", fmt.Errorf("embedding contains non-finite value at index %d", index)
		}
		parts[index] = strconv.FormatFloat(value, 'f', -1, 64)
	}
	return "[" + strings.Join(parts, ",") + "]", nil
}
