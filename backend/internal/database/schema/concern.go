package schema

import (
	"time"

	"github.com/gocql/gocql"
)

// Concern workflow statuses.
type ConcernStatus string

const (
	ConcernOpen       ConcernStatus = "OPEN"
	ConcernInProgress ConcernStatus = "IN_PROGRESS"
	ConcernResolved   ConcernStatus = "RESOLVED"
	ConcernClosed     ConcernStatus = "CLOSED"
)

// Concern represents a beneficiary concern or follow-up case.
type Concern struct {
	ConcernID     gocql.UUID    `json:"concern_id"`
	BeneficiaryID gocql.UUID    `json:"beneficiary_id"`
	SubmittedBy   gocql.UUID    `json:"submitted_by"`
	Category      *string       `json:"category,omitempty"`
	Subject       *string       `json:"subject,omitempty"`
	Description   *string       `json:"description,omitempty"`
	Status        ConcernStatus `json:"status"`
	AssignedTo    *gocql.UUID   `json:"assigned_to,omitempty"`
	CreatedAt     *time.Time    `json:"created_at,omitempty"`
	UpdatedAt     *time.Time    `json:"updated_at,omitempty"`
	ResolvedAt    *time.Time    `json:"resolved_at,omitempty"`
}

// ConcernByBeneficiary represents a beneficiary's concern-history entry.
type ConcernByBeneficiary struct {
	BeneficiaryID gocql.UUID    `json:"beneficiary_id"`
	CreatedAt     time.Time     `json:"created_at"`
	ConcernID     gocql.UUID    `json:"concern_id"`
	Subject       *string       `json:"subject,omitempty"`
	Status        ConcernStatus `json:"status"`
	UpdatedAt     *time.Time    `json:"updated_at,omitempty"`
}

// ConcernOpen represents an entry in the active staff follow-up workset.
// Only OPEN and IN_PROGRESS concerns belong in this projection.
type ConcernOpenEntry struct {
	Status        ConcernStatus `json:"status"`
	CreatedAt     time.Time     `json:"created_at"`
	ConcernID     gocql.UUID    `json:"concern_id"`
	BeneficiaryID gocql.UUID    `json:"beneficiary_id"`
	Category      *string       `json:"category,omitempty"`
	Subject       *string       `json:"subject,omitempty"`
	AssignedTo    *gocql.UUID   `json:"assigned_to,omitempty"`
}

// ConcernUpdate represents an append-only update to a concern.
type ConcernUpdate struct {
	ConcernID        gocql.UUID     `json:"concern_id"`
	UpdateID         gocql.UUID     `json:"update_id"`
	AuthorID         gocql.UUID     `json:"author_id"`
	AuthorRole       string         `json:"author_role"`
	Message          string         `json:"message"`
	NewStatus        *ConcernStatus `json:"new_status,omitempty"`
	VisibleToCitizen bool           `json:"visible_to_citizen"`
}
