package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"
	"url-shortener/internal/application"
	"url-shortener/internal/domain"
	"url-shortener/internal/infrastructure/security"
)

type stubRepo struct {
	mu          sync.RWMutex
	links       map[string]*domain.Link
	dailyClicks map[string]map[string]int64
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		links:       make(map[string]*domain.Link),
		dailyClicks: make(map[string]map[string]int64),
	}
}

func (r *stubRepo) Create(_ context.Context, link *domain.Link) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.links[link.Slug]; exists {
		return domain.ErrInvalidSlug
	}
	copied := *link
	r.links[link.Slug] = &copied
	return nil
}

func (r *stubRepo) FindBySlug(_ context.Context, slug string) (*domain.Link, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	link, ok := r.links[slug]
	if !ok {
		return nil, domain.ErrLinkNotFound
	}
	copied := *link
	return &copied, nil
}

func (r *stubRepo) Exists(_ context.Context, slug string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.links[slug]
	return ok, nil
}

func (r *stubRepo) RecordClick(_ context.Context, linkID string, at time.Time) error {
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

func (r *stubRepo) GetStats(_ context.Context, slug string, startDate time.Time, days int) (*domain.LinkStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	link, ok := r.links[slug]
	if !ok {
		return nil, domain.ErrLinkNotFound
	}

	var daily []domain.DailyClickCount
	startStr := startDate.UTC().Format("2006-01-02")
	for dayStr, clicks := range r.dailyClicks[link.ID] {
		if dayStr >= startStr {
			daily = append(daily, domain.DailyClickCount{Date: dayStr, Clicks: clicks})
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

type stubCache struct {
	mu    sync.RWMutex
	items map[string]*domain.Link
}

func (c *stubCache) Get(_ context.Context, key string) (*domain.Link, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.items[key]
	if !ok {
		return nil, application.ErrCacheMiss
	}
	return v, nil
}

func (c *stubCache) Set(_ context.Context, key string, link *domain.Link, _ int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = link
	return nil
}

func (c *stubCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
	return nil
}

type spyCaptchaVerifier struct {
	lastRemoteIP string
	err          error
}

func (s *spyCaptchaVerifier) Verify(_ context.Context, _ string, remoteIP string) error {
	s.lastRemoteIP = remoteIP
	return s.err
}

func setupTestRouter(t *testing.T) http.Handler {
	t.Helper()
	router, _ := setupTestRouterWithCache(t)
	return router
}

func setupTestRouterWithCache(t *testing.T) (http.Handler, *stubCache) {
	t.Helper()
	t.Setenv("APP_ENV", "development")

	repo := newStubRepo()
	cache := &stubCache{items: make(map[string]*domain.Link)}
	createUC := application.NewCreateLinkUseCase(repo, application.NewSlugGenerator(repo), cache, 3600)
	resolveUC := application.NewResolveLinkUseCase(repo, cache, 3600)
	statsUC := application.NewGetLinkStatsUseCase(repo)
	verifier := security.NewTurnstileVerifier("", true, "development")
	resolver := NewClientIPResolver(nil)

	handlers := NewHandlers(createUC, resolveUC, statsUC, nil, cache, "http://localhost:8080", verifier, resolver)
	return NewRouter(handlers, 100.0, 100, resolver), cache
}

func TestCreateLinkResolveAndQREndpoints(t *testing.T) {
	router, cache := setupTestRouterWithCache(t)

	// Invalid URL returns 400.
	badReq := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(`{"url":"not-a-url"}`))
	badRes := httptest.NewRecorder()
	router.ServeHTTP(badRes, badReq)
	if badRes.Code != http.StatusBadRequest {
		t.Fatalf("invalid URL status = %d, want %d", badRes.Code, http.StatusBadRequest)
	}

	// Valid creation returns 201 + short_url + qr_code_url.
	createReq := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(`{"url":"https://example.com/path"}`))
	createRes := httptest.NewRecorder()
	router.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create link status = %d, want %d: %s", createRes.Code, http.StatusCreated, createRes.Body.String())
	}

	var payload CreateLinkResponse
	if err := json.Unmarshal(createRes.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	if payload.Slug == "" || payload.OriginalURL != "https://example.com/path" {
		t.Fatalf("unexpected create payload: %+v", payload)
	}

	// Resolving slug issues 302 redirect.
	resolveReq := httptest.NewRequest(http.MethodGet, "/"+payload.Slug, nil)
	resolveRes := httptest.NewRecorder()
	router.ServeHTTP(resolveRes, resolveReq)
	if resolveRes.Code != http.StatusFound {
		t.Fatalf("resolve status = %d, want %d", resolveRes.Code, http.StatusFound)
	}
	if loc := resolveRes.Header().Get("Location"); loc != "https://example.com/path" {
		t.Fatalf("Location header = %q, want %q", loc, "https://example.com/path")
	}
	if xc := resolveRes.Header().Get("X-Cache"); xc != "HIT" {
		t.Fatalf("resolve after create X-Cache = %q, want HIT", xc)
	}

	// Evicting the entry forces the next resolve to read from the database.
	if err := cache.Delete(context.Background(), "link:slug:"+payload.Slug); err != nil {
		t.Fatalf("cache eviction failed: %v", err)
	}
	missRes := httptest.NewRecorder()
	router.ServeHTTP(missRes, httptest.NewRequest(http.MethodGet, "/"+payload.Slug, nil))
	if missRes.Code != http.StatusFound {
		t.Fatalf("resolve after eviction status = %d, want %d", missRes.Code, http.StatusFound)
	}
	if xc := missRes.Header().Get("X-Cache"); xc != "MISS" {
		t.Fatalf("resolve after eviction X-Cache = %q, want MISS", xc)
	}

	// QR endpoint returns image/png bytes.
	qrReq := httptest.NewRequest(http.MethodGet, "/api/qr/"+payload.Slug, nil)
	qrRes := httptest.NewRecorder()
	router.ServeHTTP(qrRes, qrReq)
	if qrRes.Code != http.StatusOK {
		t.Fatalf("QR status = %d, want %d", qrRes.Code, http.StatusOK)
	}
	if ct := qrRes.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("QR Content-Type = %q, want image/png", ct)
	}

	// Missing slug returns 404.
	missingReq := httptest.NewRequest(http.MethodGet, "/missing99", nil)
	missingRes := httptest.NewRecorder()
	router.ServeHTTP(missingRes, missingReq)
	if missingRes.Code != http.StatusNotFound {
		t.Fatalf("missing slug status = %d, want %d", missingRes.Code, http.StatusNotFound)
	}
}

func TestCreateLinkValidationSecurity(t *testing.T) {
	router := setupTestRouter(t)

	invalidPayloads := []struct {
		name string
		body string
	}{
		{name: "javascript scheme", body: `{"url":"javascript:alert(1)"}`},
		{name: "data scheme", body: `{"url":"data:text/html,hi"}`},
		{name: "localhost", body: `{"url":"http://localhost:8080/admin"}`},
		{name: "private IP", body: `{"url":"http://192.168.1.1/"}`},
		{name: "embedded credentials", body: `{"url":"https://user:pass@example.com/"}`},
		{name: "past expiration", body: `{"url":"https://example.com","expires_at":"2020-01-01T00:00:00Z"}`},
		{name: "malformed expiration", body: `{"url":"https://example.com","expires_at":"tomorrow"}`},
	}

	for _, tc := range invalidPayloads {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(tc.body))
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != http.StatusBadRequest {
				t.Fatalf("%s: status = %d, want %d (body: %s)", tc.name, res.Code, http.StatusBadRequest, res.Body.String())
			}
		})
	}

	// Oversized request body returns 413.
	oversized := `{"url":"https://example.com/` + strings.Repeat("a", maxRequestBodyBytes) + `"}`
	largeReq := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(oversized))
	largeRes := httptest.NewRecorder()
	router.ServeHTTP(largeRes, largeReq)
	if largeRes.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d, want %d", largeRes.Code, http.StatusRequestEntityTooLarge)
	}

	// Fractional RFC3339 future timestamp + mixed-case URL succeed with 201.
	futureISO := time.Now().Add(24 * time.Hour).UTC().Format("2006-01-02T15:04:05.000Z")
	okBody := `{"url":"HTTPS://Example.com:8443/path?x=1%202#top","expires_at":"` + futureISO + `"}`
	okReq := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(okBody))
	okRes := httptest.NewRecorder()
	router.ServeHTTP(okRes, okReq)
	if okRes.Code != http.StatusCreated {
		t.Fatalf("valid future expiration status = %d, want %d: %s", okRes.Code, http.StatusCreated, okRes.Body.String())
	}
}

