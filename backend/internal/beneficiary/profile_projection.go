package beneficiary

import (
	"context"
	"fmt"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
	"github.com/gocql/gocql"
)

func (r *Repository) readReviewFlag(
	ctx context.Context,
	current *schema.Beneficiary,
) (*string, error) {
	if current == nil || current.BarangayCode == nil {
		return nil, nil
	}

	sortName := optionalStringValue(current.SortName)
	var reviewFlag *string

	err := r.session.Query(
		`SELECT review_flag
FROM beneficiaries_by_barangay
WHERE barangay_code = ?
AND sort_name = ?
AND beneficiary_id = ?`,
		*current.BarangayCode,
		sortName,
		current.BeneficiaryID,
	).WithContext(ctx).Scan(&reviewFlag)

	if err != nil && err != gocql.ErrNotFound {
		return nil, fmt.Errorf("read beneficiary review flag: %w", err)
	}

	return reviewFlag, nil
}

func containsPrefix(prefixes []string, target string) bool {
	for _, prefix := range prefixes {
		if prefix == target {
			return true
		}
	}
	return false
}

// addProfileProjectionStatements updates listing and search projections.
// Existing primary keys are updated in place; obsolete keys are deleted.
func addProfileProjectionStatements(
	batch *gocql.Batch,
	current *schema.Beneficiary,
	updated *schema.Beneficiary,
	reviewFlag *string,
) {
	oldBarangay := optionalStringValue(current.BarangayCode)
	oldSortName := optionalStringValue(current.SortName)
	newBarangay := optionalStringValue(updated.BarangayCode)
	newSortName := optionalStringValue(updated.SortName)

	oldHasBarangay := current.BarangayCode != nil
	newHasBarangay := updated.BarangayCode != nil

	listingKeyChanged := oldHasBarangay &&
		(!newHasBarangay ||
			oldBarangay != newBarangay ||
			oldSortName != newSortName)

	if listingKeyChanged {
		batch.Query(
			`DELETE FROM beneficiaries_by_barangay
WHERE barangay_code = ?
AND sort_name = ?
AND beneficiary_id = ?`,
			oldBarangay,
			oldSortName,
			current.BeneficiaryID,
		)

		batch.Query(
			`DELETE FROM beneficiaries_by_status
WHERE record_status = ?
AND barangay_code = ?
AND sort_name = ?
AND beneficiary_id = ?`,
			string(current.RecordStatus),
			oldBarangay,
			oldSortName,
			current.BeneficiaryID,
		)
	}

	newPrefixes := nameSearchPrefixes(updated)

	if oldHasBarangay {
		for _, prefix := range nameSearchPrefixes(current) {
			obsolete := !newHasBarangay ||
				oldBarangay != newBarangay ||
				oldSortName != newSortName ||
				!containsPrefix(newPrefixes, prefix)

			if obsolete {
				batch.Query(
					`DELETE FROM beneficiary_search
WHERE barangay_code = ?
AND name_prefix = ?
AND sort_name = ?
AND beneficiary_id = ?`,
					oldBarangay,
					prefix,
					oldSortName,
					current.BeneficiaryID,
				)
			}
		}
	}

	if !newHasBarangay {
		return
	}

	name := displayName(updated)

	batch.Query(
		`INSERT INTO beneficiaries_by_barangay (
barangay_code, sort_name, beneficiary_id, osca_id,
display_name, birth_date, record_status, review_flag, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newBarangay,
		newSortName,
		updated.BeneficiaryID,
		updated.OSCAID,
		name,
		updated.BirthDate,
		string(updated.RecordStatus),
		reviewFlag,
		updated.UpdatedAt,
	)

	batch.Query(
		`INSERT INTO beneficiaries_by_status (
record_status, barangay_code, sort_name, beneficiary_id,
osca_id, display_name, birth_date, review_flag, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(updated.RecordStatus),
		newBarangay,
		newSortName,
		updated.BeneficiaryID,
		updated.OSCAID,
		name,
		updated.BirthDate,
		reviewFlag,
		updated.UpdatedAt,
	)

	for _, prefix := range newPrefixes {
		batch.Query(
			`INSERT INTO beneficiary_search (
barangay_code, name_prefix, sort_name, beneficiary_id,
osca_id, display_name, record_status
) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			newBarangay,
			prefix,
			newSortName,
			updated.BeneficiaryID,
			updated.OSCAID,
			name,
			string(updated.RecordStatus),
		)
	}
}
