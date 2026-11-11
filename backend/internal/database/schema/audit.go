package schema

import (
	"time"

	"github.com/gocql/gocql"
)

// AuditResult represents the outcome of an audited operation.
type AuditResult string

const (
	AuditSuccess AuditResult = "SUCCESS"
	AuditDenied  AuditResult = "DENIED"
	AuditFailed  AuditResult = "FAILED"
)

// AuditEvent represents an append-only security and activity event.
type AuditEvent struct {
	DayBucket  time.Time   `json:"day_bucket"`
	EventID    gocql.UUID  `json:"event_id"`
	EventTime  time.Time   `json:"event_time"`
	ActorID    gocql.UUID  `json:"actor_id"`
	ActorRole  string      `json:"actor_role"`
	Action     string      `json:"action"`
	EntityType string      `json:"entity_type"`
	EntityID   string      `json:"entity_id"`
	Result     AuditResult `json:"result"`
	BeforeJSON *string     `json:"before_json,omitempty"`
	AfterJSON  *string     `json:"after_json,omitempty"`
	IPAddress  *string     `json:"ip_address,omitempty"`
	RequestID  *string     `json:"request_id,omitempty"`
}

// AuditByEntity represents an audit event in an entity's history.
type AuditByEntity struct {
	EntityType string      `json:"entity_type"`
	EntityID   string      `json:"entity_id"`
	EventID    gocql.UUID  `json:"event_id"`
	EventTime  time.Time   `json:"event_time"`
	ActorID    gocql.UUID  `json:"actor_id"`
	ActorRole  string      `json:"actor_role"`
	Action     string      `json:"action"`
	Result     AuditResult `json:"result"`
	BeforeJSON *string     `json:"before_json,omitempty"`
	AfterJSON  *string     `json:"after_json,omitempty"`
}

// AuditByActor represents an audit event in an actor's activity history.
type AuditByActor struct {
	ActorID    gocql.UUID  `json:"actor_id"`
	EventID    gocql.UUID  `json:"event_id"`
	EventTime  time.Time   `json:"event_time"`
	ActorRole  string      `json:"actor_role"`
	Action     string      `json:"action"`
	EntityType string      `json:"entity_type"`
	EntityID   string      `json:"entity_id"`
	Result     AuditResult `json:"result"`
}
