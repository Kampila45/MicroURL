package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"url-shortener/internal/application"
	"url-shortener/internal/domain"
	"url-shortener/internal/infrastructure/security"
)

const (
	requestTimeout = 30 * time.Second
)

type Handlers struct {
	createLinkUseCase  *application.CreateLinkUseCase
	resolveLinkUseCase *application.ResolveLinkUseCase
	db                 *sql.DB
	cache              application.Cache
	baseURL            string
	captchaVerifier    *security.TurnstileVerifier
}

func NewHandlers(
	createLinkUseCase *application.CreateLinkUseCase,
	resolveLinkUseCase *application.ResolveLinkUseCase,
	db *sql.DB,
	cache application.Cache,
	baseURL string,
	captchaVerifier *security.TurnstileVerifier,
) *Handlers {
	return &Handlers{
		createLinkUseCase:  createLinkUseCase,
		resolveLinkUseCase: resolveLinkUseCase,
		db:                 db,
		cache:              cache,
		baseURL:            baseURL,
		captchaVerifier:    captchaVerifier,
	}
}

type CreateLinkRequest struct {
	URL          string  `json:"url"`
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
}

func (h *Handlers) CreateLink(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("error: failed to decode request body: %v", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	if h.captchaVerifier != nil {
		clientIP := getClientIP(r)
		if err := h.captchaVerifier.Verify(ctx, req.CaptchaToken, clientIP); err != nil {
			log.Printf("warning: captcha verification failed: %v", err)
			http.Error(w, "invalid captcha token", http.StatusForbidden)
			return
		}
	}

	parsedURL, err := url.Parse(req.URL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		log.Printf("error: invalid URL format: %s", req.URL)
		http.Error(w, "invalid url format", http.StatusBadRequest)
		return
	}

	useCaseReq := application.CreateLinkRequest{
		OriginalURL: req.URL,
	}

	if req.ExpiresAt != nil {
		parsedTime, err := time.Parse("2006-01-02T15:04:05Z07:00", *req.ExpiresAt)
		if err != nil {
			log.Printf("error: invalid expires_at format: %s", *req.ExpiresAt)
			http.Error(w, "invalid expires_at format", http.StatusBadRequest)
			return
		}
		useCaseReq.ExpiresAt = &parsedTime
	}

	resp, err := h.createLinkUseCase.Execute(ctx, useCaseReq)
	if err != nil {
		log.Printf("error: failed to create link: %v", err)
		if err == domain.ErrInvalidSlug {
			http.Error(w, "slug collision, please try again", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create link", http.StatusInternalServerError)
		return
	}

	link := resp.Link
	shortURL := fmt.Sprintf("%s/%s", h.baseURL, link.Slug)
	response := CreateLinkResponse{
		ID:          link.ID,
		Slug:        link.Slug,
		OriginalURL: link.OriginalURL,
		CreatedAt:   link.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		ShortURL:    shortURL,
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

	req := application.ResolveLinkRequest{Slug: slug}
	resp, err := h.resolveLinkUseCase.Execute(ctx, req)

	if err != nil {
		if err == domain.ErrLinkNotFound {
			log.Printf("info: link not found for slug: %s", slug)
			http.Error(w, "link not found", http.StatusNotFound)
			return
		}
		if err == domain.ErrLinkExpired {
			log.Printf("info: link expired for slug: %s", slug)
			http.Error(w, "link expired", http.StatusGone)
			return
		}
		log.Printf("error: failed to resolve link: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, resp.OriginalURL, http.StatusFound)
}

type HealthResponse struct {
	Status    string            `json:"status"`
	Message   string            `json:"message"`
	Database  HealthComponent   `json:"database"`
	Cache     HealthComponent   `json:"cache"`
	Timestamp string            `json:"timestamp"`
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

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	return r.RemoteAddr
}
