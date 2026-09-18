package persistence

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/application"
	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type PostgresAuthRepository struct{ db *sql.DB }

func NewPostgresAuthRepository(db *sql.DB) *PostgresAuthRepository {
	return &PostgresAuthRepository{db: db}
}

func (r *PostgresAuthRepository) CreateAccount(ctx context.Context, user domain.User, org domain.Organization, membership domain.Membership) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, "INSERT INTO users(id,email,password_hash,created_at) VALUES($1,$2,$3,$4)", user.ID, user.Email, user.PasswordHash, user.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			return application.ErrEmailExists
		}
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO organizations(id,name,created_at) VALUES($1,$2,$3)", org.ID, org.Name, org.CreatedAt); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO memberships(user_id,organization_id,role) VALUES($1,$2,$3)", membership.UserID, membership.OrganizationID, membership.Role); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *PostgresAuthRepository) CreateMember(ctx context.Context, user domain.User, membership domain.Membership) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, "INSERT INTO users(id,email,password_hash,created_at) VALUES($1,$2,$3,$4)", user.ID, user.Email, user.PasswordHash, user.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			return application.ErrEmailExists
		}
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO memberships(user_id,organization_id,role) VALUES($1,$2,$3)", membership.UserID, membership.OrganizationID, membership.Role); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *PostgresAuthRepository) FindUserByEmail(ctx context.Context, email string) (domain.User, error) {
	var user domain.User
	err := r.db.QueryRowContext(ctx, "SELECT id,email,password_hash,created_at FROM users WHERE email=$1", email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, application.ErrInvalidCredentials
	}
	return user, err
}

func (r *PostgresAuthRepository) FindPrincipalByUser(ctx context.Context, userID string) (domain.Principal, error) {
	var principal domain.Principal
	err := r.db.QueryRowContext(ctx,
		"SELECT u.id,u.email,o.id,o.name,m.role FROM users u JOIN memberships m ON m.user_id=u.id JOIN organizations o ON o.id=m.organization_id WHERE u.id=$1 ORDER BY o.created_at LIMIT 1",
		userID,
	).Scan(&principal.UserID, &principal.Email, &principal.OrganizationID, &principal.OrganizationName, &principal.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Principal{}, application.ErrInvalidCredentials
	}
	return principal, err
}

func (r *PostgresAuthRepository) CreateSession(ctx context.Context, session domain.Session) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO sessions(id,token_hash,user_id,organization_id,expires_at,created_at) VALUES($1,$2,$3,$4,$5,$6)",
		session.ID, session.TokenHash, session.UserID, session.OrganizationID, session.ExpiresAt, session.CreatedAt,
	)
	return err
}

func (r *PostgresAuthRepository) FindPrincipalBySessionHash(ctx context.Context, tokenHash string, now time.Time) (domain.Principal, error) {
	var principal domain.Principal
	err := r.db.QueryRowContext(ctx,
		"SELECT u.id,u.email,o.id,o.name,m.role FROM sessions s JOIN users u ON u.id=s.user_id JOIN memberships m ON m.user_id=u.id AND m.organization_id=s.organization_id JOIN organizations o ON o.id=s.organization_id WHERE s.token_hash=$1 AND s.expires_at>$2",
		tokenHash, now,
	).Scan(&principal.UserID, &principal.Email, &principal.OrganizationID, &principal.OrganizationName, &principal.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Principal{}, application.ErrInvalidSession
	}
	return principal, err
}

func (r *PostgresAuthRepository) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash=$1", tokenHash)
	return err
}

func isUniqueViolation(err error) bool {
	type sqlState interface{ SQLState() string }
	var state sqlState
	return errors.As(err, &state) && state.SQLState() == "23505"
}
