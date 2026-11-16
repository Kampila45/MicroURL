package domain

import "time"

// DailyClickCount stores aggregate redirects for a single UTC calendar date.
type DailyClickCount struct {
	Date   string `json:"date"`
	Clicks int64  `json:"clicks"`
}

// LinkStats summarizes aggregate redirect telemetry for a link without visitor PII.
type LinkStats struct {
	Slug        string            `json:"slug"`
	OriginalURL string            `json:"original_url"`
	Status      string            `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	TotalClicks int64             `json:"total_clicks"`
	Timezone    string            `json:"timezone"`
	RangeDays   int               `json:"range_days"`
	Daily       []DailyClickCount `json:"daily"`
}

