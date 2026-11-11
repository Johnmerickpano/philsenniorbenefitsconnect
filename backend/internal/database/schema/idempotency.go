package schema

import (
	"time"

	"github.com/gocql/gocql"
)

// IdempotencyKey represents a request key used to prevent duplicate processing.
type IdempotencyKey struct {
	IdempotencyKey string     `json:"idempotency_key"`
	UserID         gocql.UUID `json:"user_id"`
	RequestHash    string     `json:"request_hash"`
	ResultRef      *string    `json:"result_ref,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}
