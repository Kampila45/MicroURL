package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"url-shortener/internal/application"
	"url-shortener/internal/domain"

	"github.com/skip2/go-qrcode"
)

const (
	requestTimeout      = 30 * time.Second
	maxRequestBodyBytes = 16 << 10
)

// CaptchaVerifier validates anti-bot challenge tokens using the client IP.
type CaptchaVerifier interface {
	Verify(ctx context.Context, token string, remoteIP string) error
}

type Handlers struct {
	createLinkUseCase   *application.CreateLinkUseCase
	resolveLinkUseCase  *application.ResolveLinkUseCase
	getLinkStatsUseCase *application.GetLinkStatsUseCase
	db                  *sql.DB
	cache               application.Cache
	baseURL             string
	captchaVerifier     CaptchaVerifier
	ipResolver          *ClientIPResolver
}

func NewHandlers(
	createLinkUseCase *application.CreateLinkUseCase,
	resolveLinkUseCase *application.ResolveLinkUseCase,
	getLinkStatsUseCase *application.GetLinkStatsUseCase,
	db *sql.DB,
	cache application.Cache,
	baseURL string,
	captchaVerifier CaptchaVerifier,
	ipResolver *ClientIPResolver,
) *Handlers {
	if ipResolver == nil {
		ipResolver = NewClientIPResolver(nil)
	}
	return &Handlers{
		createLinkUseCase:   createLinkUseCase,
		resolveLinkUseCase:  resolveLinkUseCase,
		getLinkStatsUseCase: getLinkStatsUseCase,
		db:                  db,
		cache:               cache,
		baseURL:             baseURL,
		captchaVerifier:     captchaVerifier,
		ipResolver:          ipResolver,
	}
}

type CreateLinkRequest struct {
	URL          string  `json:"url"`
	CustomSlug   string  `json:"custom_slug,omitempty"`
	ExpiresAt    *string `json:"expires_at,omitempty"`
	CaptchaToken string  `json:"captcha_token"`
}

type CreateLinkResponse struct {
	ID          string  `json:"id"`
	Slug        string  `json:"slug"`
	OriginalURL string  `json:"original_url"`
	CreatedAt   string  `json:"created_at"`
	ExpiresAt   *string `json:"expires_at,omitempty"`
	ShortURL    string  `json:"short_url"`
	QRCodeURL   string  `json:"qr_code_url"`
}

