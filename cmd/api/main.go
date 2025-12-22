package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"url-shortener/internal/application"
	"url-shortener/internal/config"
	"url-shortener/internal/infrastructure/cache"
	httphandler "url-shortener/internal/infrastructure/http"
	"url-shortener/internal/infrastructure/persistence"
	"url-shortener/internal/infrastructure/security"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	db, err := config.NewDatabase(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	repo := persistence.NewPostgresRepository(db)
	
	memcacheAdapter := cache.NewMemcacheAdapter(cfg.CacheServers)
	
	slugGenerator := application.NewSlugGenerator(repo)
	createLinkUseCase := application.NewCreateLinkUseCase(repo, slugGenerator, memcacheAdapter, cfg.CacheTTL)
	resolveLinkUseCase := application.NewResolveLinkUseCase(repo, memcacheAdapter, cfg.CacheTTL)

	turnstileKey := strings.TrimSpace(cfg.TurnstileSecretKey)
	isDev := cfg.AppEnv == "development"
	if turnstileKey == "" && !isDev {
		log.Fatalf("Turnstile secret key is not set or empty (required in %s)", cfg.AppEnv)
	}

	turnstileVerifier := security.NewTurnstileVerifier(turnstileKey, isDev)
	handlers := httphandler.NewHandlers(createLinkUseCase, resolveLinkUseCase, db, memcacheAdapter, cfg.BaseURL, turnstileVerifier)
	router := httphandler.NewRouter(handlers, cfg.RateLimitRPS, cfg.RateLimitBurst)

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
	log.Printf("server starting on %s", addr)
		log.Printf("available endpoints:")
		log.Printf("  GET  /docs - API documentation (Scalar)")
		log.Printf("  GET  /openapi.json - OpenAPI specification")
		log.Printf("  GET  /health - Health check")
		log.Printf("  POST /api/links - Create short link")
		log.Printf("  GET  /{slug} - Resolve short link")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("server shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server exited")
}

