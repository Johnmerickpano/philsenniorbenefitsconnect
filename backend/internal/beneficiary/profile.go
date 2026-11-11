package beneficiary

import "time"

// BeneficiaryProfile contains the editable fields of a beneficiary record.
// It represents a full profile replacement, not a partial update.
// Nil optional fields mean that the corresponding value should be cleared.
type BeneficiaryProfile struct {
	OSCAID       *string
	FirstName    *string
	MiddleName   *string
	LastName     *string
	Suffix       *string
	BirthDate    *time.Time
	Sex          *string
	BarangayCode *string
	BarangayName *string
}
