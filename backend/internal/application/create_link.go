package application

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
	"url-shortener/internal/domain"
)

const maxSlugCreateRetries = 5

type CreateLinkUseCase struct {
	repo          LinkRepository
	slugGenerator *SlugGenerator
	cache         Cache
	ttl           int
}

func NewCreateLinkUseCase(repo LinkRepository, slugGenerator *SlugGenerator, cache Cache, ttl int) *CreateLinkUseCase {
	return &CreateLinkUseCase{
		repo:          repo,
		slugGenerator: slugGenerator,
		cache:         cache,
		ttl:           ttl,
	}
}

type CreateLinkRequest struct {
	OriginalURL string
	CustomSlug  string
	ExpiresAt   *time.Time
}

type CreateLinkResponse struct {
	Link *domain.Link
}

func (uc *CreateLinkUseCase) Execute(ctx context.Context, req CreateLinkRequest) (*CreateLinkResponse, error) {
	validatedURL, err := domain.ValidateDestinationURL(req.OriginalURL)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if req.ExpiresAt != nil && !req.ExpiresAt.After(now) {
		return nil, domain.ErrExpiredTimestamp
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ID: %w", err)
	}

	customSlug := strings.TrimSpace(req.CustomSlug)
	if customSlug != "" {
		normalizedAlias, err := domain.ValidateCustomAlias(customSlug)
		if err != nil {
			return nil, err
		}

		link := &domain.Link{
			ID:          id,
			Slug:        normalizedAlias,
			OriginalURL: validatedURL,
			CreatedAt:   now,
			ExpiresAt:   req.ExpiresAt,
		}
		if err := uc.repo.Create(ctx, link); err != nil {
			if errors.Is(err, domain.ErrInvalidSlug) {
				return nil, fmt.Errorf("alias %q already exists: %w", normalizedAlias, domain.ErrAliasConflict)
			}
			return nil, fmt.Errorf("failed to create link in repository: %w", err)
		}

		cacheKey := fmt.Sprintf("link:slug:%s", link.Slug)
		_ = uc.cache.Set(ctx, cacheKey, link, uc.ttl)
		return &CreateLinkResponse{Link: link}, nil
	}

	var createdLink *domain.Link
	for attempt := 0; attempt < maxSlugCreateRetries; attempt++ {
		slug, err := uc.slugGenerator.Generate(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to generate slug: %w", err)
		}

		candidate := &domain.Link{
			ID:          id,
			Slug:        slug,
			OriginalURL: validatedURL,
			CreatedAt:   now,
			ExpiresAt:   req.ExpiresAt,
		}
		err = uc.repo.Create(ctx, candidate)
		if err == nil {
			createdLink = candidate
			break
		}
		if errors.Is(err, domain.ErrInvalidSlug) {
			continue
		}
		return nil, fmt.Errorf("failed to create link in repository: %w", err)
	}

	if createdLink == nil {
		return nil, fmt.Errorf("failed to persist unique generated slug after %d attempts: %w", maxSlugCreateRetries, domain.ErrInvalidSlug)
	}

	cacheKey := fmt.Sprintf("link:slug:%s", createdLink.Slug)
	_ = uc.cache.Set(ctx, cacheKey, createdLink, uc.ttl)

	return &CreateLinkResponse{Link: createdLink}, nil
}

func generateID() (string, error) {
	randomStr, err := generateRandomString(16)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomStr), nil
}

func generateRandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random character: %w", err)
		}
		b[i] = charset[num.Int64()]
	}
	return string(b), nil
}

