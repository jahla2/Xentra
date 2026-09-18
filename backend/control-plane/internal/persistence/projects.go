package persistence

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type PostgresProjectRepository struct{ db *sql.DB }

func NewPostgresProjectRepository(db *sql.DB) *PostgresProjectRepository {
	return &PostgresProjectRepository{db: db}
}

func (r *PostgresProjectRepository) Save(ctx context.Context, item domain.Project) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO projects(id,organization_id,name,description,created_at) VALUES($1,$2,$3,$4,$5)",
		item.ID, item.OrganizationID, item.Name, item.Description, item.CreatedAt,
	)
	return err
}

func (r *PostgresProjectRepository) Get(ctx context.Context, organizationID, id string) (domain.Project, error) {
	var item domain.Project
	err := r.db.QueryRowContext(ctx,
		"SELECT id,organization_id,name,description,created_at FROM projects WHERE organization_id=$1 AND id=$2",
		organizationID, id,
	).Scan(&item.ID, &item.OrganizationID, &item.Name, &item.Description, &item.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Project{}, errors.New("project not found")
	}
	return item, err
}

func (r *PostgresProjectRepository) List(ctx context.Context, organizationID string) ([]domain.Project, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id,organization_id,name,description,created_at FROM projects WHERE organization_id=$1 ORDER BY name",
		organizationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.Project{}
	for rows.Next() {
		var item domain.Project
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.Name, &item.Description, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