func TestSpoofedForwardedHeadersCannotBypassRateLimitAndTurnstileAgreesOnIP(t *testing.T) {
	t.Setenv("APP_ENV", "development")

	repo := newStubRepo()
	cache := &stubCache{items: make(map[string]*domain.Link)}
	createUC := application.NewCreateLinkUseCase(repo, application.NewSlugGenerator(repo), cache, 3600)
	resolveUC := application.NewResolveLinkUseCase(repo, cache, 3600)
	statsUC := application.NewGetLinkStatsUseCase(repo)
	spyCaptcha := &spyCaptchaVerifier{}

	trusted := []netip.Prefix{mustPrefix(t, "10.0.0.0/8")}
	resolver := NewClientIPResolver(trusted)
	handlers := NewHandlers(createUC, resolveUC, statsUC, nil, cache, "http://localhost:8080", spyCaptcha, resolver)
	router := NewRouter(handlers, 0.001, 1, resolver)

	// Direct untrusted client sends different X-Forwarded-For headers; second request
	// from the same peer must still hit 429 because X-Forwarded-For is ignored.
	req1 := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(`{"url":"https://example.com/1"}`))
	req1.RemoteAddr = "198.51.100.10:1111"
	req1.Header.Set("X-Forwarded-For", "203.0.113.1")
	res1 := httptest.NewRecorder()
	router.ServeHTTP(res1, req1)
	if res1.Code != http.StatusCreated {
		t.Fatalf("first untrusted request = %d, want 201", res1.Code)
	}
	if spyCaptcha.lastRemoteIP != "198.51.100.10" {
		t.Fatalf("Turnstile remoteip = %q, want peer IP %q", spyCaptcha.lastRemoteIP, "198.51.100.10")
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(`{"url":"https://example.com/2"}`))
	req2.RemoteAddr = "198.51.100.10:2222"
	req2.Header.Set("X-Forwarded-For", "203.0.113.2")
	res2 := httptest.NewRecorder()
	router.ServeHTTP(res2, req2)
	if res2.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed XFF second request = %d, want 429", res2.Code)
	}

	// Request arriving through trusted proxy uses forwarded client IP cleanly for both
	// rate limiter and Turnstile, and returns 403 when Turnstile verification fails.
	spyCaptcha.err = errors.New("bad captcha")
	proxyReq := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(`{"url":"https://example.com/3"}`))
	proxyReq.RemoteAddr = "10.0.0.9:8080"
	proxyReq.Header.Set("X-Forwarded-For", "1.1.1.1, 203.0.113.99, 10.0.0.2")
	proxyRes := httptest.NewRecorder()
	router.ServeHTTP(proxyRes, proxyReq)
	if proxyRes.Code != http.StatusForbidden {
		t.Fatalf("captcha failure status = %d, want 403", proxyRes.Code)
	}
	if spyCaptcha.lastRemoteIP != "203.0.113.99" {
		t.Fatalf("Turnstile remoteip behind trusted proxy = %q, want %q", spyCaptcha.lastRemoteIP, "203.0.113.99")
	}
}

