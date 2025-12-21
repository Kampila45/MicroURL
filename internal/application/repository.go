package application

import (
	"context"
	"url-shortener/internal/domain"
)

type LinkRepository interface {
	Create(ctx context.Context, link *domain.Link) error
	FindBySlug(ctx context.Context, slug string) (*domain.Link, error)
	Exists(ctx context.Context, slug string) (bool, error)
}

