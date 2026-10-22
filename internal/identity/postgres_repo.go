package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implements Repository on top of pgxpool.Pool.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository initializes a new PostgresRepository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// CreateUser persists a new user with a unique lowercase email.
func (r *PostgresRepository) CreateUser(ctx context.Context, email, passwordHash, displayName string) (*User, error) {
	query := `
		INSERT INTO users (email, password_hash, display_name)
		VALUES (lower($1), $2, $3)
		RETURNING id, email, password_hash, display_name, email_verified_at, created_at, updated_at
	`
	user := &User{}
	err := r.pool.QueryRow(ctx, query, email, passwordHash, displayName).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.DisplayName,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil, ErrUserAlreadyExists
		}
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}
	return user, nil
}

// GetUserByEmail queries a user record by email (case-insensitive).
func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, password_hash, display_name, email_verified_at, created_at, updated_at
		FROM users
		WHERE email = lower($1)
	`
	user := &User{}
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.DisplayName,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user by email: %w", err)
	}
	return user, nil
}

// GetUserByID queries a user record by its UUID.
func (r *PostgresRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	query := `
		SELECT id, email, password_hash, display_name, email_verified_at, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	user := &User{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.DisplayName,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user by id: %w", err)
	}
	return user, nil
}

// CreateRefreshSession persists a new refresh session.
func (r *PostgresRepository) CreateRefreshSession(ctx context.Context, session *RefreshSession) error {
	query := `
		INSERT INTO refresh_sessions (id, user_id, token_hash, family_id, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.pool.Exec(ctx, query, session.ID, session.UserID, session.TokenHash, session.FamilyID, session.ExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to insert refresh session: %w", err)
	}
	return nil
}

// GetRefreshSessionByHash retrieves a refresh session using the SHA-256 token hash.
func (r *PostgresRepository) GetRefreshSessionByHash(ctx context.Context, tokenHash []byte) (*RefreshSession, error) {
	query := `
		SELECT id, user_id, token_hash, family_id, expires_at, revoked_at, replaced_by, created_at
		FROM refresh_sessions
		WHERE token_hash = $1
	`
	session := &RefreshSession{}
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.FamilyID,
		&session.ExpiresAt,
		&session.RevokedAt,
		&session.ReplacedBy,
		&session.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to query refresh session: %w", err)
	}
	return session, nil
}

// RotateRefreshSession atomically marks oldSession as revoked/replaced and inserts newSession.
func (r *PostgresRepository) RotateRefreshSession(ctx context.Context, oldSessionID uuid.UUID, newSession *RefreshSession) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert the new session
	insertQuery := `
		INSERT INTO refresh_sessions (id, user_id, token_hash, family_id, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = tx.Exec(ctx, insertQuery, newSession.ID, newSession.UserID, newSession.TokenHash, newSession.FamilyID, newSession.ExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to insert rotated refresh session: %w", err)
	}

	// Mark old session replaced and revoked
	now := time.Now().UTC()
	updateQuery := `
		UPDATE refresh_sessions
		SET revoked_at = $1, replaced_by = $2
		WHERE id = $3
	`
	res, err := tx.Exec(ctx, updateQuery, now, newSession.ID, oldSessionID)
	if err != nil {
		return fmt.Errorf("failed to update old refresh session: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrSessionNotFound
	}

	return tx.Commit(ctx)
}

// RevokeRefreshFamily immediately marks every session in a family as revoked.
func (r *PostgresRepository) RevokeRefreshFamily(ctx context.Context, familyID uuid.UUID) error {
	query := `
		UPDATE refresh_sessions
		SET revoked_at = now()
		WHERE family_id = $1 AND revoked_at IS NULL
	`
	_, err := r.pool.Exec(ctx, query, familyID)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh family: %w", err)
	}
	return nil
}

// RevokeRefreshSessionByHash marks a specific session as revoked.
func (r *PostgresRepository) RevokeRefreshSessionByHash(ctx context.Context, tokenHash []byte) error {
	query := `
		UPDATE refresh_sessions
		SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`
	_, err := r.pool.Exec(ctx, query, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh session: %w", err)
	}
	return nil
}

// CreateOrganization transactionally creates an organization and assigns the creator as owner.
func (r *PostgresRepository) CreateOrganization(ctx context.Context, name string, creatorID uuid.UUID) (*Organization, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	org := &Organization{}
	orgQuery := `
		INSERT INTO organizations (name, created_by)
		VALUES ($1, $2)
		RETURNING id, name, created_by, created_at
	`
	err = tx.QueryRow(ctx, orgQuery, name, creatorID).Scan(
		&org.ID,
		&org.Name,
		&org.CreatedBy,
		&org.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert organization: %w", err)
	}

	memberQuery := `
		INSERT INTO memberships (organization_id, user_id, role)
		VALUES ($1, $2, $3)
	`
	_, err = tx.Exec(ctx, memberQuery, org.ID, creatorID, RoleOwner)
	if err != nil {
		return nil, fmt.Errorf("failed to insert creator membership: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit organization transaction: %w", err)
	}

	return org, nil
}

// GetOrganizationByID retrieves organization metadata.
func (r *PostgresRepository) GetOrganizationByID(ctx context.Context, orgID uuid.UUID) (*Organization, error) {
	query := `
		SELECT id, name, created_by, created_at
		FROM organizations
		WHERE id = $1
	`
	org := &Organization{}
	err := r.pool.QueryRow(ctx, query, orgID).Scan(
		&org.ID,
		&org.Name,
		&org.CreatedBy,
		&org.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrgNotFound
		}
		return nil, fmt.Errorf("failed to query organization: %w", err)
	}
	return org, nil
}

