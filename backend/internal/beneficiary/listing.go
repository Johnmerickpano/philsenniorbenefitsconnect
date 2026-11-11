package beneficiary

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
)

const (
	DefaultBeneficiaryPageSize = 25
	MaxBeneficiaryPageSize     = 100
)

// ListByBarangay retrieves one alphabetically ordered page of beneficiaries.
// Pass the returned paging state to retrieve the next page.
// An empty paging state starts from the first page.
func (r *Repository) ListByBarangay(
	ctx context.Context,
	barangayCode string,
	pageSize int,
	pagingState []byte,
) ([]schema.BeneficiaryByBarangay, []byte, error) {
	if !r.initialized() {
		return nil, nil, errors.New("beneficiary repository is not initialized")
	}

	barangayCode = strings.TrimSpace(barangayCode)
	if barangayCode == "" {
		return nil, nil, ErrInvalidBeneficiary
	}

	if pageSize <= 0 {
		pageSize = DefaultBeneficiaryPageSize
	}

	if pageSize > MaxBeneficiaryPageSize {
		return nil, nil, ErrInvalidBeneficiary
	}

	query := r.session.Query(
		`SELECT
barangay_code, sort_name, beneficiary_id, osca_id,
display_name, birth_date, record_status, review_flag, updated_at
FROM beneficiaries_by_barangay
WHERE barangay_code = ?`,
		barangayCode,
	).WithContext(ctx).
		PageSize(pageSize).
		PageState(pagingState)

	iter := query.Iter()
	results := make([]schema.BeneficiaryByBarangay, 0, pageSize)

	for {
		var item schema.BeneficiaryByBarangay

		if !iter.Scan(
			&item.BarangayCode,
			&item.SortName,
			&item.BeneficiaryID,
			&item.OSCAID,
			&item.DisplayName,
			&item.BirthDate,
			&item.RecordStatus,
			&item.ReviewFlag,
			&item.UpdatedAt,
		) {
			break
		}

		results = append(results, item)
	}

	nextPagingState := iter.PageState()

	if err := iter.Close(); err != nil {
		return nil, nil, fmt.Errorf("list beneficiaries by barangay: %w", err)
	}

	return results, nextPagingState, nil
}

// ListByStatus retrieves beneficiaries by workflow status and barangay.
// Results are ordered by sort_name and use ScyllaDB paging state.
func (r *Repository) ListByStatus(
	ctx context.Context,
	status schema.BeneficiaryStatus,
	barangayCode string,
	pageSize int,
	pagingState []byte,
) ([]schema.BeneficiaryByStatus, []byte, error) {
	if !r.initialized() {
		return nil, nil, errors.New("beneficiary repository is not initialized")
	}

	if !validStatus(status) {
		return nil, nil, fmt.Errorf(
			"%w: invalid beneficiary status",
			ErrInvalidBeneficiary,
		)
	}

	barangayCode = strings.TrimSpace(barangayCode)
	if barangayCode == "" {
		return nil, nil, fmt.Errorf(
			"%w: barangay code is required",
			ErrInvalidBeneficiary,
		)
	}

	if pageSize <= 0 {
		pageSize = DefaultBeneficiaryPageSize
	}
	if pageSize > MaxBeneficiaryPageSize {
		return nil, nil, fmt.Errorf(
			"%w: page size exceeds maximum of %d",
			ErrInvalidBeneficiary,
			MaxBeneficiaryPageSize,
		)
	}

	query := r.session.Query(`
        SELECT record_status, barangay_code, sort_name,
               beneficiary_id, osca_id, display_name,
               birth_date, review_flag, updated_at
        FROM beneficiaries_by_status
        WHERE record_status = ? AND barangay_code = ?
    `,
		status,
		barangayCode,
	).WithContext(ctx).PageSize(pageSize).PageState(pagingState)

	iter := query.Iter()

	results := make([]schema.BeneficiaryByStatus, 0, pageSize)

	var item schema.BeneficiaryByStatus
	for iter.Scan(
		&item.RecordStatus,
		&item.BarangayCode,
		&item.SortName,
		&item.BeneficiaryID,
		&item.OSCAID,
		&item.DisplayName,
		&item.BirthDate,
		&item.ReviewFlag,
		&item.UpdatedAt,
	) {
		results = append(results, item)
		item = schema.BeneficiaryByStatus{}
	}

	nextState := iter.PageState()

	if err := iter.Close(); err != nil {
		return nil, nil, fmt.Errorf("list beneficiaries by status: %w", err)
	}

	return results, nextState, nil
}
