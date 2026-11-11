package schema

import "github.com/gocql/gocql"

// Notification represents an in-app notification belonging to a user.
type Notification struct {
	UserID         gocql.UUID `json:"user_id"`
	NotificationID gocql.UUID `json:"notification_id"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	Category       string     `json:"category"`
	IsRead         bool       `json:"is_read"`
}
