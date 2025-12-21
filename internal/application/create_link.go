package application

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
	"url-shortener/internal/domain"
)

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
	ExpiresAt   *time.Time
}

type CreateLinkResponse struct {
	Link *domain.Link
}

func (uc *CreateLinkUseCase) Execute(ctx context.Context, req CreateLinkRequest) (*CreateLinkResponse, error) {
	slug, err := uc.slugGenerator.Generate(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate slug: %w", err)
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ID: %w", err)
	}

	link := &domain.Link{
		ID:          id,
		Slug:        slug,
		OriginalURL: req.OriginalURL,
		CreatedAt:   time.Now(),
		ExpiresAt:   req.ExpiresAt,
	}

	if err := uc.repo.Create(ctx, link); err != nil {
		return nil, fmt.Errorf("failed to create link in repository: %w", err)
	}

	cacheKey := fmt.Sprintf("link:slug:%s", link.Slug)
	_ = uc.cache.Set(ctx, cacheKey, link, uc.ttl)

	return &CreateLinkResponse{Link: link}, nil
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

