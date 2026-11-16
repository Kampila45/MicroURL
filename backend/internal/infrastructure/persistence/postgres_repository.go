package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
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

// EnsureSchema idempotently provisions and upgrades the links and daily click tables.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS links (
			id TEXT PRIMARY KEY,
			slug TEXT UNIQUE NOT NULL,
			original_url TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			expires_at TIMESTAMP,
			click_count BIGINT NOT NULL DEFAULT 0
		)`,
		`ALTER TABLE links ADD COLUMN IF NOT EXISTS click_count BIGINT NOT NULL DEFAULT 0`,
		`CREATE INDEX IF NOT EXISTS idx_links_slug ON links(slug)`,
		`CREATE TABLE IF NOT EXISTS link_click_daily (
			link_id TEXT NOT NULL REFERENCES links(id) ON DELETE CASCADE,
			click_date DATE NOT NULL,
			clicks BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (link_id, click_date)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_link_click_daily_date ON link_click_daily(link_id, click_date)`,
	}

	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to apply schema statement: %w", err)
		}
	}
	return nil
}

func (r *PostgresRepository) Create(ctx context.Context, link *domain.Link) error {
	query := `INSERT INTO links (id, slug, original_url, created_at, expires_at, click_count)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.ExecContext(
		ctx,
		query,
		link.ID,
		link.Slug,
		link.OriginalURL,
		link.CreatedAt,
		link.ExpiresAt,
		link.ClickCount,
	)
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
	query := `SELECT id, slug, original_url, created_at, expires_at, click_count
		FROM links WHERE slug = $1`
	row := r.db.QueryRowContext(ctx, query, slug)

	var link domain.Link
	var expiresAt sql.NullTime

	err := row.Scan(
		&link.ID,
		&link.Slug,
		&link.OriginalURL,
		&link.CreatedAt,
		&expiresAt,
		&link.ClickCount,
	)
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

func (r *PostgresRepository) RecordClick(ctx context.Context, linkID string, at time.Time) error {
	dayStr := at.UTC().Format("2006-01-02")
	query := `WITH updated AS (
		UPDATE links SET click_count = click_count + 1 WHERE id = $1 RETURNING id
	)
	INSERT INTO link_click_daily (link_id, click_date, clicks)
	SELECT id, $2::date, 1 FROM updated
	ON CONFLICT (link_id, click_date) DO UPDATE SET clicks = link_click_daily.clicks + 1`
	if _, err := r.db.ExecContext(ctx, query, linkID, dayStr); err != nil {
		return fmt.Errorf("database error recording click: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetStats(ctx context.Context, slug string, startDate time.Time, days int) (*domain.LinkStats, error) {
	link, err := r.FindBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	startStr := startDate.UTC().Format("2006-01-02")
	query := `SELECT TO_CHAR(click_date, 'YYYY-MM-DD'), clicks
		FROM link_click_daily
		WHERE link_id = $1 AND click_date >= $2::date
		ORDER BY click_date ASC`
	rows, err := r.db.QueryContext(ctx, query, link.ID, startStr)
	if err != nil {
		return nil, fmt.Errorf("database error loading click stats: %w", err)
	}
	defer rows.Close()

	var daily []domain.DailyClickCount
	for rows.Next() {
		var entry domain.DailyClickCount
		if err := rows.Scan(&entry.Date, &entry.Clicks); err != nil {
			return nil, fmt.Errorf("database error scanning click stats: %w", err)
		}
		daily = append(daily, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("database error iterating click stats: %w", err)
	}

	return &domain.LinkStats{
		Slug:        link.Slug,
		OriginalURL: link.OriginalURL,
		Status:      link.EffectiveStatus(),
		CreatedAt:   link.CreatedAt,
		ExpiresAt:   link.ExpiresAt,
		TotalClicks: link.ClickCount,
		Timezone:    "UTC",
		RangeDays:   days,
		Daily:       daily,
	}, nil
}
