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

	schemaCtx, schemaCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := persistence.EnsureSchema(schemaCtx, db); err != nil {
		schemaCancel()
		log.Fatalf("failed to ensure database schema: %v", err)
	}
	schemaCancel()

	repo := persistence.NewPostgresRepository(db)

	redisCache := cache.NewUpstashRedisAdapter(cfg.UpstashRedisRESTURL, cfg.UpstashRedisRESTToken)

	slugGenerator := application.NewSlugGenerator(repo)
	createLinkUseCase := application.NewCreateLinkUseCase(repo, slugGenerator, redisCache, cfg.CacheTTL)
	resolveLinkUseCase := application.NewResolveLinkUseCase(repo, redisCache, cfg.CacheTTL)
	getLinkStatsUseCase := application.NewGetLinkStatsUseCase(repo)

	turnstileKey := strings.TrimSpace(cfg.TurnstileSecretKey)
	isDev := cfg.AppEnv == "development"
	if turnstileKey == "" && !isDev {
		log.Fatalf("Turnstile secret key is not set or empty (required in %s)", cfg.AppEnv)
	}

	if turnstileKey == "" && isDev {
		log.Printf("WARNING: Captcha verification is DISABLED (dev mode with no secret key)")
		log.Printf("This should only be used in development environments!")
		log.Printf("Current APP_ENV: %s", cfg.AppEnv)
		if cfg.AppEnv != "development" {
			log.Printf("SECURITY RISK: APP_ENV is not 'development' but captcha is being bypassed!")
		}
	}

	turnstileVerifier := security.NewTurnstileVerifier(turnstileKey, isDev, cfg.AppEnv)
	ipResolver := httphandler.NewClientIPResolver(cfg.TrustedProxies)
	handlers := httphandler.NewHandlers(createLinkUseCase, resolveLinkUseCase, getLinkStatsUseCase, db, redisCache, cfg.BaseURL, turnstileVerifier, ipResolver)
	router := httphandler.NewRouter(handlers, cfg.RateLimitRPS, cfg.RateLimitBurst, ipResolver)

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("server starting on %s", addr)
		log.Printf("available endpoints:")
		log.Printf("  GET  /docs - API documentation (Scalar)")
		log.Printf("  GET  /openapi.json - OpenAPI specification")
		log.Printf("  GET  /health - Health check")
		log.Printf("  POST /api/links - Create short link")
		log.Printf("  GET  /api/links/{slug}/stats - Get aggregate click analytics")
		log.Printf("  GET  /api/qr/{slug} - Generate QR code for short link")
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

