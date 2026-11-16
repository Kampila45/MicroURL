package config

import (
	"testing"
)

func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://microurl:microurl_dev_password@localhost:5432/microurl?sslmode=disable")
	t.Setenv("BASE_URL", "http://localhost:8080")
	t.Setenv("APP_ENV", "")
	t.Setenv("PORT", "")
	t.Setenv("RATE_LIMIT_RPS", "")
	t.Setenv("RATE_LIMIT_BURST", "")
	t.Setenv("UPSTASH_REDIS_REST_URL", "")
	t.Setenv("UPSTASH_REDIS_REST_TOKEN", "")
	t.Setenv("CACHE_TTL", "")
	t.Setenv("TRUSTED_PROXIES", "")
	t.Setenv("TURNSTILE_SECRET_KEY", "")
}

func TestLoadDefaults(t *testing.T) {
	setBaseEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.AppEnv != "development" {
		t.Fatalf("AppEnv = %q, want %q", cfg.AppEnv, "development")
	}
	if cfg.Port != "8080" {
		t.Fatalf("Port = %q, want %q", cfg.Port, "8080")
	}
	if cfg.RateLimitRPS != 10.0 || cfg.RateLimitBurst != 20 {
		t.Fatalf("unexpected rate limit defaults: rps=%v burst=%d", cfg.RateLimitRPS, cfg.RateLimitBurst)
	}
	if cfg.UpstashRedisRESTURL != "" || cfg.UpstashRedisRESTToken != "" || cfg.CacheTTL != 3600 {
		t.Fatalf("unexpected cache defaults: url=%q token=%q ttl=%d", cfg.UpstashRedisRESTURL, cfg.UpstashRedisRESTToken, cfg.CacheTTL)
	}
}

func TestLoadValidation(t *testing.T) {
	t.Run("missing DATABASE_URL", func(t *testing.T) {
		setBaseEnv(t)
		t.Setenv("DATABASE_URL", "")
		if _, err := Load(); err == nil {
			t.Fatal("expected error when DATABASE_URL is missing")
		}
	})

	t.Run("missing BASE_URL", func(t *testing.T) {
		setBaseEnv(t)
		t.Setenv("BASE_URL", "")
		if _, err := Load(); err == nil {
			t.Fatal("expected error when BASE_URL is missing")
		}
	})

	t.Run("production requires TURNSTILE_SECRET_KEY", func(t *testing.T) {
		setBaseEnv(t)
		t.Setenv("APP_ENV", "production")
		if _, err := Load(); err == nil {
			t.Fatal("expected error when TURNSTILE_SECRET_KEY is missing in production")
		}
	})

	t.Run("non-positive RATE_LIMIT_RPS rejected", func(t *testing.T) {
		setBaseEnv(t)
		t.Setenv("RATE_LIMIT_RPS", "0")
		if _, err := Load(); err == nil {
			t.Fatal("expected error when RATE_LIMIT_RPS <= 0")
		}
	})

	t.Run("accepts and trims Upstash Redis REST config", func(t *testing.T) {
		setBaseEnv(t)
		t.Setenv("UPSTASH_REDIS_REST_URL", " https://example.upstash.io/ ")
		t.Setenv("UPSTASH_REDIS_REST_TOKEN", " test-token ")
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}
		if cfg.UpstashRedisRESTURL != "https://example.upstash.io" || cfg.UpstashRedisRESTToken != "test-token" {
			t.Fatalf("Upstash config = (%q, %q), want trimmed URL and token", cfg.UpstashRedisRESTURL, cfg.UpstashRedisRESTToken)
		}
	})

	t.Run("rejects invalid UPSTASH_REDIS_REST_URL", func(t *testing.T) {
		setBaseEnv(t)
		t.Setenv("UPSTASH_REDIS_REST_URL", "redis://localhost:6379")
		if _, err := Load(); err == nil {
			t.Fatal("expected error for non-http(s) UPSTASH_REDIS_REST_URL")
		}
	})

	t.Run("parses trusted proxy IPs and CIDRs", func(t *testing.T) {
		setBaseEnv(t)
		t.Setenv("TRUSTED_PROXIES", "127.0.0.1, 10.0.0.0/8, ::1")
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() unexpected error: %v", err)
		}
		if len(cfg.TrustedProxies) != 3 {
			t.Fatalf("TrustedProxies len = %d, want 3", len(cfg.TrustedProxies))
		}
	})

	t.Run("rejects invalid TRUSTED_PROXIES", func(t *testing.T) {
		setBaseEnv(t)
		t.Setenv("TRUSTED_PROXIES", "not-an-ip")
		if _, err := Load(); err == nil {
			t.Fatal("expected error when TRUSTED_PROXIES contains an invalid entry")
		}
	})

	t.Run("rejects trailing characters and NaN in numeric env vars", func(t *testing.T) {
		setBaseEnv(t)
		t.Setenv("RATE_LIMIT_RPS", "10abc")
		if _, err := Load(); err == nil {
			t.Fatal("expected error for RATE_LIMIT_RPS with trailing characters")
		}

		setBaseEnv(t)
		t.Setenv("RATE_LIMIT_RPS", "NaN")
		if _, err := Load(); err == nil {
			t.Fatal("expected error for RATE_LIMIT_RPS=NaN")
		}

		setBaseEnv(t)
		t.Setenv("CACHE_TTL", "3600s")
		if _, err := Load(); err == nil {
			t.Fatal("expected error for CACHE_TTL with trailing characters")
		}
	})
}