func TestCORSPreflightAndOpenAPI(t *testing.T) {
	router := setupTestRouter(t)

	optReq := httptest.NewRequest(http.MethodOptions, "/api/links", nil)
	optRes := httptest.NewRecorder()
	router.ServeHTTP(optRes, optReq)
	if optRes.Code != http.StatusOK {
		t.Fatalf("OPTIONS status = %d, want %d", optRes.Code, http.StatusOK)
	}
	if got := optRes.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
	}

	specReq := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	specRes := httptest.NewRecorder()
	router.ServeHTTP(specRes, specReq)
	if specRes.Code != http.StatusOK {
		t.Fatalf("openapi.json status = %d, want %d", specRes.Code, http.StatusOK)
	}
}

func TestCustomAliasAndClickAnalyticsFlow(t *testing.T) {
	router := setupTestRouter(t)

	// Reserved alias rejected with 400.
	badAliasReq := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(`{"url":"https://example.com","custom_slug":"health"}`))
	badAliasRes := httptest.NewRecorder()
	router.ServeHTTP(badAliasRes, badAliasReq)
	if badAliasRes.Code != http.StatusBadRequest {
		t.Fatalf("reserved alias status = %d, want 400", badAliasRes.Code)
	}

	// Custom alias succeeds with 201 and normalizes to lowercase.
	createReq := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(`{"url":"https://example.com/portfolio","custom_slug":"Portfolio_2026"}`))
	createRes := httptest.NewRecorder()
	router.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("custom alias create status = %d, want 201: %s", createRes.Code, createRes.Body.String())
	}

	var created CreateLinkResponse
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Slug != "portfolio_2026" {
		t.Fatalf("unexpected create response: %+v", created)
	}

	// Duplicate custom alias returns 409 Conflict.
	dupReq := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(`{"url":"https://example.com/other","custom_slug":"portfolio_2026"}`))
	dupRes := httptest.NewRecorder()
	router.ServeHTTP(dupRes, dupReq)
	if dupRes.Code != http.StatusConflict {
		t.Fatalf("duplicate alias status = %d, want 409", dupRes.Code)
	}

	// QR generation must not increment clicks; redirects must increment clicks.
	qrRes := httptest.NewRecorder()
	router.ServeHTTP(qrRes, httptest.NewRequest(http.MethodGet, "/api/qr/portfolio_2026", nil))
	if qrRes.Code != http.StatusOK {
		t.Fatalf("qr status = %d, want 200", qrRes.Code)
	}
	for i := 0; i < 2; i++ {
		redRes := httptest.NewRecorder()
		router.ServeHTTP(redRes, httptest.NewRequest(http.MethodGet, "/portfolio_2026", nil))
		if redRes.Code != http.StatusFound {
			t.Fatalf("redirect #%d status = %d, want 302", i+1, redRes.Code)
		}
	}

	// Stats JSON endpoint returns 2 total clicks over 7d UTC buckets.
	statsRes := httptest.NewRecorder()
	router.ServeHTTP(statsRes, httptest.NewRequest(http.MethodGet, "/api/links/portfolio_2026/stats?range=7d", nil))
	if statsRes.Code != http.StatusOK {
		t.Fatalf("stats status = %d, want 200", statsRes.Code)
	}
	var stats domain.LinkStats
	if err := json.Unmarshal(statsRes.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if stats.TotalClicks != 2 || stats.Timezone != "UTC" || len(stats.Daily) != 7 {
		t.Fatalf("unexpected stats payload: %+v", stats)
	}

	// Invalid range rejected cleanly.
	badRangeRes := httptest.NewRecorder()
	router.ServeHTTP(badRangeRes, httptest.NewRequest(http.MethodGet, "/api/links/portfolio_2026/stats?range=90d", nil))
	if badRangeRes.Code != http.StatusBadRequest {
		t.Fatalf("invalid range status = %d, want 400", badRangeRes.Code)
	}
}
