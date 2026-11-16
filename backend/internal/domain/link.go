package domain

import "time"

type Link struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	OriginalURL string     `json:"original_url"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	ClickCount  int64      `json:"click_count,omitempty"`
}

func (l *Link) IsExpired() bool {
	if l.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*l.ExpiresAt)
}

// EffectiveStatus resolves the externally visible link state ("active" or "expired").
func (l *Link) EffectiveStatus() string {
	if l.IsExpired() {
		return "expired"
	}
	return "active"
}

