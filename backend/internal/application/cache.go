package application

import (
	"context"
	"url-shortener/internal/domain"
)

type Cache interface {
	Get(ctx context.Context, key string) (*domain.Link, error)
	Set(ctx context.Context, key string, link *domain.Link, ttl int) error
	Delete(ctx context.Context, key string) error
}

