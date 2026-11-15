package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"url-shortener/internal/application"
	"url-shortener/internal/domain"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) application.LinkRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, link *domain.Link) error {
	query := `INSERT INTO links (id, slug, original_url, created_at, expires_at) VALUES ($1, $2, $3, $4, $5)`
	_, err := r.db.ExecContext(ctx, query, link.ID, link.Slug, link.OriginalURL, link.CreatedAt, link.ExpiresAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("slug already exists: %w", domain.ErrInvalidSlug)
		}
		return fmt.Errorf("database error: %w", err)
	}
	return nil
}

func (r *PostgresRepository) FindBySlug(ctx context.Context, slug string) (*domain.Link, error) {
	query := `SELECT id, slug, original_url, created_at, expires_at FROM links WHERE slug = $1`
	row := r.db.QueryRowContext(ctx, query, slug)

	var link domain.Link
	var expiresAt sql.NullTime

	err := row.Scan(&link.ID, &link.Slug, &link.OriginalURL, &link.CreatedAt, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrLinkNotFound
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	if expiresAt.Valid {
		link.ExpiresAt = &expiresAt.Time
	}

	return &link, nil
}

func (r *PostgresRepository) Exists(ctx context.Context, slug string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM links WHERE slug = $1)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, slug).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("database error: %w", err)
	}
	return exists, nil
}

