package beneficiary

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
)

// matchesNameSearch checks whether any displayed name component starts
// with the normalized search term.
func matchesNameSearch(displayName *string, searchKey string) bool {
	if displayName == nil || searchKey == "" {
		return false
	}

	searchKey = normalizeNameSearchKey(searchKey)
	if searchKey == "" {
		return false
	}

	for _, part := range strings.Fields(*displayName) {
		if strings.HasPrefix(normalizeNameSearchKey(part), searchKey) {
			return true
		}
	}

	return false
}

// SearchByName searches one barangay's three-character name-prefix bucket.
// The search term must contain at least three Unicode letters or digits.
// Results are filtered by the full normalized name-component prefix.
//
// Each call processes at most one ScyllaDB page. A page can contain fewer
// matches than pageSize because unrelated prefix collisions are filtered out.
// Continue while the returned paging state is non-empty.
func (r *Repository) SearchByName(
	ctx context.Context,
	barangayCode string,
	searchTerm string,
	pageSize int,
	pagingState []byte,
) ([]schema.BeneficiarySearch, []byte, error) {
	if !r.initialized() {
		return nil, nil, errors.New("beneficiary repository is not initialized")
	}

	barangayCode = strings.TrimSpace(barangayCode)
	searchTerm = strings.TrimSpace(searchTerm)

	if barangayCode == "" {
		return nil, nil, fmt.Errorf(
			"%w: barangay code is required",
			ErrInvalidBeneficiary,
		)
	}

	searchKey := normalizeNameSearchKey(searchTerm)
	namePrefix := searchNamePrefix(searchTerm)

	if namePrefix == "" || searchKey == "" {
		return nil, nil, fmt.Errorf(
			"%w: name search requires at least three letters or digits",
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
        SELECT barangay_code, name_prefix, sort_name,
               beneficiary_id, osca_id, display_name, record_status
        FROM beneficiary_search
        WHERE barangay_code = ? AND name_prefix = ?
    `,
		barangayCode,
		namePrefix,
	).WithContext(ctx).
		PageSize(pageSize).
		PageState(pagingState)

	iter := query.Iter()

	results := make([]schema.BeneficiarySearch, 0, pageSize)

	// Read no more than one database page. This keeps the returned paging
	// state aligned with the rows consumed by this request.
	for scanned := 0; scanned < pageSize; scanned++ {
		var item schema.BeneficiarySearch

		if !iter.Scan(
			&item.BarangayCode,
			&item.NamePrefix,
			&item.SortName,
			&item.BeneficiaryID,
			&item.OSCAID,
			&item.DisplayName,
			&item.RecordStatus,
		) {
			break
		}

		if matchesNameSearch(item.DisplayName, searchKey) {
			results = append(results, item)
		}
	}

	nextPagingState := iter.PageState()

	if err := iter.Close(); err != nil {
		return nil, nil, fmt.Errorf("search beneficiaries by name: %w", err)
	}

	return results, nextPagingState, nil
}