// ListUserOrganizations queries all organizations where the given user is a member.
func (r *PostgresRepository) ListUserOrganizations(ctx context.Context, userID uuid.UUID) ([]*UserOrganization, error) {
	query := `
		SELECT o.id, o.name, m.role, o.created_at
		FROM organizations o
		JOIN memberships m ON o.id = m.organization_id
		WHERE m.user_id = $1
		ORDER BY o.created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user organizations: %w", err)
	}
	defer rows.Close()

	var orgs []*UserOrganization
	for rows.Next() {
		org := &UserOrganization{}
		if err := rows.Scan(&org.ID, &org.Name, &org.Role, &org.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan organization: %w", err)
		}
		orgs = append(orgs, org)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row error listing organizations: %w", err)
	}
	return orgs, nil
}

// GetMembership queries the membership role of a specific user in an organization.
func (r *PostgresRepository) GetMembership(ctx context.Context, orgID, userID uuid.UUID) (*Membership, error) {
	query := `
		SELECT organization_id, user_id, role, created_at
		FROM memberships
		WHERE organization_id = $1 AND user_id = $2
	`
	membership := &Membership{}
	err := r.pool.QueryRow(ctx, query, orgID, userID).Scan(
		&membership.OrganizationID,
		&membership.UserID,
		&membership.Role,
		&membership.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMembershipNotFound
		}
		return nil, fmt.Errorf("failed to query membership: %w", err)
	}
	return membership, nil
}

// AddMember grants a user a role within an organization.
func (r *PostgresRepository) AddMember(ctx context.Context, orgID, userID uuid.UUID, role string) (*Membership, error) {
	query := `
		INSERT INTO memberships (organization_id, user_id, role)
		VALUES ($1, $2, $3)
		RETURNING organization_id, user_id, role, created_at
	`
	membership := &Membership{}
	err := r.pool.QueryRow(ctx, query, orgID, userID, role).Scan(
		&membership.OrganizationID,
		&membership.UserID,
		&membership.Role,
		&membership.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil, ErrMembershipExists
		}
		return nil, fmt.Errorf("failed to add member: %w", err)
	}
	return membership, nil
}

// ListMembers retrieves all members and their roles in an organization.
func (r *PostgresRepository) ListMembers(ctx context.Context, orgID uuid.UUID) ([]*MemberDetail, error) {
	query := `
		SELECT u.id, u.email, u.display_name, m.role, m.created_at
		FROM memberships m
		JOIN users u ON m.user_id = u.id
		WHERE m.organization_id = $1
		ORDER BY m.created_at ASC
	`
	rows, err := r.pool.Query(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list organization members: %w", err)
	}
	defer rows.Close()

	var members []*MemberDetail
	for rows.Next() {
		m := &MemberDetail{}
		if err := rows.Scan(&m.UserID, &m.Email, &m.DisplayName, &m.Role, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("failed to scan member detail: %w", err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row error listing members: %w", err)
	}
	return members, nil
}

// RemoveMember removes a user from an organization.
func (r *PostgresRepository) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	query := `
		DELETE FROM memberships
		WHERE organization_id = $1 AND user_id = $2
	`
	res, err := r.pool.Exec(ctx, query, orgID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete membership: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrMembershipNotFound
	}
	return nil
}

// CountOwners returns the number of active owners for an organization.
func (r *PostgresRepository) CountOwners(ctx context.Context, orgID uuid.UUID) (int, error) {
	query := `
		SELECT count(*)
		FROM memberships
		WHERE organization_id = $1 AND role = $2
	`
	var count int
	err := r.pool.QueryRow(ctx, query, orgID, RoleOwner).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count owners: %w", err)
	}
	return count, nil
}
