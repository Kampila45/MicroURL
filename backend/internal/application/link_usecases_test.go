package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
	"url-shortener/internal/domain"
)

type memoryRepo struct {
	mu                 sync.RWMutex
	links              map[string]*domain.Link
	dailyClicks        map[string]map[string]int64
	existsChecks       int
	alwaysExists       bool
	createConflictLeft int
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		links:       make(map[string]*domain.Link),
		dailyClicks: make(map[string]map[string]int64),
	}
}

func (r *memoryRepo) Create(_ context.Context, link *domain.Link) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createConflictLeft > 0 {
		r.createConflictLeft--
		return domain.ErrInvalidSlug
	}
	if _, exists := r.links[link.Slug]; exists {
		return domain.ErrInvalidSlug
	}
	copied := *link
	r.links[link.Slug] = &copied
	return nil
}

func (r *memoryRepo) FindBySlug(_ context.Context, slug string) (*domain.Link, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	link, ok := r.links[slug]
	if !ok {
		return nil, domain.ErrLinkNotFound
	}
	copied := *link
	return &copied, nil
}

func (r *memoryRepo) Exists(_ context.Context, slug string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.existsChecks++
	if r.alwaysExists {
		return true, nil
	}
	_, ok := r.links[slug]
	return ok, nil
}

func (r *memoryRepo) RecordClick(_ context.Context, linkID string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, link := range r.links {
		if link.ID == linkID {
			link.ClickCount++
			dayKey := at.UTC().Format("2006-01-02")
			if r.dailyClicks[linkID] == nil {
				r.dailyClicks[linkID] = make(map[string]int64)
			}
			r.dailyClicks[linkID][dayKey]++
			return nil
		}
	}
	return domain.ErrLinkNotFound
}

