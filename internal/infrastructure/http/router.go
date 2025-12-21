package http

import (
	"log"
	"net/http"
	"time"
)

func NewRouter(handlers *Handlers, rateLimitRPS float64, rateLimitBurst int) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/docs", handlers.ScalarUI)
	mux.HandleFunc("/openapi.json", handlers.OpenAPISpec)
	mux.HandleFunc("/health", handlers.Health)
	mux.HandleFunc("/api/links", handlers.CreateLink)
	mux.HandleFunc("/", handlers.ResolveLink)

	corsHandler := &corsMiddleware{handler: mux}
	loggingHandler := &loggingMiddleware{handler: corsHandler}
	return newRateLimitMiddleware(rateLimitRPS, rateLimitBurst, loggingHandler)
}

type corsMiddleware struct {
	handler http.Handler
}

func (c *corsMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE, PATCH")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, X-Requested-With, Origin")
	w.Header().Set("Access-Control-Max-Age", "3600")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	c.handler.ServeHTTP(w, r)
}

type loggingMiddleware struct {
	handler http.Handler
}

func (l *loggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	l.handler.ServeHTTP(w, r)
	duration := time.Since(start)
	log.Printf("%s %s %s %v", r.Method, r.URL.Path, r.RemoteAddr, duration)
}

