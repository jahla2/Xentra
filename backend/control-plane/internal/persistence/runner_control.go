package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type PostgresRunnerControlRepository struct{ db *sql.DB }

func NewPostgresRunnerControlRepository(db *sql.DB) *PostgresRunnerControlRepository {
	return &PostgresRunnerControlRepository{db: db}
}

func (r *PostgresRunnerControlRepository) SaveRunner(ctx context.Context, item domain.RunnerRegistration) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO outbound_runners(id,organization_id,environment_id,credential_id,last_seen_at,created_at) VALUES($1,$2,$3,$4,$5,$6)",
		item.ID, item.OrganizationID, item.EnvironmentID, item.CredentialID, item.LastSeenAt, item.CreatedAt,
	)
	return err
}

func (r *PostgresRunnerControlRepository) FindRunnerByID(ctx context.Context, id string) (domain.RunnerRegistration, error) {
	var item domain.RunnerRegistration
	var lastSeen sql.NullTime
	err := r.db.QueryRowContext(ctx,
		"SELECT id,organization_id,environment_id,credential_id,last_seen_at,created_at FROM outbound_runners WHERE id=$1",
		id,
	).Scan(&item.ID, &item.OrganizationID, &item.EnvironmentID, &item.CredentialID, &lastSeen, &item.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RunnerRegistration{}, errors.New("outbound runner not found")
	}
	if err != nil {
		return domain.RunnerRegistration{}, err
	}
	if lastSeen.Valid {
		value := lastSeen.Time
		item.LastSeenAt = &value
	}
	return item, nil
}

func (r *PostgresRunnerControlRepository) FindRunnerByEnvironment(ctx context.Context, organizationID, environmentID string) (domain.RunnerRegistration, error) {
	var item domain.RunnerRegistration
	var lastSeen sql.NullTime
	err := r.db.QueryRowContext(ctx,
		"SELECT id,organization_id,environment_id,credential_id,last_seen_at,created_at FROM outbound_runners WHERE organization_id=$1 AND environment_id=$2",
		organizationID, environmentID,
	).Scan(&item.ID, &item.OrganizationID, &item.EnvironmentID, &item.CredentialID, &lastSeen, &item.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RunnerRegistration{}, errors.New("outbound runner not found")
	}
	if err != nil {
		return domain.RunnerRegistration{}, err
	}
	if lastSeen.Valid {
		value := lastSeen.Time
		item.LastSeenAt = &value
	}
	return item, nil
}

func (r *PostgresRunnerControlRepository) TouchRunner(ctx context.Context, id string, seenAt time.Time) error {
	_, err := r.db.ExecContext(ctx, "UPDATE outbound_runners SET last_seen_at=$2 WHERE id=$1", id, seenAt)
	return err
}

func (r *PostgresRunnerControlRepository) EnqueueTask(ctx context.Context, item domain.RunnerTask) error {
	args, err := json.Marshal(item.Arguments)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		"INSERT INTO runner_tasks(id,runner_id,tool,arguments,status,created_at) VALUES($1,$2,$3,$4,$5,$6)",
		item.ID, item.RunnerID, item.Tool, args, item.Status, item.CreatedAt,
	)
	return err
}

func (r *PostgresRunnerControlRepository) ClaimNextTask(ctx context.Context, runnerID string, staleBefore time.Time) (domain.RunnerTask, bool, error) {
	if _, err := r.db.ExecContext(ctx,
		"UPDATE runner_tasks SET status='queued',claimed_at=NULL WHERE runner_id=$1 AND status='claimed' AND claimed_at<$2",
		runnerID, staleBefore,
	); err != nil {
		return domain.RunnerTask{}, false, err
	}

	var item domain.RunnerTask
	var args []byte
	var claimedAt time.Time
	err := r.db.QueryRowContext(ctx, `
WITH next_task AS (
    SELECT id FROM runner_tasks
    WHERE runner_id=$1 AND status='queued'
    ORDER BY created_at
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE runner_tasks AS task
SET status='claimed', claimed_at=NOW()
FROM next_task
WHERE task.id=next_task.id
RETURNING task.id,task.runner_id,task.tool,task.arguments,task.status,task.created_at,task.claimed_at
`, runnerID).Scan(
		&item.ID, &item.RunnerID, &item.Tool, &args, &item.Status, &item.CreatedAt, &claimedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RunnerTask{}, false, nil
	}
	if err != nil {
		return domain.RunnerTask{}, false, err
	}
	item.ClaimedAt = &claimedAt
	if err := json.Unmarshal(args, &item.Arguments); err != nil {
		return domain.RunnerTask{}, false, err
	}
	return item, true, nil
}

func (r *PostgresRunnerControlRepository) CompleteTask(ctx context.Context, runnerID, taskID string, result domain.RunnerTaskResult, completedAt time.Time) error {
	execResult, err := r.db.ExecContext(ctx, `
UPDATE runner_tasks
SET status='completed',result_success=$3,result_output=$4,result_error=$5,completed_at=$6
WHERE id=$1 AND runner_id=$2 AND status='claimed'
`, taskID, runnerID, result.Success, result.Output, result.Error, completedAt)
	if err != nil {
		return err
	}
	rows, err := execResult.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 1 {
		return nil
	}

	var ownerID, status string
	if err := r.db.QueryRowContext(ctx, "SELECT runner_id,status FROM runner_tasks WHERE id=$1", taskID).Scan(&ownerID, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("runner task not found")
		}
		return err
	}
	if ownerID == runnerID && status == "completed" {
		return nil
	}
	return errors.New("runner task is not claimable for completion")
}

func (r *PostgresRunnerControlRepository) GetTask(ctx context.Context, id string) (domain.RunnerTask, error) {
	var item domain.RunnerTask
	var args []byte
	var success sql.NullBool
	var claimed, completed sql.NullTime
	err := r.db.QueryRowContext(ctx, `
SELECT id,runner_id,tool,arguments,status,result_success,result_output,result_error,created_at,claimed_at,completed_at
FROM runner_tasks WHERE id=$1
`, id).Scan(
		&item.ID, &item.RunnerID, &item.Tool, &args, &item.Status,
		&success, &item.Result.Output, &item.Result.Error, &item.CreatedAt, &claimed, &completed,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RunnerTask{}, errors.New("runner task not found")
	}
	if err != nil {
		return domain.RunnerTask{}, err
	}
	if err := json.Unmarshal(args, &item.Arguments); err != nil {
		return domain.RunnerTask{}, err
	}
	if success.Valid {
		item.Result.Success = success.Bool
	}
	if claimed.Valid {
		value := claimed.Time
		item.ClaimedAt = &value
	}
	if completed.Valid {
		value := completed.Time
		item.CompletedAt = &value
	}
	return item, nil
}
