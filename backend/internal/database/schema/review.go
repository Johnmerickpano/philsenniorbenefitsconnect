package schema

import (
	"time"

	"github.com/gocql/gocql"
)

// Review flag categories.
type ReviewFlagType string

const (
	ReviewPossibleDuplicate    ReviewFlagType = "POSSIBLE_DUPLICATE"
	ReviewDeceasedVerification ReviewFlagType = "DECEASED_VERIFICATION"
	ReviewInconsistentInfo     ReviewFlagType = "INCONSISTENT_INFO"
	ReviewIncompleteRecord     ReviewFlagType = "INCOMPLETE_RECORD"
)

// Review flag workflow statuses.
type ReviewFlagStatus string

const (
	ReviewFlagOpen        ReviewFlagStatus = "OPEN"
	ReviewFlagUnderReview ReviewFlagStatus = "UNDER_REVIEW"
	ReviewFlagResolved    ReviewFlagStatus = "RESOLVED"
	ReviewFlagDismissed   ReviewFlagStatus = "DISMISSED"
)

// Sources that can raise a review flag.
type ReviewFlagSource string

const (
	ReviewSourceManual      ReviewFlagSource = "MANUAL"
	ReviewSourceImport      ReviewFlagSource = "IMPORT"
	ReviewSourceSystemCheck ReviewFlagSource = "SYSTEM_CHECK"
)

// ReviewFlag represents a staff-review item linked to a beneficiary.
type ReviewFlag struct {
	FlagID               gocql.UUID       `json:"flag_id"`
	BeneficiaryID        gocql.UUID       `json:"beneficiary_id"`
	RelatedBeneficiaryID *gocql.UUID      `json:"related_beneficiary_id,omitempty"`
	FlagType             ReviewFlagType   `json:"flag_type"`
	Status               ReviewFlagStatus `json:"status"`
	Source               ReviewFlagSource `json:"source"`
	Reason               *string          `json:"reason,omitempty"`
	RaisedBy             gocql.UUID       `json:"raised_by"`
	RaisedAt             time.Time        `json:"raised_at"`
	DecidedBy            *gocql.UUID      `json:"decided_by,omitempty"`
	DecidedAt            *time.Time       `json:"decided_at,omitempty"`
	Decision             *string          `json:"decision,omitempty"`
	DecisionRemarks      *string          `json:"decision_remarks,omitempty"`
}

// ReviewFlagOpen represents an item in the active review workset.
// Only OPEN and UNDER_REVIEW flags belong in this projection.
type ReviewFlagOpenEntry struct {
	FlagType             ReviewFlagType   `json:"flag_type"`
	RaisedAt             time.Time        `json:"raised_at"`
	FlagID               gocql.UUID       `json:"flag_id"`
	BeneficiaryID        gocql.UUID       `json:"beneficiary_id"`
	RelatedBeneficiaryID *gocql.UUID      `json:"related_beneficiary_id,omitempty"`
	Status               ReviewFlagStatus `json:"status"`
	Reason               *string          `json:"reason,omitempty"`
}

// ReviewFlagClosed represents a decided flag in a monthly partition.
type ReviewFlagClosed struct {
	FlagType      ReviewFlagType   `json:"flag_type"`
	MonthBucket   string           `json:"month_bucket"`
	DecidedAt     time.Time        `json:"decided_at"`
	FlagID        gocql.UUID       `json:"flag_id"`
	BeneficiaryID gocql.UUID       `json:"beneficiary_id"`
	Status        ReviewFlagStatus `json:"status"`
	Decision      *string          `json:"decision,omitempty"`
}

// ReviewFlagByBeneficiary represents a beneficiary's review history entry.
type ReviewFlagByBeneficiary struct {
	BeneficiaryID gocql.UUID       `json:"beneficiary_id"`
	RaisedAt      time.Time        `json:"raised_at"`
	FlagID        gocql.UUID       `json:"flag_id"`
	FlagType      ReviewFlagType   `json:"flag_type"`
	Status        ReviewFlagStatus `json:"status"`
}
