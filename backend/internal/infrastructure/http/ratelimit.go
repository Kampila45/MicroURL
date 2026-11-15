package http

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type rateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	cleanup  *time.Ticker
}

func newRateLimiter(requestsPerSecond float64, burst int) *rateLimiter {
	rl := &rateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:      rate.Limit(requestsPerSecond),
		burst:     burst,
		cleanup:   time.NewTicker(1 * time.Minute),
	}

	go rl.cleanupLimiters()

	return rl
}

func (rl *rateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[ip]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter = rate.NewLimiter(rl.rate, rl.burst)
	rl.limiters[ip] = limiter

	return limiter
}

func (rl *rateLimiter) cleanupLimiters() {
	for range rl.cleanup.C {
		rl.mu.Lock()
		for ip, limiter := range rl.limiters {
			if limiter.Tokens() >= float64(rl.burst) {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	return rl.getLimiter(ip).Allow()
}

func extractIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		return ip
	}

	ip = r.Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

type rateLimitMiddleware struct {
	limiter *rateLimiter
	handler http.Handler
}

func (m *rateLimitMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" || r.URL.Path == "/docs" || r.URL.Path == "/openapi.json" {
		m.handler.ServeHTTP(w, r)
		return
	}

	ip := extractIP(r)
	if !m.limiter.allow(ip) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprintf(w, `{"error": "rate limit exceeded"}`)
		return
	}

	m.handler.ServeHTTP(w, r)
}

func newRateLimitMiddleware(requestsPerSecond float64, burst int, handler http.Handler) http.Handler {
	limiter := newRateLimiter(requestsPerSecond, burst)
	return &rateLimitMiddleware{
		limiter: limiter,
		handler: handler,
	}
}

