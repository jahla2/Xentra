package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type PostgresEnvironmentRepository struct{ db *sql.DB }

func NewPostgresEnvironmentRepository(db *sql.DB) *PostgresEnvironmentRepository {
	return &PostgresEnvironmentRepository{db: db}
}

func (r *PostgresEnvironmentRepository) Save(ctx context.Context, env domain.Environment) error {
	capabilities, err := json.Marshal(env.Capabilities)
	if err != nil {
		return err
	}
	containers, err := json.Marshal(env.Containers)
	if err != nil {
		return err
	}
	query := "INSERT INTO environments(id,organization_id,project_id,name,environment_type,connection_type,runner_url,ssh_host,ssh_port,ssh_user,ssh_host_key_fingerprint,credential_id,health_url,aws_region,aws_instance_id,os,hostname,cpu,memory,disk,containers,capabilities) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22) ON CONFLICT(id) DO UPDATE SET organization_id=EXCLUDED.organization_id,project_id=EXCLUDED.project_id,name=EXCLUDED.name,environment_type=EXCLUDED.environment_type,connection_type=EXCLUDED.connection_type,runner_url=EXCLUDED.runner_url,ssh_host=EXCLUDED.ssh_host,ssh_port=EXCLUDED.ssh_port,ssh_user=EXCLUDED.ssh_user,ssh_host_key_fingerprint=EXCLUDED.ssh_host_key_fingerprint,credential_id=EXCLUDED.credential_id,health_url=EXCLUDED.health_url,aws_region=EXCLUDED.aws_region,aws_instance_id=EXCLUDED.aws_instance_id,os=EXCLUDED.os,hostname=EXCLUDED.hostname,cpu=EXCLUDED.cpu,memory=EXCLUDED.memory,disk=EXCLUDED.disk,containers=EXCLUDED.containers,capabilities=EXCLUDED.capabilities"
	_, err = r.db.ExecContext(ctx, query,
		env.ID, env.OrganizationID, env.ProjectID, env.Name, env.Type, env.ConnectionType, nullable(env.RunnerURL),
		nullable(env.SSHHost), env.SSHPort, nullable(env.SSHUser), nullable(env.SSHHostKeyFingerprint),
		nullable(env.CredentialID), env.HealthURL, env.AWSRegion, env.AWSInstanceID, env.OS, env.Hostname, env.CPU, env.Memory, env.Disk, containers, capabilities,
	)
	return err
}

func (r *PostgresEnvironmentRepository) Get(ctx context.Context, organizationID, id string) (domain.Environment, error) {
	query := "SELECT id,organization_id,COALESCE(project_id,''),name,environment_type,connection_type,COALESCE(runner_url,''),COALESCE(ssh_host,''),ssh_port,COALESCE(ssh_user,''),COALESCE(ssh_host_key_fingerprint,''),COALESCE(credential_id,''),COALESCE(health_url,''),COALESCE(aws_region,''),COALESCE(aws_instance_id,''),os,hostname,cpu,memory,disk,containers,capabilities FROM environments WHERE organization_id=$1 AND id=$2"
	env, err := scanEnvironment(r.db.QueryRowContext(ctx, query, organizationID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Environment{}, errors.New("environment not found")
	}
	return env, err
}

func (r *PostgresEnvironmentRepository) List(ctx context.Context, organizationID string) ([]domain.Environment, error) {
	query := "SELECT id,organization_id,COALESCE(project_id,''),name,environment_type,connection_type,COALESCE(runner_url,''),COALESCE(ssh_host,''),ssh_port,COALESCE(ssh_user,''),COALESCE(ssh_host_key_fingerprint,''),COALESCE(credential_id,''),COALESCE(health_url,''),COALESCE(aws_region,''),COALESCE(aws_instance_id,''),os,hostname,cpu,memory,disk,containers,capabilities FROM environments WHERE organization_id=$1 ORDER BY name"
	rows, err := r.db.QueryContext(ctx, query, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []domain.Environment{}
	for rows.Next() {
		env, err := scanEnvironment(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, env)
	}
	return result, rows.Err()
}

type rowScanner interface{ Scan(...any) error }

func scanEnvironment(row rowScanner) (domain.Environment, error) {
	var env domain.Environment
	var containers, capabilities []byte
	err := row.Scan(
		&env.ID, &env.OrganizationID, &env.ProjectID, &env.Name, &env.Type, &env.ConnectionType, &env.RunnerURL,
		&env.SSHHost, &env.SSHPort, &env.SSHUser, &env.SSHHostKeyFingerprint, &env.CredentialID, &env.HealthURL, &env.AWSRegion, &env.AWSInstanceID,
		&env.OS, &env.Hostname, &env.CPU, &env.Memory, &env.Disk, &containers, &capabilities,
	)
	if err != nil {
		return domain.Environment{}, err
	}
	if err := json.Unmarshal(containers, &env.Containers); err != nil {
		return domain.Environment{}, err
	}
	if err := json.Unmarshal(capabilities, &env.Capabilities); err != nil {
		return domain.Environment{}, err
	}
	return env, nil
}

type PostgresCredentialRepository struct{ db *sql.DB }

func NewPostgresCredentialRepository(db *sql.DB) *PostgresCredentialRepository {
	return &PostgresCredentialRepository{db: db}
}

func (r *PostgresCredentialRepository) Save(ctx context.Context, record domain.CredentialRecord) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO credentials(id,kind,ciphertext) VALUES($1,$2,$3) ON CONFLICT(id) DO UPDATE SET kind=EXCLUDED.kind,ciphertext=EXCLUDED.ciphertext", record.ID, record.Kind, record.Ciphertext)
	return err
}

func (r *PostgresCredentialRepository) Get(ctx context.Context, id string) (domain.CredentialRecord, error) {
	var record domain.CredentialRecord
	err := r.db.QueryRowContext(ctx, "SELECT id,kind,ciphertext FROM credentials WHERE id=$1", id).Scan(&record.ID, &record.Kind, &record.Ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CredentialRecord{}, errors.New("credential not found")
	}
	return record, err
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
