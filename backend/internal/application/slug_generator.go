package application

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
)

const (
	slugLength    = 8
	maxRetries    = 10
	alphanumeric  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

type SlugGenerator struct {
	repo LinkRepository
}

func NewSlugGenerator(repo LinkRepository) *SlugGenerator {
	return &SlugGenerator{repo: repo}
}

func (g *SlugGenerator) Generate(ctx context.Context) (string, error) {
	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		slug, err := g.generateRandomSlug()
		if err != nil {
			return "", fmt.Errorf("failed to generate random slug: %w", err)
		}

		exists, err := g.repo.Exists(ctx, slug)
		if err != nil {
			return "", fmt.Errorf("failed to check slug existence: %w", err)
		}

		if !exists {
			return slug, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique slug after %d attempts", maxRetries)
}

func (g *SlugGenerator) generateRandomSlug() (string, error) {
	chars := make([]byte, slugLength)
	for i := range chars {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphanumeric))))
		if err != nil {
			return "", err
		}
		chars[i] = alphanumeric[num.Int64()]
	}
	return string(chars), nil
}

