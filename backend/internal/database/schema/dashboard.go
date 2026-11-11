package schema

import "time"

// DashboardMetricCache represents the latest cached value of a dashboard metric.
type DashboardMetricCache struct {
	MetricKey   string    `json:"metric_key"`
	MetricValue int64     `json:"metric_value"`
	ComputedAt  time.Time `json:"computed_at"`
}

// DashboardMetricSnapshot represents a historical monthly metric snapshot.
type DashboardMetricSnapshot struct {
	MetricKey   string    `json:"metric_key"`
	MonthBucket string    `json:"month_bucket"`
	MetricValue int64     `json:"metric_value"`
	CapturedAt  time.Time `json:"captured_at"`
}