func (r *memoryRepo) GetStats(_ context.Context, slug string, startDate time.Time, days int) (*domain.LinkStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	link, ok := r.links[slug]
	if !ok {
		return nil, domain.ErrLinkNotFound
	}

	var daily []domain.DailyClickCount
	startStr := startDate.UTC().Format("2006-01-02")
	for dayStr, count := range r.dailyClicks[link.ID] {
		if dayStr >= startStr {
			daily = append(daily, domain.DailyClickCount{Date: dayStr, Clicks: count})
		}
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

type memoryCache struct {
	mu          sync.Mutex
	items       map[string]*domain.Link
	deletedKeys []string
	forceErr    error
}

func newMemoryCache() *memoryCache {
	return &memoryCache{items: make(map[string]*domain.Link)}
}

func (c *memoryCache) Get(_ context.Context, key string) (*domain.Link, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.forceErr != nil {
		return nil, c.forceErr
	}
	link, ok := c.items[key]
	if !ok {
		return nil, ErrCacheMiss
	}
	return link, nil
}

func (c *memoryCache) Set(_ context.Context, key string, link *domain.Link, _ int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = link
	return nil
}

func (c *memoryCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
	c.deletedKeys = append(c.deletedKeys, key)
	return nil
}

func TestCreateAndResolveLinkUseCases(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepo()
	cache := newMemoryCache()

	createUC := NewCreateLinkUseCase(repo, NewSlugGenerator(repo), cache, 3600)
	resolveUC := NewResolveLinkUseCase(repo, cache, 3600)

	created, err := createUC.Execute(ctx, CreateLinkRequest{
		OriginalURL: "https://example.com/docs",
	})
	if err != nil {
		t.Fatalf("CreateLinkUseCase.Execute() error: %v", err)
	}
	if len(created.Link.Slug) != slugLength {
		t.Fatalf("slug length = %d, want %d", len(created.Link.Slug), slugLength)
	}

	resolved, err := resolveUC.Execute(ctx, ResolveLinkRequest{Slug: created.Link.Slug})
	if err != nil {
		t.Fatalf("ResolveLinkUseCase.Execute() error: %v", err)
	}
	if resolved.OriginalURL != "https://example.com/docs" {
		t.Fatalf("OriginalURL = %q, want %q", resolved.OriginalURL, "https://example.com/docs")
	}

	// Cache miss falls back to repository cleanly and warms the cache.
	delete(cache.items, "link:slug:"+created.Link.Slug)
	resolvedFallback, err := resolveUC.Execute(ctx, ResolveLinkRequest{Slug: created.Link.Slug})
	if err != nil {
		t.Fatalf("ResolveLinkUseCase fallback error: %v", err)
	}
	if resolvedFallback.OriginalURL != "https://example.com/docs" {
		t.Fatalf("fallback OriginalURL = %q", resolvedFallback.OriginalURL)
	}
	if _, warmed := cache.items["link:slug:"+created.Link.Slug]; !warmed {
		t.Fatal("expected cache to be warmed on database fallback")
	}
}

func TestResolveExpiredLinkEvictsCache(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepo()
	cache := newMemoryCache()

	past := time.Now().Add(-5 * time.Minute)
	expired := &domain.Link{
		ID:          "1",
		Slug:        "expired1",
		OriginalURL: "https://example.com/expired",
		ExpiresAt:   &past,
	}
	_ = cache.Set(ctx, "link:slug:expired1", expired, 3600)

	resolveUC := NewResolveLinkUseCase(repo, cache, 3600)
	_, err := resolveUC.Execute(ctx, ResolveLinkRequest{Slug: "expired1"})
	if !errors.Is(err, domain.ErrLinkExpired) {
		t.Fatalf("Execute() error = %v, want ErrLinkExpired", err)
	}
	if len(cache.deletedKeys) != 1 || cache.deletedKeys[0] != "link:slug:expired1" {
		t.Fatalf("expected expired cache entry deletion, got %v", cache.deletedKeys)
	}
}

func TestSlugGeneratorExhaustsRetries(t *testing.T) {
	repo := newMemoryRepo()
	repo.alwaysExists = true

	gen := NewSlugGenerator(repo)
	if _, err := gen.Generate(context.Background()); err == nil {
		t.Fatal("expected error when all slug attempts collide")
	}
	if repo.existsChecks != maxRetries {
		t.Fatalf("existsChecks = %d, want %d", repo.existsChecks, maxRetries)
	}
}

func TestCreateLinkRejectsInvalidInputBeforePersisting(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepo()
	cache := newMemoryCache()
	createUC := NewCreateLinkUseCase(repo, NewSlugGenerator(repo), cache, 3600)

	if _, err := createUC.Execute(ctx, CreateLinkRequest{
		OriginalURL: "javascript:alert(1)",
	}); !errors.Is(err, domain.ErrInvalidURL) {
		t.Fatalf("javascript URL error = %v, want ErrInvalidURL", err)
	}

	past := time.Now().Add(-1 * time.Minute)
	if _, err := createUC.Execute(ctx, CreateLinkRequest{
		OriginalURL: "https://example.com/valid",
		ExpiresAt:   &past,
	}); !errors.Is(err, domain.ErrExpiredTimestamp) {
		t.Fatalf("past expiration error = %v, want ErrExpiredTimestamp", err)
	}

	if len(repo.links) != 0 {
		t.Fatalf("expected 0 persisted links on validation failure, got %d", len(repo.links))
	}
}

func TestCustomAliasRetriesAndConflict(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepo()
	cache := newMemoryCache()
	createUC := NewCreateLinkUseCase(repo, NewSlugGenerator(repo), cache, 3600)

	// Custom alias normalizes to lowercase.
	created, err := createUC.Execute(ctx, CreateLinkRequest{
		OriginalURL: "https://example.com/portfolio",
		CustomSlug:  "My-Portfolio",
	})
	if err != nil {
		t.Fatalf("custom alias create error: %v", err)
	}
	if created.Link.Slug != "my-portfolio" {
		t.Fatalf("Slug = %q, want my-portfolio", created.Link.Slug)
	}

	// Duplicate custom alias yields ErrAliasConflict.
	if _, err := createUC.Execute(ctx, CreateLinkRequest{
		OriginalURL: "https://example.com/other",
		CustomSlug:  "my-portfolio",
	}); !errors.Is(err, domain.ErrAliasConflict) {
		t.Fatalf("duplicate alias error = %v, want ErrAliasConflict", err)
	}

	// Random slug retries cleanly on repository unique-constraint collision.
	repo.createConflictLeft = 2
	retried, err := createUC.Execute(ctx, CreateLinkRequest{
		OriginalURL: "https://example.com/retried",
	})
	if err != nil {
		t.Fatalf("random slug retry error = %v, want success", err)
	}
	if retried.Link.Slug == "" || repo.createConflictLeft != 0 {
		t.Fatalf("expected retries to clear conflicts, got left=%d", repo.createConflictLeft)
	}
}

func TestClickAnalyticsTracking(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepo()
	cache := newMemoryCache()
	createUC := NewCreateLinkUseCase(repo, NewSlugGenerator(repo), cache, 3600)
	resolveUC := NewResolveLinkUseCase(repo, cache, 3600)
	statsUC := NewGetLinkStatsUseCase(repo)

	if _, err := createUC.Execute(ctx, CreateLinkRequest{
		OriginalURL: "https://example.com/tracked",
		CustomSlug:  "tracked-link",
	}); err != nil {
		t.Fatalf("create error: %v", err)
	}

	// QR verification checks (CountClick=false) must NOT increment clicks.
	if _, err := resolveUC.Execute(ctx, ResolveLinkRequest{Slug: "tracked-link", CountClick: false}); err != nil {
		t.Fatalf("non-click resolve error: %v", err)
	}

	// One cache hit redirect + one DB fallback redirect must count as 2 clicks.
	if _, err := resolveUC.Execute(ctx, ResolveLinkRequest{Slug: "tracked-link", CountClick: true}); err != nil {
		t.Fatalf("cache hit redirect error: %v", err)
	}
	delete(cache.items, "link:slug:tracked-link")
	if _, err := resolveUC.Execute(ctx, ResolveLinkRequest{Slug: "tracked-link", CountClick: true}); err != nil {
		t.Fatalf("cache miss redirect error: %v", err)
	}

	stats, err := statsUC.Execute(ctx, GetLinkStatsRequest{Slug: "tracked-link", Days: 7})
	if err != nil {
		t.Fatalf("stats error: %v", err)
	}
	if stats.TotalClicks != 2 || len(stats.Daily) != 7 || stats.Daily[6].Clicks != 2 {
		t.Fatalf("unexpected stats: total=%d daily=%+v", stats.TotalClicks, stats.Daily)
	}
}

