package beneficiary

import (
	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/database/schema"
	"github.com/gocql/gocql"
)

// addCreateBatchStatements adds the primary record and all applicable
// query projections to the same logged batch.
func addCreateBatchStatements(
	batch *gocql.Batch,
	b *schema.Beneficiary,
) {
	batch.Query(
		`INSERT INTO beneficiaries_by_id (
beneficiary_id, osca_id, first_name, middle_name,
last_name, suffix, sort_name, birth_date, sex,
barangay_code, barangay_name, record_status,
last_verified_at, last_verified_by, row_version,
created_at, created_by, updated_at, updated_by,
archived_at, archived_by, archive_reason
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.BeneficiaryID,
		b.OSCAID,
		b.FirstName,
		b.MiddleName,
		b.LastName,
		b.Suffix,
		b.SortName,
		b.BirthDate,
		b.Sex,
		b.BarangayCode,
		b.BarangayName,
		string(b.RecordStatus),
		b.LastVerifiedAt,
		b.LastVerifiedBy,
		b.RowVersion,
		b.CreatedAt,
		b.CreatedBy,
		b.UpdatedAt,
		b.UpdatedBy,
		b.ArchivedAt,
		b.ArchivedBy,
		b.ArchiveReason,
	)

	// Barangay-based projections require a barangay partition key.
	if b.BarangayCode == nil {
		return
	}

	name := displayName(b)
	sortName := optionalStringValue(b.SortName)

	batch.Query(
		`INSERT INTO beneficiaries_by_barangay (
barangay_code, sort_name, beneficiary_id, osca_id,
display_name, birth_date, record_status, review_flag, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		*b.BarangayCode,
		sortName,
		b.BeneficiaryID,
		b.OSCAID,
		name,
		b.BirthDate,
		string(b.RecordStatus),
		nil,
		b.UpdatedAt,
	)

	batch.Query(
		`INSERT INTO beneficiaries_by_status (
record_status, barangay_code, sort_name, beneficiary_id,
osca_id, display_name, birth_date, review_flag, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(b.RecordStatus),
		*b.BarangayCode,
		sortName,
		b.BeneficiaryID,
		b.OSCAID,
		name,
		b.BirthDate,
		nil,
		b.UpdatedAt,
	)

	// Search rows are generated for first, middle, and last name prefixes.
	for _, prefix := range nameSearchPrefixes(b) {
		batch.Query(
			`INSERT INTO beneficiary_search (
barangay_code, name_prefix, sort_name, beneficiary_id,
osca_id, display_name, record_status
) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			*b.BarangayCode,
			prefix,
			sortName,
			b.BeneficiaryID,
			b.OSCAID,
			name,
			string(b.RecordStatus),
		)
	}
}