func (h *Handlers) CreateLink(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req CreateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		log.Printf("error: failed to decode request body: %v", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.URL) == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	validatedURL, err := domain.ValidateDestinationURL(req.URL)
	if err != nil {
		log.Printf("error: invalid URL rejected: %v", err)
		http.Error(w, formatValidationError(err), http.StatusBadRequest)
		return
	}

	useCaseReq := application.CreateLinkRequest{
		OriginalURL: validatedURL,
		CustomSlug:  req.CustomSlug,
	}

	if strings.TrimSpace(req.CustomSlug) != "" {
		normalizedAlias, err := domain.ValidateCustomAlias(req.CustomSlug)
		if err != nil {
			log.Printf("error: invalid custom alias rejected: %v", err)
			http.Error(w, formatValidationError(err), http.StatusBadRequest)
			return
		}
		useCaseReq.CustomSlug = normalizedAlias
	}

	if req.ExpiresAt != nil {
		trimmedExpiry := strings.TrimSpace(*req.ExpiresAt)
		parsedTime, err := time.Parse(time.RFC3339Nano, trimmedExpiry)
		if err != nil {
			log.Printf("error: invalid expires_at format: %v", err)
			http.Error(w, "invalid expires_at format", http.StatusBadRequest)
			return
		}
		if !parsedTime.After(time.Now()) {
			http.Error(w, "expires_at must be in the future", http.StatusBadRequest)
			return
		}
		useCaseReq.ExpiresAt = &parsedTime
	}

	if h.captchaVerifier != nil {
		clientIP := h.ipResolver.Resolve(r)
		if err := h.captchaVerifier.Verify(ctx, req.CaptchaToken, clientIP); err != nil {
			log.Printf("warning: captcha verification failed: %v", err)
			http.Error(w, "invalid captcha token", http.StatusForbidden)
			return
		}
	}

	resp, err := h.createLinkUseCase.Execute(ctx, useCaseReq)
	if err != nil {
		log.Printf("error: failed to create link: %v", err)
		if errors.Is(err, domain.ErrInvalidURL) || errors.Is(err, domain.ErrInvalidAlias) {
			http.Error(w, formatValidationError(err), http.StatusBadRequest)
			return
		}
		if errors.Is(err, domain.ErrExpiredTimestamp) {
			http.Error(w, "expires_at must be in the future", http.StatusBadRequest)
			return
		}
		if errors.Is(err, domain.ErrAliasConflict) {
			http.Error(w, "custom alias is already taken", http.StatusConflict)
			return
		}
		if errors.Is(err, domain.ErrInvalidSlug) {
			http.Error(w, "slug collision, please try again", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create link", http.StatusInternalServerError)
		return
	}

	link := resp.Link
	shortURL := fmt.Sprintf("%s/%s", h.baseURL, link.Slug)
	qrCodeURL := fmt.Sprintf("%s/api/qr/%s", h.baseURL, link.Slug)
	response := CreateLinkResponse{
		ID:          link.ID,
		Slug:        link.Slug,
		OriginalURL: link.OriginalURL,
		CreatedAt:   link.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		ShortURL:    shortURL,
		QRCodeURL:   qrCodeURL,
	}

	if link.ExpiresAt != nil {
		expiresAtStr := link.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
		response.ExpiresAt = &expiresAtStr
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("error: failed to encode response: %v", err)
	}
}

func (h *Handlers) ResolveLink(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if len(r.URL.Path) > 4 && r.URL.Path[:4] == "/api" {
		http.NotFound(w, r)
		return
	}

	slug := r.URL.Path[1:]
	if slug == "" {
		http.Error(w, "slug is required", http.StatusBadRequest)
		return
	}

	req := application.ResolveLinkRequest{Slug: slug, CountClick: true}
	resp, err := h.resolveLinkUseCase.Execute(ctx, req)

	if err != nil {
		if errors.Is(err, domain.ErrLinkNotFound) {
			log.Printf("info: link not found for slug: %s", slug)
			http.Error(w, "link not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, domain.ErrLinkExpired) {
			log.Printf("info: link expired for slug: %s", slug)
			http.Error(w, "link expired", http.StatusGone)
			return
		}
		log.Printf("error: failed to resolve link: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if resp.CacheHit {
		w.Header().Set("X-Cache", "HIT")
	} else {
		w.Header().Set("X-Cache", "MISS")
	}
	http.Redirect(w, r, resp.OriginalURL, http.StatusFound)
}

func (h *Handlers) GenerateQRCode(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/qr/")
	if path == "" || path == r.URL.Path {
		http.Error(w, "slug is required", http.StatusBadRequest)
		return
	}

	slug := path
	req := application.ResolveLinkRequest{Slug: slug, CountClick: false}
	_, err := h.resolveLinkUseCase.Execute(ctx, req)

	if err != nil {
		if errors.Is(err, domain.ErrLinkNotFound) {
			log.Printf("info: link not found for slug: %s", slug)
			http.Error(w, "link not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, domain.ErrLinkExpired) {
			log.Printf("info: link expired for slug: %s", slug)
			http.Error(w, "link expired", http.StatusGone)
			return
		}
		log.Printf("error: failed to resolve link: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	shortURL := fmt.Sprintf("%s/%s", h.baseURL, slug)

	qrCode, err := qrcode.New(shortURL, qrcode.Medium)
	if err != nil {
		log.Printf("error: failed to generate QR code: %v", err)
		http.Error(w, "failed to generate QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if err := qrCode.Write(256, w); err != nil {
		log.Printf("error: failed to write QR code: %v", err)
		return
	}
}

type HealthResponse struct {
	Status    string          `json:"status"`
	Message   string          `json:"message"`
	Database  HealthComponent `json:"database"`
	Cache     HealthComponent `json:"cache"`
	Timestamp string          `json:"timestamp"`
}

type HealthComponent struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	dbComponent := HealthComponent{
		Status:  "healthy",
		Message: "Database connection is active and responding",
	}
	if err := h.db.PingContext(ctx); err != nil {
		dbComponent.Status = "unhealthy"
		dbComponent.Message = fmt.Sprintf("Database connection failed: %v", err)
		log.Printf("error: database health check failed: %v", err)
	}

	cacheComponent := HealthComponent{
		Status:  "healthy",
		Message: "Cache connection is active and responding",
	}
	testKey := "health:check"
	testLink := &domain.Link{
		ID:          "health-check",
		Slug:        "health-check",
		OriginalURL: "https://example.com",
		CreatedAt:   time.Now(),
	}
	if err := h.cache.Set(ctx, testKey, testLink, 1); err != nil {
		cacheComponent.Status = "degraded"
		cacheComponent.Message = fmt.Sprintf("Cache connection failed: %v. API will continue to work but without caching benefits", err)
		log.Printf("warning: cache health check failed: %v", err)
	} else {
		_ = h.cache.Delete(ctx, testKey)
	}

	overallStatus := "healthy"
	overallMessage := "All systems operational"
	statusCode := http.StatusOK

	if dbComponent.Status == "unhealthy" {
		overallStatus = "unhealthy"
		overallMessage = "Service is unavailable due to database connection failure"
		statusCode = http.StatusServiceUnavailable
	} else if cacheComponent.Status == "degraded" {
		overallStatus = "degraded"
		overallMessage = "Service is operational but cache is unavailable. Performance may be impacted"
		statusCode = http.StatusOK
	}

	response := HealthResponse{
		Status:    overallStatus,
		Message:   overallMessage,
		Database:  dbComponent,
		Cache:     cacheComponent,
		Timestamp: time.Now().Format("2006-01-02T15:04:05Z07:00"),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("error: failed to encode health response: %v", err)
	}
}

// LinksSubresource routes /api/links/{slug}/stats.
func (h *Handlers) LinksSubresource(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/links/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	slug := parts[0]
	action := parts[1]

	switch action {
	case "stats":
		h.getLinkStats(w, r, slug)
	default:
		http.NotFound(w, r)
	}
}

func parseStatsRange(rawRange string) (int, string, error) {
	switch strings.TrimSpace(strings.ToLower(rawRange)) {
	case "", "7d":
		return 7, "7d", nil
	case "30d":
		return 30, "30d", nil
	default:
		return 0, "", fmt.Errorf("range must be 7d or 30d")
	}
}

func (h *Handlers) getLinkStats(w http.ResponseWriter, r *http.Request, slug string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	days, _, err := parseStatsRange(r.URL.Query().Get("range"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	stats, err := h.getLinkStatsUseCase.Execute(ctx, application.GetLinkStatsRequest{
		Slug: slug,
		Days: days,
	})
	if err != nil {
		if errors.Is(err, domain.ErrLinkNotFound) {
			http.Error(w, "link not found", http.StatusNotFound)
			return
		}
		log.Printf("error: failed to load link stats for %s: %v", slug, err)
		http.Error(w, "failed to load link stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(stats)
}

// formatValidationError strips the wrapped sentinel suffix while exposing a clean,
// static validation explanation without echoing untrusted input back to callers.
func formatValidationError(err error) string {
	msg := err.Error()
	for _, suffix := range []string{": invalid url", ": invalid custom alias"} {
		if idx := strings.Index(msg, suffix); idx > 0 {
			return msg[:idx]
		}
	}
	return "invalid request input"
}
