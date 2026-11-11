package schema

import (
	"time"

	"github.com/gocql/gocql"
)

// User roles
type UserRole string

const (
	RoleSuperAdmin               UserRole = "SUPER_ADMIN"
	RoleOSCAAdmin                UserRole = "OSCA_ADMIN"
	RoleOSCAStaff                UserRole = "OSCA_STAFF"
	RoleSeniorCitizen            UserRole = "SENIOR_CITIZEN"
	RoleAuthorizedRepresentative UserRole = "AUTHORIZED_REPRESENTATIVE"
)

// User represents an authenticated application account.
type User struct {
	UserID             gocql.UUID  `json:"user_id"`
	Username           string      `json:"username"`
	PasswordHash       string      `json:"-"`
	UserRole           UserRole    `json:"user_role"`
	FullName           string      `json:"full_name"`
	Email              *string     `json:"email,omitempty"`
	IsActive           bool        `json:"is_active"`
	BeneficiaryID      *gocql.UUID `json:"beneficiary_id,omitempty"`
	FailedLoginCount   int32       `json:"failed_login_count"`
	LockedUntil        *time.Time  `json:"locked_until,omitempty"`
	MustChangePassword bool        `json:"must_change_password"`
	CreatedAt          time.Time   `json:"created_at"`
	CreatedBy          *gocql.UUID `json:"created_by,omitempty"`
	UpdatedAt          *time.Time  `json:"updated_at,omitempty"`
	LastLoginAt        *time.Time  `json:"last_login_at,omitempty"`
}

// UserUsername maps a unique username to a user ID.
type UserUsername struct {
	Username  string     `json:"username"`
	UserID    gocql.UUID `json:"user_id"`
	CreatedAt time.Time  `json:"created_at"`
}

// StaffByRole represents an OSCA staff listing entry.
type StaffByRole struct {
	UserRole UserRole   `json:"user_role"`
	UserID   gocql.UUID `json:"user_id"`
	Username string     `json:"username"`
	FullName string     `json:"full_name"`
	IsActive bool       `json:"is_active"`
}

// RefreshToken stores only a hashed refresh token.
type RefreshToken struct {
	TokenHash      string     `json:"-"`
	UserID         gocql.UUID `json:"user_id"`
	FamilyID       gocql.UUID `json:"family_id"`
	IssuedAt       time.Time  `json:"issued_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	Revoked        bool       `json:"revoked"`
	ReplacedByHash *string    `json:"-"`
}

// RefreshTokenByUser supports revoking all sessions belonging to a user.
type RefreshTokenByUser struct {
	UserID    gocql.UUID `json:"user_id"`
	TokenHash string     `json:"-"`
}

// ActivationCode stores only a hashed activation code.
type ActivationCode struct {
	CodeHash      string     `json:"-"`
	BeneficiaryID gocql.UUID `json:"beneficiary_id"`
	IssuedBy      gocql.UUID `json:"issued_by"`
	IssuedAt      time.Time  `json:"issued_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
	Used          bool       `json:"used"`
}

// RateLimitEvent represents one sliding-window rate-limit event.
type RateLimitEvent struct {
	BucketKey string     `json:"-"`
	EventID   gocql.UUID `json:"event_id"`
}
