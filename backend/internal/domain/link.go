package domain

import "time"

type Link struct {
	ID          string
	Slug        string
	OriginalURL string
	CreatedAt   time.Time
	ExpiresAt   *time.Time
}

func (l *Link) IsExpired() bool {
	if l.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*l.ExpiresAt)
}

