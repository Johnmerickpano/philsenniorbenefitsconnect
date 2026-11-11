package beneficiary

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
	"github.com/gocql/gocql"
)

// UpdateProfile replaces the editable profile fields of a beneficiary.
// It preserves workflow status and all server-managed fields.
func (r *Repository) UpdateProfile(
	ctx context.Context,
	beneficiaryID gocql.UUID,
	profile BeneficiaryProfile,
	expectedVersion int64,
	updatedBy gocql.UUID,
) (*schema.Beneficiary, error) {
	if !r.initialized() {
		return nil, errors.New("beneficiary repository is not initialized")
	}

	if beneficiaryID == (gocql.UUID{}) ||
		updatedBy == (gocql.UUID{}) ||
		expectedVersion <= 0 {
		return nil, ErrInvalidBeneficiary
	}

	current, err := r.GetByID(ctx, beneficiaryID)
	if err != nil {
		return nil, err
	}

	if current.RowVersion != expectedVersion {
		return nil, ErrVersionConflict
	}

	updated := *current

	updated.OSCAID = normalizeOptionalString(profile.OSCAID)
	updated.FirstName = normalizeOptionalString(profile.FirstName)
	updated.MiddleName = normalizeOptionalString(profile.MiddleName)
	updated.LastName = normalizeOptionalString(profile.LastName)
	updated.Suffix = normalizeOptionalString(profile.Suffix)
	updated.BirthDate = profile.BirthDate
	updated.Sex = normalizeOptionalString(profile.Sex)
	updated.BarangayCode = normalizeOptionalString(profile.BarangayCode)
	updated.BarangayName = normalizeOptionalString(profile.BarangayName)

	sortName := deriveSortName(&updated)
	updated.SortName = &sortName

	now := time.Now().UTC()
	updated.UpdatedAt = &now
	updated.UpdatedBy = &updatedBy
	updated.RowVersion = expectedVersion + 1

	// Preserve the latest review flag before replacing listing projections.
	reviewFlag, err := r.readReviewFlag(ctx, current)
	if err != nil {
		return nil, err
	}

	// Reserve a changed OSCA ID before updating the source record.
	oldOSCAID := optionalStringValue(current.OSCAID)
	newOSCAID := optionalStringValue(updated.OSCAID)

	if updated.OSCAID != nil && oldOSCAID != newOSCAID {
		_, err := r.reserveOSCAID(
			ctx,
			newOSCAID,
			beneficiaryID,
			now,
		)
		if err != nil {
			return nil, err
		}
	}

	// Update the source-of-truth record using optimistic concurrency.
	applied, err := r.session.Query(
		`UPDATE beneficiaries_by_id
SET osca_id = ?, first_name = ?, middle_name = ?,
    last_name = ?, suffix = ?, sort_name = ?,
    birth_date = ?, sex = ?, barangay_code = ?,
    barangay_name = ?, updated_at = ?, updated_by = ?,
    row_version = ?
WHERE beneficiary_id = ?
IF row_version = ?`,
		updated.OSCAID,
		updated.FirstName,
		updated.MiddleName,
		updated.LastName,
		updated.Suffix,
		updated.SortName,
		updated.BirthDate,
		updated.Sex,
		updated.BarangayCode,
		updated.BarangayName,
		updated.UpdatedAt,
		updated.UpdatedBy,
		updated.RowVersion,
		beneficiaryID,
		expectedVersion,
	).WithContext(ctx).MapScanCAS(map[string]interface{}{})

	if err != nil {
		// The result may be uncertain. Keep any new OSCA reservation.
		return nil, fmt.Errorf("update beneficiary profile: %w", err)
	}

	if !applied {
		// Another update won the version check.
		// Keep any newly created reservation for safe reconciliation.
		return nil, ErrVersionConflict
	}

	// Refresh listing and search projections after the source update.
	var projectionErr error

	if current.BarangayCode != nil || updated.BarangayCode != nil {
		for attempt := 1; attempt <= 2; attempt++ {
			batch := r.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)

			addProfileProjectionStatements(
				batch,
				current,
				&updated,
				reviewFlag,
			)

			projectionErr = r.session.ExecuteBatch(batch)
			if projectionErr == nil {
				break
			}
		}
	}

	// The source record now uses the new OSCA ID (or no OSCA ID).
	// It is now safe to release the old reservation.
	if current.OSCAID != nil && oldOSCAID != newOSCAID {
		if err := r.releaseOSCAID(
			ctx,
			oldOSCAID,
			beneficiaryID,
		); err != nil {
			return nil, fmt.Errorf(
				"profile updated, but old OSCA ID release failed: %w",
				err,
			)
		}
	}

	if projectionErr != nil {
		return nil, fmt.Errorf(
			"profile updated, but projection refresh failed: %w",
			projectionErr,
		)
	}

	return &updated, nil
}
