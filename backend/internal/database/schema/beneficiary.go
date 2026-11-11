package schema

import (
	"time"

	"github.com/gocql/gocql"
)

// Beneficiary record workflow statuses.
// These are application statuses, not official eligibility decisions.
type BeneficiaryStatus string

const (
	BeneficiaryPendingVerification BeneficiaryStatus = "PENDING_VERIFICATION"
	BeneficiaryVerified            BeneficiaryStatus = "VERIFIED"
	BeneficiaryActive              BeneficiaryStatus = "ACTIVE"
	BeneficiaryInactive            BeneficiaryStatus = "INACTIVE"
	BeneficiaryForReview           BeneficiaryStatus = "FOR_REVIEW"
	BeneficiaryArchived            BeneficiaryStatus = "ARCHIVED"
)

// Beneficiary represents the primary beneficiary record.
type Beneficiary struct {
	BeneficiaryID  gocql.UUID        `json:"beneficiary_id"`
	OSCAID         *string           `json:"osca_id,omitempty"`
	FirstName      *string           `json:"first_name,omitempty"`
	MiddleName     *string           `json:"middle_name,omitempty"`
	LastName       *string           `json:"last_name,omitempty"`
	Suffix         *string           `json:"suffix,omitempty"`
	SortName       *string           `json:"sort_name,omitempty"`
	BirthDate      *time.Time        `json:"birth_date,omitempty"`
	Sex            *string           `json:"sex,omitempty"`
	BarangayCode   *string           `json:"barangay_code,omitempty"`
	BarangayName   *string           `json:"barangay_name,omitempty"`
	RecordStatus   BeneficiaryStatus `json:"record_status"`
	LastVerifiedAt *time.Time        `json:"last_verified_at,omitempty"`
	LastVerifiedBy *gocql.UUID       `json:"last_verified_by,omitempty"`
	RowVersion     int64             `json:"row_version"`
	CreatedAt      *time.Time        `json:"created_at,omitempty"`
	CreatedBy      *gocql.UUID       `json:"created_by,omitempty"`
	UpdatedAt      *time.Time        `json:"updated_at,omitempty"`
	UpdatedBy      *gocql.UUID       `json:"updated_by,omitempty"`
	ArchivedAt     *time.Time        `json:"archived_at,omitempty"`
	ArchivedBy     *gocql.UUID       `json:"archived_by,omitempty"`
	ArchiveReason  *string           `json:"archive_reason,omitempty"`
}

// BeneficiaryOSCAID maps an OSCA ID to its beneficiary.
type BeneficiaryOSCAID struct {
	OSCAID        string     `json:"osca_id"`
	BeneficiaryID gocql.UUID `json:"beneficiary_id"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
}

// BeneficiarySearch represents a name-search projection.
type BeneficiarySearch struct {
	BarangayCode  string            `json:"barangay_code"`
	NamePrefix    string            `json:"name_prefix"`
	SortName      string            `json:"sort_name"`
	BeneficiaryID gocql.UUID        `json:"beneficiary_id"`
	OSCAID        *string           `json:"osca_id,omitempty"`
	DisplayName   *string           `json:"display_name,omitempty"`
	RecordStatus  BeneficiaryStatus `json:"record_status"`
}

// BeneficiaryByBarangay represents the default admin listing projection.
type BeneficiaryByBarangay struct {
	BarangayCode  string            `json:"barangay_code"`
	SortName      string            `json:"sort_name"`
	BeneficiaryID gocql.UUID        `json:"beneficiary_id"`
	OSCAID        *string           `json:"osca_id,omitempty"`
	DisplayName   *string           `json:"display_name,omitempty"`
	BirthDate     *time.Time        `json:"birth_date,omitempty"`
	RecordStatus  BeneficiaryStatus `json:"record_status"`
	ReviewFlag    *string           `json:"review_flag,omitempty"`
	UpdatedAt     *time.Time        `json:"updated_at,omitempty"`
}

// BeneficiaryByStatus represents the admin listing filtered by status.
type BeneficiaryByStatus struct {
	RecordStatus  BeneficiaryStatus `json:"record_status"`
	BarangayCode  string            `json:"barangay_code"`
	SortName      string            `json:"sort_name"`
	BeneficiaryID gocql.UUID        `json:"beneficiary_id"`
	OSCAID        *string           `json:"osca_id,omitempty"`
	DisplayName   *string           `json:"display_name,omitempty"`
	BirthDate     *time.Time        `json:"birth_date,omitempty"`
	ReviewFlag    *string           `json:"review_flag,omitempty"`
	UpdatedAt     *time.Time        `json:"updated_at,omitempty"`
}

// BeneficiaryStatusHistory is an append-only status timeline event.
type BeneficiaryStatusHistory struct {
	BeneficiaryID gocql.UUID `json:"beneficiary_id"`
	ChangedAt     time.Time  `json:"changed_at"`
	EventID       gocql.UUID `json:"event_id"`
	OldStatus     *string    `json:"old_status,omitempty"`
	NewStatus     string     `json:"new_status"`
	ChangedBy     gocql.UUID `json:"changed_by"`
	Remarks       *string    `json:"remarks,omitempty"`
}

// Representative authorization statuses.
type RepresentativeStatus string

const (
	RepresentativePending  RepresentativeStatus = "PENDING"
	RepresentativeVerified RepresentativeStatus = "VERIFIED"
	RepresentativeRevoked  RepresentativeStatus = "REVOKED"
	RepresentativeExpired  RepresentativeStatus = "EXPIRED"
)

// Representative represents an authorized person linked to a beneficiary.
// Only the ID type and last four ID characters are retained.
type Representative struct {
	BeneficiaryID      gocql.UUID           `json:"beneficiary_id"`
	RepresentativeID   gocql.UUID           `json:"rep_id"`
	RepresentativeName string               `json:"rep_name"`
	Relationship       *string              `json:"relationship,omitempty"`
	IDType             *string              `json:"id_type,omitempty"`
	IDNumberLast4      *string              `json:"id_number_last4,omitempty"`
	AuthStatus         RepresentativeStatus `json:"auth_status"`
	ValidFrom          *time.Time           `json:"valid_from,omitempty"`
	ValidUntil         *time.Time           `json:"valid_until,omitempty"`
	UserID             *gocql.UUID          `json:"user_id,omitempty"`
	RequestedAt        *time.Time           `json:"requested_at,omitempty"`
	VerifiedBy         *gocql.UUID          `json:"verified_by,omitempty"`
	VerifiedAt         *time.Time           `json:"verified_at,omitempty"`
	RevokedBy          *gocql.UUID          `json:"revoked_by,omitempty"`
	RevokedAt          *time.Time           `json:"revoked_at,omitempty"`
	Remarks            *string              `json:"remarks,omitempty"`
}

// RepresentativeByUser lets a logged-in representative find linked beneficiaries.
type RepresentativeByUser struct {
	UserID           gocql.UUID `json:"user_id"`
	BeneficiaryID    gocql.UUID `json:"beneficiary_id"`
	RepresentativeID gocql.UUID `json:"rep_id"`
}
