package beneficiary

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
	ErrNotFound               = errors.New("beneficiary not found")
	ErrInvalidBeneficiary     = errors.New("invalid beneficiary data")
	ErrDuplicateOSCAID        = errors.New("OSCA ID already exists")
	ErrDuplicateBeneficiaryID = errors.New("beneficiary ID already exists")
	ErrVersionConflict        = errors.New("beneficiary was modified by another request")
)

type Repository struct {
	session *gocql.Session
}

func NewRepository(session *gocql.Session) (*Repository, error) {
	if session == nil {
		return nil, errors.New("ScyllaDB session is required")
	}

	return &Repository{session: session}, nil
}

func (r *Repository) initialized() bool {
	return r != nil && r.session != nil
}

func validStatus(status schema.BeneficiaryStatus) bool {
	switch status {
	case schema.BeneficiaryPendingVerification,
		schema.BeneficiaryVerified,
		schema.BeneficiaryActive,
		schema.BeneficiaryInactive,
		schema.BeneficiaryForReview,
		schema.BeneficiaryArchived:
		return true
	default:
		return false
	}
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil
	}

	return &normalized
}

// Create inserts a beneficiary and reserves its OSCA ID when supplied.
// Search and listing projections are maintained in a later repository stage.
func (r *Repository) Create(
	ctx context.Context,
	beneficiary *schema.Beneficiary,
) error {
	if !r.initialized() {
		return errors.New("beneficiary repository is not initialized")
	}

	if beneficiary == nil {
		return ErrInvalidBeneficiary
	}

	if beneficiary.BeneficiaryID == (gocql.UUID{}) {
		beneficiary.BeneficiaryID = gocql.TimeUUID()
	}

	beneficiary.OSCAID = normalizeOptionalString(beneficiary.OSCAID)
	beneficiary.FirstName = normalizeOptionalString(beneficiary.FirstName)
	beneficiary.MiddleName = normalizeOptionalString(beneficiary.MiddleName)
	beneficiary.LastName = normalizeOptionalString(beneficiary.LastName)
	beneficiary.Suffix = normalizeOptionalString(beneficiary.Suffix)
	beneficiary.Sex = normalizeOptionalString(beneficiary.Sex)
	beneficiary.BarangayCode = normalizeOptionalString(beneficiary.BarangayCode)
	beneficiary.BarangayName = normalizeOptionalString(beneficiary.BarangayName)
	beneficiary.ArchiveReason = normalizeOptionalString(beneficiary.ArchiveReason)

	if beneficiary.RecordStatus == "" {
		beneficiary.RecordStatus = schema.BeneficiaryPendingVerification
	}

	if !validStatus(beneficiary.RecordStatus) || beneficiary.RowVersion < 0 {
		return ErrInvalidBeneficiary
	}

	// Sort order is derived consistently from last, first, and middle names.
	sortName := deriveSortName(beneficiary)
	beneficiary.SortName = &sortName

	now := time.Now().UTC()

	if beneficiary.CreatedAt == nil {
		beneficiary.CreatedAt = &now
	}

	if beneficiary.UpdatedAt == nil {
		beneficiary.UpdatedAt = beneficiary.CreatedAt
	}

	if beneficiary.RowVersion == 0 {
		beneficiary.RowVersion = 1
	}

	// Check whether the beneficiary UUID already exists.
	// This is a preflight check, not a concurrency-safe UUID constraint.
	_, err := r.GetByID(ctx, beneficiary.BeneficiaryID)
	switch {
	case err == nil:
		return ErrDuplicateBeneficiaryID
	case errors.Is(err, ErrNotFound):
	// Continue with creation.
	default:
		return fmt.Errorf("check beneficiary ID: %w", err)
	}

	// Reserve the OSCA ID using the dedicated LWT lookup table.
	if beneficiary.OSCAID != nil {
		applied, err := r.session.Query(
			`INSERT INTO beneficiaries_by_osca_id (
osca_id, beneficiary_id, created_at
) VALUES (?, ?, ?) IF NOT EXISTS`,
			*beneficiary.OSCAID,
			beneficiary.BeneficiaryID,
			*beneficiary.CreatedAt,
		).WithContext(ctx).MapScanCAS(map[string]interface{}{})

		if err != nil {
			return fmt.Errorf("reserve OSCA ID: %w", err)
		}

		if !applied {
			// Permit a retry if this same beneficiary already owns the reservation.
			var existingBeneficiaryID gocql.UUID

			err := r.session.Query(
				`SELECT beneficiary_id
FROM beneficiaries_by_osca_id
WHERE osca_id = ?`,
				*beneficiary.OSCAID,
			).WithContext(ctx).Scan(&existingBeneficiaryID)

			if err != nil {
				if errors.Is(err, gocql.ErrNotFound) {
					return fmt.Errorf("OSCA ID reservation was not applied and could not be found: %w", err)
				}
				return fmt.Errorf("check existing OSCA ID reservation: %w", err)
			}

			if existingBeneficiaryID != beneficiary.BeneficiaryID {
				return ErrDuplicateOSCAID
			}
		}
	}

	// The base record and all query projections are written together.
	// Repeating these inserts with the same primary keys and values is idempotent.
	var batchErr error

	for attempt := 1; attempt <= 2; attempt++ {
		batch := r.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
		addCreateBatchStatements(batch, beneficiary)

		batchErr = r.session.ExecuteBatch(batch)
		if batchErr == nil {
			return nil
		}

		if ctx.Err() != nil {
			return fmt.Errorf("insert beneficiary batch: %w", batchErr)
		}
	}

	// Keep the OSCA reservation if the batch result is uncertain.
	// Releasing it could allow another beneficiary to claim an ID
	// even though the original batch may have committed.
	return fmt.Errorf("insert beneficiary batch after retry: %w", batchErr)
}
func (r *Repository) GetByID(
	ctx context.Context,
	beneficiaryID gocql.UUID,
) (*schema.Beneficiary, error) {
	if !r.initialized() {
		return nil, errors.New("beneficiary repository is not initialized")
	}

	if beneficiaryID == (gocql.UUID{}) {
		return nil, ErrNotFound
	}

	beneficiary := &schema.Beneficiary{}

	err := r.session.Query(
		`SELECT
beneficiary_id, osca_id, first_name, middle_name,
last_name, suffix, sort_name, birth_date, sex,
barangay_code, barangay_name, record_status,
last_verified_at, last_verified_by, row_version,
created_at, created_by, updated_at, updated_by,
archived_at, archived_by, archive_reason
FROM beneficiaries_by_id
WHERE beneficiary_id = ?`,
		beneficiaryID,
	).WithContext(ctx).Scan(
		&beneficiary.BeneficiaryID,
		&beneficiary.OSCAID,
		&beneficiary.FirstName,
		&beneficiary.MiddleName,
		&beneficiary.LastName,
		&beneficiary.Suffix,
		&beneficiary.SortName,
		&beneficiary.BirthDate,
		&beneficiary.Sex,
		&beneficiary.BarangayCode,
		&beneficiary.BarangayName,
		&beneficiary.RecordStatus,
		&beneficiary.LastVerifiedAt,
		&beneficiary.LastVerifiedBy,
		&beneficiary.RowVersion,
		&beneficiary.CreatedAt,
		&beneficiary.CreatedBy,
		&beneficiary.UpdatedAt,
		&beneficiary.UpdatedBy,
		&beneficiary.ArchivedAt,
		&beneficiary.ArchivedBy,
		&beneficiary.ArchiveReason,
	)

	if errors.Is(err, gocql.ErrNotFound) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get beneficiary by ID: %w", err)
	}

	return beneficiary, nil
}

func (r *Repository) GetByOSCAID(
	ctx context.Context,
	oscaID string,
) (*schema.Beneficiary, error) {
	if !r.initialized() {
		return nil, errors.New("beneficiary repository is not initialized")
	}

	oscaID = strings.TrimSpace(oscaID)
	if oscaID == "" {
		return nil, ErrNotFound
	}

	var beneficiaryID gocql.UUID

	err := r.session.Query(
		`SELECT beneficiary_id
FROM beneficiaries_by_osca_id
WHERE osca_id = ?`,
		oscaID,
	).WithContext(ctx).Scan(&beneficiaryID)

	if errors.Is(err, gocql.ErrNotFound) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find OSCA ID: %w", err)
	}

	return r.GetByID(ctx, beneficiaryID)
}
