package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
	"github.com/gocql/gocql"
)

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrUsernameTaken = errors.New("username already exists")
	ErrInvalidUser   = errors.New("invalid user data")
)

type Repository struct {
	session *gocql.Session
}

func NewRepository(session *gocql.Session) (*Repository, error) {
	if session == nil {
		return nil, errors.New("ScyllaDB session is required")
	}

	return &Repository{
		session: session,
	}, nil
}

// CreateUser reserves the username before inserting the account.
// The username lookup table uses LWT to enforce uniqueness.
func (r *Repository) CreateUser(
	ctx context.Context,
	user *schema.User,
) error {
	if r == nil || r.session == nil {
		return errors.New("authentication repository is not initialized")
	}

	if user == nil {
		return ErrInvalidUser
	}

	user.Username = strings.TrimSpace(user.Username)
	user.FullName = strings.TrimSpace(user.FullName)

	if user.UserID == (gocql.UUID{}) ||
		user.Username == "" ||
		user.PasswordHash == "" ||
		user.FullName == "" ||
		!isValidRole(user.UserRole) {
		return ErrInvalidUser
	}

	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}

	applied, err := r.session.Query(
		`INSERT INTO users_by_username (username, user_id, created_at)
 VALUES (?, ?, ?) IF NOT EXISTS`,
		user.Username,
		user.UserID,
		user.CreatedAt,
	).WithContext(ctx).MapScanCAS(map[string]interface{}{})

	if err != nil {
		return fmt.Errorf("reserve username: %w", err)
	}

	if !applied {
		return ErrUsernameTaken
	}

	err = r.session.Query(
		`INSERT INTO users_by_id (
user_id, username, password_hash, user_role, full_name,
email, is_active, beneficiary_id, failed_login_count,
locked_until, must_change_password, created_at, created_by,
updated_at, last_login_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.UserID,
		user.Username,
		user.PasswordHash,
		string(user.UserRole),
		user.FullName,
		user.Email,
		user.IsActive,
		user.BeneficiaryID,
		user.FailedLoginCount,
		user.LockedUntil,
		user.MustChangePassword,
		user.CreatedAt,
		user.CreatedBy,
		user.UpdatedAt,
		user.LastLoginAt,
	).WithContext(ctx).Exec()

	if err != nil {
		// Release the username reservation if account creation fails.
		_ = r.session.Query(
			`DELETE FROM users_by_username
 WHERE username = ? IF user_id = ?`,
			user.Username,
			user.UserID,
		).WithContext(ctx).Exec()

		return fmt.Errorf("insert user: %w", err)
	}

	// Maintain the staff listing index for administrative accounts.
	if isStaffRole(user.UserRole) {
		err = r.session.Query(
			`INSERT INTO staff_by_role (
user_role, user_id, username, full_name, is_active
) VALUES (?, ?, ?, ?, ?)`,
			string(user.UserRole),
			user.UserID,
			user.Username,
			user.FullName,
			user.IsActive,
		).WithContext(ctx).Exec()

		if err != nil {
			_ = r.session.Query(
				`DELETE FROM users_by_id WHERE user_id = ?`,
				user.UserID,
			).WithContext(ctx).Exec()

			_ = r.session.Query(
				`DELETE FROM users_by_username
 WHERE username = ? IF user_id = ?`,
				user.Username,
				user.UserID,
			).WithContext(ctx).Exec()

			return fmt.Errorf("insert staff listing: %w", err)
		}
	}

	return nil
}

func (r *Repository) GetUserByUsername(
	ctx context.Context,
	username string,
) (*schema.User, error) {
	if r == nil || r.session == nil {
		return nil, errors.New("authentication repository is not initialized")
	}

	username = strings.TrimSpace(username)
	if username == "" {
		return nil, ErrUserNotFound
	}

	var userID gocql.UUID

	err := r.session.Query(
		`SELECT user_id FROM users_by_username WHERE username = ?`,
		username,
	).WithContext(ctx).Scan(&userID)

	if errors.Is(err, gocql.ErrNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find username: %w", err)
	}

	return r.GetUserByID(ctx, userID)
}

func (r *Repository) GetUserByID(
	ctx context.Context,
	userID gocql.UUID,
) (*schema.User, error) {
	if r == nil || r.session == nil {
		return nil, errors.New("authentication repository is not initialized")
	}

	if userID == (gocql.UUID{}) {
		return nil, ErrUserNotFound
	}

	user := &schema.User{}

	err := r.session.Query(
		`SELECT
user_id, username, password_hash, user_role, full_name,
email, is_active, beneficiary_id, failed_login_count,
locked_until, must_change_password, created_at, created_by,
updated_at, last_login_at
FROM users_by_id WHERE user_id = ?`,
		userID,
	).WithContext(ctx).Scan(
		&user.UserID,
		&user.Username,
		&user.PasswordHash,
		&user.UserRole,
		&user.FullName,
		&user.Email,
		&user.IsActive,
		&user.BeneficiaryID,
		&user.FailedLoginCount,
		&user.LockedUntil,
		&user.MustChangePassword,
		&user.CreatedAt,
		&user.CreatedBy,
		&user.UpdatedAt,
		&user.LastLoginAt,
	)

	if errors.Is(err, gocql.ErrNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by ID: %w", err)
	}

	return user, nil
}

func isStaffRole(role schema.UserRole) bool {
	switch role {
	case schema.RoleSuperAdmin,
		schema.RoleOSCAAdmin,
		schema.RoleOSCAStaff:
		return true
	default:
		return false
	}
}

func (r *Repository) UpdateLoginFailure(
	ctx context.Context,
	userID gocql.UUID,
	failedCount int32,
	lockedUntil *time.Time,
) error {
	if r == nil || r.session == nil {
		return errors.New("authentication repository is not initialized")
	}
	if userID == (gocql.UUID{}) || failedCount < 0 {
		return ErrInvalidUser
	}

	err := r.session.Query(
		`UPDATE users_by_id
 SET failed_login_count = ?, locked_until = ?, updated_at = ?
 WHERE user_id = ?`,
		failedCount,
		lockedUntil,
		time.Now().UTC(),
		userID,
	).WithContext(ctx).Exec()

	if err != nil {
		return fmt.Errorf("update login failure: %w", err)
	}

	return nil
}

func (r *Repository) ResetLoginFailures(
	ctx context.Context,
	userID gocql.UUID,
) error {
	if r == nil || r.session == nil {
		return errors.New("authentication repository is not initialized")
	}
	if userID == (gocql.UUID{}) {
		return ErrInvalidUser
	}

	now := time.Now().UTC()

	err := r.session.Query(
		`UPDATE users_by_id
 SET failed_login_count = 0, locked_until = null,
     last_login_at = ?, updated_at = ?
 WHERE user_id = ?`,
		now,
		now,
		userID,
	).WithContext(ctx).Exec()

	if err != nil {
		return fmt.Errorf("reset login failures: %w", err)
	}

	return nil
}

func (r *Repository) UpdatePassword(
	ctx context.Context,
	userID gocql.UUID,
	passwordHash string,
) error {
	if r == nil || r.session == nil {
		return errors.New("authentication repository is not initialized")
	}
	if userID == (gocql.UUID{}) || strings.TrimSpace(passwordHash) == "" {
		return ErrInvalidUser
	}

	err := r.session.Query(
		`UPDATE users_by_id
 SET password_hash = ?, must_change_password = false,
     failed_login_count = 0, locked_until = null, updated_at = ?
 WHERE user_id = ?`,
		passwordHash,
		time.Now().UTC(),
		userID,
	).WithContext(ctx).Exec()

	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	return nil
}

func (r *Repository) StoreRefreshToken(
	ctx context.Context,
	token *schema.RefreshToken,
) error {
	if r == nil || r.session == nil {
		return errors.New("authentication repository is not initialized")
	}

	if token == nil ||
		strings.TrimSpace(token.TokenHash) == "" ||
		token.UserID == (gocql.UUID{}) ||
		token.FamilyID == (gocql.UUID{}) ||
		token.Revoked {
		return ErrInvalidUser
	}

	now := time.Now().UTC()

	if token.IssuedAt.IsZero() {
		token.IssuedAt = now
	}

	ttl := int(token.ExpiresAt.Sub(now).Seconds())
	if ttl < 1 {
		return ErrInvalidUser
	}

	applied, err := r.session.Query(
		`INSERT INTO refresh_tokens_by_hash (
token_hash, user_id, family_id, issued_at,
expires_at, revoked, replaced_by_hash
) VALUES (?, ?, ?, ?, ?, ?, ?)
IF NOT EXISTS USING TTL ?`,
		token.TokenHash,
		token.UserID,
		token.FamilyID,
		token.IssuedAt,
		token.ExpiresAt,
		false,
		token.ReplacedByHash,
		ttl,
	).WithContext(ctx).MapScanCAS(map[string]interface{}{})

	if err != nil {
		return fmt.Errorf("store refresh token: %w", err)
	}

	if !applied {
		return errors.New("refresh token hash already exists")
	}

	err = r.session.Query(
		`INSERT INTO refresh_tokens_by_user (
user_id, token_hash
) VALUES (?, ?) USING TTL ?`,
		token.UserID,
		token.TokenHash,
		ttl,
	).WithContext(ctx).Exec()

	if err != nil {
		_ = r.session.Query(
			`DELETE FROM refresh_tokens_by_hash
 WHERE token_hash = ? IF user_id = ?`,
			token.TokenHash,
			token.UserID,
		).WithContext(ctx).Exec()

		return fmt.Errorf("index refresh token by user: %w", err)
	}

	return nil
}

func (r *Repository) GetRefreshToken(
	ctx context.Context,
	tokenHash string,
) (*schema.RefreshToken, error) {
	if r == nil || r.session == nil {
		return nil, errors.New("authentication repository is not initialized")
	}

	tokenHash = strings.TrimSpace(tokenHash)
	if tokenHash == "" {
		return nil, ErrInvalidToken
	}

	token := &schema.RefreshToken{}

	err := r.session.Query(
		`SELECT token_hash, user_id, family_id, issued_at,
 expires_at, revoked, replaced_by_hash
 FROM refresh_tokens_by_hash WHERE token_hash = ?`,
		tokenHash,
	).WithContext(ctx).Scan(
		&token.TokenHash,
		&token.UserID,
		&token.FamilyID,
		&token.IssuedAt,
		&token.ExpiresAt,
		&token.Revoked,
		&token.ReplacedByHash,
	)

	if errors.Is(err, gocql.ErrNotFound) {
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, fmt.Errorf("get refresh token: %w", err)
	}

	if token.Revoked || !token.ExpiresAt.After(time.Now().UTC()) {
		return nil, ErrInvalidToken
	}

	return token, nil
}

func (r *Repository) RevokeRefreshToken(
	ctx context.Context,
	tokenHash string,
) error {
	if r == nil || r.session == nil {
		return errors.New("authentication repository is not initialized")
	}

	token, err := r.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		return err
	}

	applied, err := r.session.Query(
		`UPDATE refresh_tokens_by_hash
 SET revoked = true
 WHERE token_hash = ?
 IF revoked = false`,
		token.TokenHash,
	).WithContext(ctx).MapScanCAS(map[string]interface{}{})

	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}

	if !applied {
		return ErrInvalidToken
	}

	err = r.session.Query(
		`DELETE FROM refresh_tokens_by_user
 WHERE user_id = ? AND token_hash = ?`,
		token.UserID,
		token.TokenHash,
	).WithContext(ctx).Exec()

	if err != nil {
		return fmt.Errorf("remove revoked refresh token index: %w", err)
	}

	return nil
}

// RotateRefreshToken stores a replacement and atomically marks the old token
// as revoked. Only one concurrent request can successfully rotate oldHash.
func (r *Repository) RotateRefreshToken(
	ctx context.Context,
	oldHash string,
	replacement *schema.RefreshToken,
) error {
	if r == nil || r.session == nil {
		return errors.New("authentication repository is not initialized")
	}

	oldHash = strings.TrimSpace(oldHash)
	if oldHash == "" || replacement == nil ||
		strings.TrimSpace(replacement.TokenHash) == "" ||
		oldHash == replacement.TokenHash {
		return ErrInvalidToken
	}

	if replacement.Revoked {
		return ErrInvalidToken
	}

	// Preserve the replacement before claiming the old token.
	if err := r.StoreRefreshToken(ctx, replacement); err != nil {
		return fmt.Errorf("store replacement refresh token: %w", err)
	}

	applied, err := r.session.Query(
		`UPDATE refresh_tokens_by_hash
 SET revoked = true, replaced_by_hash = ?
 WHERE token_hash = ?
 IF revoked = false`,
		replacement.TokenHash,
		oldHash,
	).WithContext(ctx).MapScanCAS(map[string]interface{}{})

	if err != nil {
		// The LWT outcome may be uncertain after a timeout.
		// Do not delete the replacement: the rotation may have committed.
		return fmt.Errorf("claim old refresh token: %w", err)
	}

	if !applied {
		// Another request already claimed the old token.
		// Revoke our unused replacement to avoid leaving an active orphan.
		if cleanupErr := r.RevokeRefreshToken(
			ctx,
			replacement.TokenHash,
		); cleanupErr != nil {
			return fmt.Errorf(
				"refresh token already used; replacement cleanup failed: %w",
				cleanupErr,
			)
		}

		return ErrInvalidToken
	}

	// The rotation has committed. The user index has TTL, so a stale
	// index entry is harmless if this best-effort cleanup fails.
	_ = r.session.Query(
		`DELETE FROM refresh_tokens_by_user
 WHERE user_id = ? AND token_hash = ?`,
		replacement.UserID,
		oldHash,
	).WithContext(ctx).Exec()

	return nil
}
