package application

import (
	"context"
	"fmt"
	"log"
	"time"
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
	Slug       string
	CountClick bool
}

type ResolveLinkResponse struct {
	OriginalURL string
	CacheHit    bool
}

func (uc *ResolveLinkUseCase) Execute(ctx context.Context, req ResolveLinkRequest) (*ResolveLinkResponse, error) {
	cacheKey := fmt.Sprintf("link:slug:%s", req.Slug)

	cached, err := uc.cache.Get(ctx, cacheKey)
	if err == nil {
		if cached.IsExpired() {
			_ = uc.cache.Delete(ctx, cacheKey)
			return nil, domain.ErrLinkExpired
		}
		if req.CountClick {
			if clickErr := uc.repo.RecordClick(ctx, cached.ID, time.Now().UTC()); clickErr != nil {
				log.Printf("warning: failed to record redirect click for %s: %v", req.Slug, clickErr)
			}
		}
		return &ResolveLinkResponse{OriginalURL: cached.OriginalURL, CacheHit: true}, nil
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

	if req.CountClick {
		if clickErr := uc.repo.RecordClick(ctx, link.ID, time.Now().UTC()); clickErr != nil {
			log.Printf("warning: failed to record redirect click for %s: %v", req.Slug, clickErr)
		}
	}

	return &ResolveLinkResponse{OriginalURL: link.OriginalURL}, nil
}
