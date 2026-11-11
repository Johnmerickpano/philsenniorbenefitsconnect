package schema

import (
	"time"

	"github.com/gocql/gocql"
	"gopkg.in/inf.v0"
)

// Payout schedule workflow statuses.
type PayoutScheduleStatus string

const (
	PayoutScheduleDraft     PayoutScheduleStatus = "DRAFT"
	PayoutSchedulePublished PayoutScheduleStatus = "PUBLISHED"
	PayoutScheduleCancelled PayoutScheduleStatus = "CANCELLED"
)

// PayoutSchedule represents the primary payout schedule.
type PayoutSchedule struct {
	ScheduleID              gocql.UUID           `json:"schedule_id"`
	Title                   *string              `json:"title,omitempty"`
	YearQuarter             *string              `json:"year_quarter,omitempty"`
	PayoutDate              *time.Time           `json:"payout_date,omitempty"`
	PayoutTime              *int64               `json:"payout_time,omitempty"` // Nanoseconds since midnight.
	VenueName               *string              `json:"venue_name,omitempty"`
	VenueAddress            *string              `json:"venue_address,omitempty"`
	ApplicableBarangayCodes []string             `json:"applicable_barangay_codes,omitempty"`
	AmountPerBeneficiary    *inf.Dec             `json:"amount_per_beneficiary,omitempty"`
	Instructions            *string              `json:"instructions,omitempty"`
	Status                  PayoutScheduleStatus `json:"status"`
	RowVersion              int64                `json:"row_version"`
	CreatedAt               *time.Time           `json:"created_at,omitempty"`
	CreatedBy               *gocql.UUID          `json:"created_by,omitempty"`
	UpdatedAt               *time.Time           `json:"updated_at,omitempty"`
	UpdatedBy               *gocql.UUID          `json:"updated_by,omitempty"`
	PublishedAt             *time.Time           `json:"published_at,omitempty"`
	PublishedBy             *gocql.UUID          `json:"published_by,omitempty"`
}

// ScheduleByQuarterStatus represents an admin schedule-list projection.
type ScheduleByQuarterStatus struct {
	YearQuarter string               `json:"year_quarter"`
	Status      PayoutScheduleStatus `json:"status"`
	PayoutDate  time.Time            `json:"payout_date"`
	ScheduleID  gocql.UUID           `json:"schedule_id"`
	Title       *string              `json:"title,omitempty"`
	PayoutTime  *int64               `json:"payout_time,omitempty"`
	VenueName   *string              `json:"venue_name,omitempty"`
}

// ScheduleByBarangay represents a published citizen-facing schedule.
type ScheduleByBarangay struct {
	BarangayCode         string     `json:"barangay_code"`
	YearQuarter          string     `json:"year_quarter"`
	PayoutDate           time.Time  `json:"payout_date"`
	ScheduleID           gocql.UUID `json:"schedule_id"`
	Title                *string    `json:"title,omitempty"`
	PayoutTime           *int64     `json:"payout_time,omitempty"`
	VenueName            *string    `json:"venue_name,omitempty"`
	VenueAddress         *string    `json:"venue_address,omitempty"`
	AmountPerBeneficiary *inf.Dec   `json:"amount_per_beneficiary,omitempty"`
	Instructions         *string    `json:"instructions,omitempty"`
	PublishedAt          *time.Time `json:"published_at,omitempty"`
}

// ScheduleHistoryAction identifies a schedule lifecycle event.
type ScheduleHistoryAction string

const (
	ScheduleCreated         ScheduleHistoryAction = "CREATED"
	ScheduleEdited          ScheduleHistoryAction = "EDITED"
	SchedulePublishedAction ScheduleHistoryAction = "PUBLISHED"
	ScheduleCancelledAction ScheduleHistoryAction = "CANCELLED"
)

// ScheduleHistory is an append-only schedule event.
type ScheduleHistory struct {
	ScheduleID gocql.UUID            `json:"schedule_id"`
	EventID    gocql.UUID            `json:"event_id"`
	ChangedAt  time.Time             `json:"changed_at"`
	Action     ScheduleHistoryAction `json:"action"`
	ChangedBy  gocql.UUID            `json:"changed_by"`
	Summary    *string               `json:"summary,omitempty"`
}

// PayoutClaimMethod identifies who received the payout.
type PayoutClaimMethod string

const (
	PayoutClaimSelf           PayoutClaimMethod = "SELF"
	PayoutClaimRepresentative PayoutClaimMethod = "REPRESENTATIVE"
)

// PayoutClaim records that a payout was received.
// This record does not represent a money transfer or payment transaction.
type PayoutClaim struct {
	ScheduleID       gocql.UUID        `json:"schedule_id"`
	ClaimMethod      PayoutClaimMethod `json:"claim_method"`
	BeneficiaryID    gocql.UUID        `json:"beneficiary_id"`
	RepresentativeID *gocql.UUID       `json:"rep_id,omitempty"`
	RecordedBy       gocql.UUID        `json:"recorded_by"`
	RecordedAt       time.Time         `json:"recorded_at"`
	Remarks          *string           `json:"remarks,omitempty"`
}

// PayoutClaimByBeneficiary represents a beneficiary claim-history entry.
type PayoutClaimByBeneficiary struct {
	BeneficiaryID    gocql.UUID        `json:"beneficiary_id"`
	RecordedAt       time.Time         `json:"recorded_at"`
	ScheduleID       gocql.UUID        `json:"schedule_id"`
	ClaimMethod      PayoutClaimMethod `json:"claim_method"`
	RepresentativeID *gocql.UUID       `json:"rep_id,omitempty"`
}
