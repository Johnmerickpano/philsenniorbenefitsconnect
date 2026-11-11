package schema

import (
	"time"

	"github.com/gocql/gocql"
)

// ImportBatch represents an Excel masterlist import batch.
type ImportBatch struct {
	BatchID       gocql.UUID `json:"batch_id"`
	FileName      string     `json:"file_name"`
	FileHash      string     `json:"file_hash"`
	Status        string     `json:"status"`
	ColumnMapping string     `json:"column_mapping"`
	TotalRows     int        `json:"total_rows"`
	ValidRows     int        `json:"valid_rows"`
	ErrorRows     int        `json:"error_rows"`
	DuplicateRows int        `json:"duplicate_rows"`
	CreatedBy     gocql.UUID `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	ConfirmedBy   gocql.UUID `json:"confirmed_by"`
	ConfirmedAt   time.Time  `json:"confirmed_at"`
	CompletedAt   time.Time  `json:"completed_at"`
}

// ImportBatchByMonth represents the monthly import batch listing.
type ImportBatchByMonth struct {
	MonthBucket string     `json:"month_bucket"`
	CreatedAt   time.Time  `json:"created_at"`
	BatchID     gocql.UUID `json:"batch_id"`
	FileName    string     `json:"file_name"`
	Status      string     `json:"status"`
}

// ImportFileHash prevents duplicate file imports.
type ImportFileHash struct {
	FileHash  string     `json:"file_hash"`
	BatchID   gocql.UUID `json:"batch_id"`
	CreatedAt time.Time  `json:"created_at"`
}

// ImportRow represents a staged row from an imported file.
type ImportRow struct {
	BatchID        gocql.UUID `json:"batch_id"`
	RowNumber      int        `json:"row_number"`
	RawJSON        string     `json:"raw_json"`
	NormalizedJSON string     `json:"normalized_json"`
	RowStatus      string     `json:"row_status"`
	Action         string     `json:"action"`
}

// ImportError represents a validation error for an imported row.
type ImportError struct {
	BatchID   gocql.UUID `json:"batch_id"`
	RowNumber int        `json:"row_number"`
	ErrorCode string     `json:"error_code"`
	FieldName string     `json:"field_name"`
	Message   string     `json:"message"`
}
