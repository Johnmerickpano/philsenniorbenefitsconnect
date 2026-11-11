package beneficiary

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gocql/gocql"
)

// reserveOSCAID reserves an OSCA ID for a beneficiary.
// The returned bool is true only when this call created the reservation.
func (r *Repository) reserveOSCAID(
	ctx context.Context,
	oscaID string,
	beneficiaryID gocql.UUID,
	createdAt time.Time,
) (bool, error) {
	applied, err := r.session.Query(
		`INSERT INTO beneficiaries_by_osca_id (
    osca_id, beneficiary_id, created_at
) VALUES (?, ?, ?) IF NOT EXISTS`,
		oscaID,
		beneficiaryID,
		createdAt,
	).WithContext(ctx).MapScanCAS(map[string]interface{}{})

	if err != nil {
		return false, fmt.Errorf("reserve OSCA ID: %w", err)
	}

	if applied {
		return true, nil
	}

	var existingBeneficiaryID gocql.UUID

	err = r.session.Query(
		`SELECT beneficiary_id
FROM beneficiaries_by_osca_id
WHERE osca_id = ?`,
		oscaID,
	).WithContext(ctx).Scan(&existingBeneficiaryID)

	if errors.Is(err, gocql.ErrNotFound) {
		return false, fmt.Errorf(
			"OSCA ID reservation was not applied and could not be found",
		)
	}

	if err != nil {
		return false, fmt.Errorf("check OSCA ID reservation: %w", err)
	}

	if existingBeneficiaryID != beneficiaryID {
		return false, ErrDuplicateOSCAID
	}

	// The same beneficiary already owns this ID.
	// This makes retries safe.
	return false, nil
}

// releaseOSCAID removes a reservation only when it belongs to the
// specified beneficiary.
func (r *Repository) releaseOSCAID(
	ctx context.Context,
	oscaID string,
	beneficiaryID gocql.UUID,
) error {
	applied, err := r.session.Query(
		`DELETE FROM beneficiaries_by_osca_id
WHERE osca_id = ?
IF beneficiary_id = ?`,
		oscaID,
		beneficiaryID,
	).WithContext(ctx).MapScanCAS(map[string]interface{}{})

	if err != nil {
		return fmt.Errorf("release OSCA ID reservation: %w", err)
	}

	if applied {
		return nil
	}

	// A missing reservation is already in the desired state.
	var existingBeneficiaryID gocql.UUID

	err = r.session.Query(
		`SELECT beneficiary_id
FROM beneficiaries_by_osca_id
WHERE osca_id = ?`,
		oscaID,
	).WithContext(ctx).Scan(&existingBeneficiaryID)

	if errors.Is(err, gocql.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("verify OSCA ID reservation release: %w", err)
	}

	if existingBeneficiaryID != beneficiaryID {
		return fmt.Errorf(
			"cannot release OSCA ID %q: reservation belongs to another beneficiary",
			oscaID,
		)
	}

	return fmt.Errorf(
		"OSCA ID reservation for beneficiary %s was not released",
		beneficiaryID,
	)
}
