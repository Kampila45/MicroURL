package application

import (
	"context"
	"fmt"
	"log"
	"url-shortener/internal/domain"
)

type ResolveLinkUseCase struct {
	repo  LinkRepository
	cache Cache
	ttl   int
}

func NewResolveLinkUseCase(repo LinkRepository, cache Cache, ttl int) *ResolveLinkUseCase {
	return &ResolveLinkUseCase{
		repo:  repo,
		cache: cache,
		ttl:   ttl,
	}
}

type ResolveLinkRequest struct {
	Slug string
}

type ResolveLinkResponse struct {
	OriginalURL string
}

func (uc *ResolveLinkUseCase) Execute(ctx context.Context, req ResolveLinkRequest) (*ResolveLinkResponse, error) {
	cacheKey := fmt.Sprintf("link:slug:%s", req.Slug)

	cached, err := uc.cache.Get(ctx, cacheKey)
	if err == nil {
		if cached.IsExpired() {
			_ = uc.cache.Delete(ctx, cacheKey)
			return nil, domain.ErrLinkExpired
		}
		return &ResolveLinkResponse{OriginalURL: cached.OriginalURL}, nil
	}

	if err != ErrCacheMiss {
		log.Printf("warning: cache unavailable, falling back to database: %v", err)
	}

	link, err := uc.repo.FindBySlug(ctx, req.Slug)
	if err != nil {
		return nil, fmt.Errorf("failed to find link by slug: %w", err)
	}

	if link.IsExpired() {
		return nil, domain.ErrLinkExpired
	}

	_ = uc.cache.Set(ctx, cacheKey, link, uc.ttl)

	return &ResolveLinkResponse{OriginalURL: link.OriginalURL}, nil
}

