package application

import (
	"context"
	"time"
	"url-shortener/internal/domain"
)

type LinkRepository interface {
	Create(ctx context.Context, link *domain.Link) error
	FindBySlug(ctx context.Context, slug string) (*domain.Link, error)
	Exists(ctx context.Context, slug string) (bool, error)
	RecordClick(ctx context.Context, linkID string, at time.Time) error
	GetStats(ctx context.Context, slug string, startDate time.Time, days int) (*domain.LinkStats, error)
}

