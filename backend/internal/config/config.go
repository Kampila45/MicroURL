package config

import (
	"fmt"
	"math"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL           string
	BaseURL               string
	Port                  string
	RateLimitRPS          float64
	RateLimitBurst        int
	TrustedProxies        []netip.Prefix
	UpstashRedisRESTURL   string
	UpstashRedisRESTToken string
	CacheTTL              int
	TurnstileSecretKey    string
	AppEnv                string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		BaseURL:            os.Getenv("BASE_URL"),
		Port:               os.Getenv("PORT"),
		TurnstileSecretKey: os.Getenv("TURNSTILE_SECRET_KEY"),
		AppEnv:             os.Getenv("APP_ENV"),
	}

	if cfg.AppEnv == "" {
		cfg.AppEnv = "development"
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("BASE_URL environment variable is required")
	}

	if cfg.AppEnv == "production" && cfg.TurnstileSecretKey == "" {
		return nil, fmt.Errorf("TURNSTILE_SECRET_KEY environment variable is required when APP_ENV is production")
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	rps := os.Getenv("RATE_LIMIT_RPS")
	if rps == "" {
		cfg.RateLimitRPS = 10.0
	} else {
		var err error
		cfg.RateLimitRPS, err = parseFloat64(rps)
		if err != nil {
			return nil, fmt.Errorf("invalid RATE_LIMIT_RPS: %w", err)
		}
		if cfg.RateLimitRPS <= 0 {
			return nil, fmt.Errorf("RATE_LIMIT_RPS must be greater than 0")
		}
	}

	burst := os.Getenv("RATE_LIMIT_BURST")
	if burst == "" {
		cfg.RateLimitBurst = 20
	} else {
		var err error
		cfg.RateLimitBurst, err = parseInt(burst)
		if err != nil {
			return nil, fmt.Errorf("invalid RATE_LIMIT_BURST: %w", err)
		}
		if cfg.RateLimitBurst <= 0 {
			return nil, fmt.Errorf("RATE_LIMIT_BURST must be greater than 0")
		}
	}

	upstashURL := strings.TrimSpace(os.Getenv("UPSTASH_REDIS_REST_URL"))
	if upstashURL != "" {
		parsedURL, err := url.ParseRequestURI(upstashURL)
		if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
			return nil, fmt.Errorf("invalid UPSTASH_REDIS_REST_URL: must be a valid http or https URL")
		}
		cfg.UpstashRedisRESTURL = strings.TrimRight(upstashURL, "/")
	}
	cfg.UpstashRedisRESTToken = strings.TrimSpace(os.Getenv("UPSTASH_REDIS_REST_TOKEN"))

	ttl := os.Getenv("CACHE_TTL")
	if ttl == "" {
		cfg.CacheTTL = 3600
	} else {
		var err error
		cfg.CacheTTL, err = parseInt(ttl)
		if err != nil {
			return nil, fmt.Errorf("invalid CACHE_TTL: %w", err)
		}
		if cfg.CacheTTL <= 0 {
			return nil, fmt.Errorf("CACHE_TTL must be greater than 0")
		}
	}

	trustedProxies := os.Getenv("TRUSTED_PROXIES")
	if trustedProxies != "" {
		parsedProxies, err := parseTrustedProxies(trustedProxies)
		if err != nil {
			return nil, fmt.Errorf("invalid TRUSTED_PROXIES: %w", err)
		}
		cfg.TrustedProxies = parsedProxies
	}

	return cfg, nil
}

func parseTrustedProxies(raw string) ([]netip.Prefix, error) {
	entries := splitString(raw, ",")
	prefixes := make([]netip.Prefix, 0, len(entries))
	for _, entry := range entries {
		if strings.Contains(entry, "/") {
			prefix, err := netip.ParsePrefix(entry)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR %q: %w", entry, err)
			}
			prefixes = append(prefixes, prefix.Masked())
			continue
		}

		addr, err := netip.ParseAddr(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid IP address %q: %w", entry, err)
		}
		addr = addr.Unmap()
		prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return prefixes, nil
}

func splitString(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseFloat64(s string) (float64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("non-finite float value")
	}
	return f, nil
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}

